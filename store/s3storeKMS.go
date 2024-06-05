package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go"
)

// latest is used to keep a single object in s3 with all of the
// most recent values for the given service's secrets.  Keeping this
// in a single s3 object allows us to use a single s3 GetObject
// for ListRaw (and thus chamber exec).
type LatestIndexFile struct {
	Latest map[string]LatestValue `json:"latest"`
}

type LatestValue struct {
	Version  int    `json:"version"`
	Value    string `json:"value"`
	KMSAlias string `json:"KMSAlias"`
}

var _ Store = &S3KMSStore{}

type S3KMSStore struct {
	S3Store
	svc         apiS3
	stsSvc      apiSTS
	bucket      string
	kmsKeyAlias string
}

func NewS3KMSStore(ctx context.Context, numRetries int, bucket string, kmsKeyAlias string) (*S3KMSStore, error) {
	config, _, err := getConfig(ctx, numRetries, aws.RetryModeStandard)
	if err != nil {
		return nil, err
	}

	svc := s3.NewFromConfig(config)

	stsSvc := sts.NewFromConfig(config)

	if kmsKeyAlias == "" {
		kmsKeyAlias = DefaultKeyID
	}

	s3store := &S3Store{
		svc:    svc,
		stsSvc: stsSvc,
		bucket: bucket,
	}

	return &S3KMSStore{
		S3Store:     *s3store,
		svc:         svc,
		stsSvc:      stsSvc,
		bucket:      bucket,
		kmsKeyAlias: kmsKeyAlias,
	}, nil
}

func (s *S3KMSStore) Write(ctx context.Context, id SecretId, value string) error {
	index, err := s.readLatest(ctx, id.Service)
	if err != nil {
		return err
	}

	if val, ok := index.Latest[id.Key]; val.KMSAlias != s.kmsKeyAlias && ok {
		return fmt.Errorf("Unable to overwrite secret %s using new KMS key %s; mismatch with existing key %s", id.Key, s.kmsKeyAlias, val.KMSAlias)
	}

	objPath := getObjectPath(id)
	existing, ok, err := s.readObjectById(ctx, id)
	if err != nil {
		return err
	}

	var obj secretObject
	if ok {
		obj = existing
	} else {
		obj = secretObject{
			Service: id.Service,
			Key:     fmt.Sprintf("/%s/%s", id.Service, id.Key),
			Values:  map[int]secretVersion{},
		}
	}

	thisVersion := getLatestVersion(obj.Values) + 1
	user, err := s.getCurrentUser(ctx)
	if err != nil {
		return err
	}
	obj.Values[thisVersion] = secretVersion{
		Version:   thisVersion,
		Value:     value,
		Created:   time.Now().UTC(),
		CreatedBy: user,
	}

	pruneOldVersions(obj.Values)

	contents, err := json.Marshal(obj)
	if err != nil {
		return err
	}

	putObjectInput := &s3.PutObjectInput{
		Bucket:               aws.String(s.bucket),
		ServerSideEncryption: types.ServerSideEncryptionAwsKms,
		SSEKMSKeyId:          aws.String(s.kmsKeyAlias),
		Key:                  aws.String(objPath),
		Body:                 bytes.NewReader(contents),
	}

	_, err = s.svc.PutObject(ctx, putObjectInput)
	if err != nil {
		// TODO: catch specific awserr
		return err
	}

	index.Latest[id.Key] = LatestValue{
		Version:  thisVersion,
		Value:    value,
		KMSAlias: s.kmsKeyAlias,
	}
	return s.writeLatest(ctx, id.Service, index)
}

func (s *S3KMSStore) ListServices(ctx context.Context, service string, includeSecretName bool) ([]string, error) {
	return nil, fmt.Errorf("S3KMS Backend is experimental and does not implement this command")
}

func (s *S3KMSStore) List(ctx context.Context, service string, includeValues bool) ([]Secret, error) {
	index, err := s.readLatest(ctx, service)
	if err != nil {
		return []Secret{}, err
	}

	secrets := []Secret{}
	for key := range index.Latest {
		obj, ok, err := s.readObjectById(ctx, SecretId{Service: service, Key: key})
		if err != nil {
			return []Secret{}, err
		}
		if !ok {
			return []Secret{}, ErrSecretNotFound
		}
		version := getLatestVersion(obj.Values)

		val, ok := obj.Values[version]
		if !ok {
			return []Secret{}, ErrSecretNotFound
		}

		s := Secret{
			Meta: SecretMetadata{
				Created:   val.Created,
				CreatedBy: val.CreatedBy,
				Version:   val.Version,
				Key:       obj.Key,
			},
		}

		if includeValues {
			s.Value = &val.Value
		}
		secrets = append(secrets, s)

	}

	return secrets, nil
}

// ListRaw returns RawSecrets by extracting them from the index file. It only ever uses the
// index file; it never consults the actual secrets, so if the index file is out of sync, these
// results will reflect that.
func (s *S3KMSStore) ListRaw(ctx context.Context, service string) ([]RawSecret, error) {
	index, err := s.readLatest(ctx, service)
	if err != nil {
		return []RawSecret{}, err
	}

	// Read raw secrets directly from the index file (which caches the latest values)
	secrets := []RawSecret{}
	for key := range index.Latest {
		s := RawSecret{
			Key:   fmt.Sprintf("/%s/%s", service, key),
			Value: index.Latest[key].Value,
		}
		secrets = append(secrets, s)
	}

	return secrets, nil
}

func (s *S3KMSStore) Delete(ctx context.Context, id SecretId) error {
	index, err := s.readLatest(ctx, id.Service)
	if err != nil {
		return err
	}

	if val, ok := index.Latest[id.Key]; val.KMSAlias != s.kmsKeyAlias && ok {
		return fmt.Errorf("Unable to overwrite secret %s using new KMS key %s; mismatch with existing key %s", id.Key, s.kmsKeyAlias, val.KMSAlias)
	}

	if _, ok := index.Latest[id.Key]; ok {
		delete(index.Latest, id.Key)
	}

	if err := s.deleteObjectById(ctx, id); err != nil {
		return err
	}

	return s.writeLatest(ctx, id.Service, index)
}

func (s *S3KMSStore) readObject(ctx context.Context, path string) (secretObject, bool, error) {
	getObjectInput := &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	}

	resp, err := s.svc.GetObject(ctx, getObjectInput)
	if err != nil {
		// handle specific AWS  errors
		var nsb *types.NoSuchBucket
		if errors.As(err, &nsb) {
			return secretObject{}, false, err
		}
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			return secretObject{}, false, nil
		}
		// generic errors
		return secretObject{}, false, err
	}

	raw, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return secretObject{}, false, err
	}

	var obj secretObject
	if err := json.Unmarshal(raw, &obj); err != nil {
		return secretObject{}, false, err
	}

	return obj, true, nil

}

func (s *S3KMSStore) puts3raw(ctx context.Context, path string, contents []byte) error {
	putObjectInput := &s3.PutObjectInput{
		Bucket:               aws.String(s.bucket),
		ServerSideEncryption: types.ServerSideEncryptionAwsKms,
		SSEKMSKeyId:          aws.String(s.kmsKeyAlias),
		Key:                  aws.String(path),
		Body:                 bytes.NewReader(contents),
	}

	_, err := s.svc.PutObject(ctx, putObjectInput)
	return err
}

func (s *S3KMSStore) readLatestFile(ctx context.Context, path string) (LatestIndexFile, error) {
	getObjectInput := &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	}

	resp, err := s.svc.GetObject(ctx, getObjectInput)

	if err != nil {
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			// Index doesn't exist yet, return an empty index
			return LatestIndexFile{Latest: map[string]LatestValue{}}, nil
		}
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if apiErr.ErrorCode() == "AccessDenied" {
				// If we're not able to read the latest index for a KMS Key then proceed like it doesn't exist.
				// We do this because in a chamber secret folder there might be other secrets written with a KMS Key that you don't have access to.
				return LatestIndexFile{Latest: map[string]LatestValue{}}, nil
			}
		}
		return LatestIndexFile{}, err
	}

	raw, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return LatestIndexFile{}, err
	}

	var index LatestIndexFile
	if err := json.Unmarshal(raw, &index); err != nil {
		return LatestIndexFile{}, err
	}

	return index, nil
}

func (s *S3KMSStore) readLatest(ctx context.Context, service string) (LatestIndexFile, error) {
	// Create an empty latest, this will be used to merge together the various KMS Latest Files
	latestResult := LatestIndexFile{Latest: map[string]LatestValue{}}

	// List all the files that are prefixed with kms and use them as latest.json files for that KMS Key.
	params := &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(fmt.Sprintf("%s/__kms", service)),
	}

	var paginationError error
	paginator := s3.NewListObjectsV2Paginator(s.svc, params)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return latestResult, err
		}

		for index := range page.Contents {
			key_name := *page.Contents[index].Key
			result, err := s.readLatestFile(ctx, key_name)

			if err != nil {
				paginationError = errors.New(fmt.Sprintf("Error reading latest index for KMS Key (%s): %s", key_name, err))
				break
			}

			// Check if the chamber key already exists in the index.Latest map.
			// Prefer the most recent version.
			for k, v := range result.Latest {
				if val, ok := latestResult.Latest[k]; ok {
					if val.Version > v.Version {
						latestResult.Latest[k] = val
					} else {
						latestResult.Latest[k] = v
					}
				} else {
					latestResult.Latest[k] = v
				}
			}
		}
	}

	if paginationError != nil {
		return latestResult, paginationError
	}

	return latestResult, nil
}

func (s *S3KMSStore) latestFileKeyNameByKMSKey() string {
	return fmt.Sprintf("__kms_%s__latest.json", strings.Replace(s.kmsKeyAlias, "/", "_", -1))
}

func (s *S3KMSStore) writeLatest(ctx context.Context, service string, index LatestIndexFile) error {
	path := fmt.Sprintf("%s/%s", service, s.latestFileKeyNameByKMSKey())
	for k, v := range index.Latest {
		if v.KMSAlias != s.kmsKeyAlias {
			delete(index.Latest, k)
		}
	}

	raw, err := json.Marshal(index)
	if err != nil {
		return err
	}

	return s.puts3raw(ctx, path, raw)
}
