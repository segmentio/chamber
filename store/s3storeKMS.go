package store

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/pkg/errors"
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
	svc         s3iface.S3API
	stsSvc      *sts.STS
	bucket      string
	kmsKeyAlias string
}

func NewS3KMSStore(numRetries int, bucket string, kmsKeyAlias string) (*S3KMSStore, error) {
	session, region, err := getSession(numRetries)
	if err != nil {
		return nil, err
	}

	svc := s3.New(session, &aws.Config{
		MaxRetries: aws.Int(numRetries),
		Region:     region,
	})

	stsSvc := sts.New(session, &aws.Config{
		MaxRetries: aws.Int(numRetries),
		Region:     region,
	})

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

func (s *S3KMSStore) Write(id SecretId, value string) error {
	index, err := s.readLatest(id.Service)
	if err != nil {
		return err
	}

	if val, ok := index.Latest[id.Key]; val.KMSAlias != s.kmsKeyAlias && ok {
		return fmt.Errorf("Unable to overwrite secret %s using new KMS key %s; mismatch with existing key %s", id.Key, s.kmsKeyAlias, val.KMSAlias)
	}

	objPath := getObjectPath(id)
	existing, ok, err := s.readObjectById(id)
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
	user, err := s.getCurrentUser()
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
		ServerSideEncryption: aws.String(s3.ServerSideEncryptionAwsKms),
		SSEKMSKeyId:          aws.String(s.kmsKeyAlias),
		Key:                  aws.String(objPath),
		Body:                 bytes.NewReader(contents),
	}

	_, err = s.svc.PutObject(putObjectInput)
	if err != nil {
		// TODO: catch specific awserr
		return err
	}

	index.Latest[id.Key] = LatestValue{
		Version:  thisVersion,
		Value:    value,
		KMSAlias: s.kmsKeyAlias,
	}
	return s.writeLatest(id.Service, index)
}

func (s *S3KMSStore) List(service string, includeValues bool) ([]Secret, error) {
	index, err := s.readLatest(service)
	if err != nil {
		return []Secret{}, err
	}

	secrets := []Secret{}
	for key := range index.Latest {
		obj, ok, err := s.readObjectById(SecretId{Service: service, Key: key})
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

func (s *S3KMSStore) ListRaw(service string) ([]RawSecret, error) {
	index, err := s.readLatest(service)
	if err != nil {
		return []RawSecret{}, err
	}

	secrets := []RawSecret{}
	for key, value := range index.Latest {
		s := RawSecret{
			Key:   fmt.Sprintf("/%s/%s", service, key),
			Value: value.Value,
		}
		secrets = append(secrets, s)

	}

	return secrets, nil
}

func (s *S3KMSStore) Delete(id SecretId) error {
	index, err := s.readLatest(id.Service)
	if err != nil {
		return err
	}

	if val, ok := index.Latest[id.Key]; val.KMSAlias != s.kmsKeyAlias && ok {
		return fmt.Errorf("Unable to overwrite secret %s using new KMS key %s; mismatch with existing key %s", id.Key, s.kmsKeyAlias, val.KMSAlias)
	}

	if _, ok := index.Latest[id.Key]; ok {
		delete(index.Latest, id.Key)
	}

	if err := s.deleteObjectById(id); err != nil {
		return err
	}

	return s.writeLatest(id.Service, index)
}

func (s *S3KMSStore) readObject(path string) (secretObject, bool, error) {
	getObjectInput := &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	}

	resp, err := s.svc.GetObject(getObjectInput)
	if err != nil {
		// handle aws errors
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case s3.ErrCodeNoSuchBucket:
				return secretObject{}, false, err
			case s3.ErrCodeNoSuchKey:
				return secretObject{}, false, nil
			default:
				return secretObject{}, false, err
			}
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

func (s *S3KMSStore) readObjectById(id SecretId) (secretObject, bool, error) {
	path := getObjectPath(id)
	return s.readObject(path)
}

func (s *S3KMSStore) puts3raw(path string, contents []byte) error {
	putObjectInput := &s3.PutObjectInput{
		Bucket:               aws.String(s.bucket),
		ServerSideEncryption: aws.String(s3.ServerSideEncryptionAwsKms),
		SSEKMSKeyId:          aws.String(s.kmsKeyAlias),
		Key:                  aws.String(path),
		Body:                 bytes.NewReader(contents),
	}

	_, err := s.svc.PutObject(putObjectInput)
	return err
}

func (s *S3KMSStore) readLatestFile(path string) (LatestIndexFile, error) {
	getObjectInput := &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	}

	resp, err := s.svc.GetObject(getObjectInput)

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			if aerr.Code() == "AccessDenied" {
				// If we're not able to read the latest index for a KMS Key then proceed like it doesn't exist.
				// We do this because in a chamber secret folder there might be other secrets written with a KMS Key that you don't have access to.
				return LatestIndexFile{Latest: map[string]LatestValue{}}, nil
			}

			if aerr.Code() == s3.ErrCodeNoSuchKey {
				// Index doesn't exist yet, return an empty index
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

func (s *S3KMSStore) readLatest(service string) (LatestIndexFile, error) {
	// Create an empty latest, this will be used to merge together the various KMS Latest Files
	latestResult := LatestIndexFile{Latest: map[string]LatestValue{}}

	// List all the files that are prefixed with kms and use them as latest.json files for that KMS Key.
	params := &s3.ListObjectsInput{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(fmt.Sprintf("%s/__kms", service)),
	}

	var paginationError error

	err := s.svc.ListObjectsPages(params, func(page *s3.ListObjectsOutput, lastPage bool) bool {
		for index := range page.Contents {
			key_name := *page.Contents[index].Key
			result, err := s.readLatestFile(key_name)

			if err != nil {
				paginationError = errors.New(fmt.Sprintf("Error reading latest index for KMS Key (%s): %s", key_name, err))
				return false
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

		return !lastPage
	})

	if paginationError != nil {
		return latestResult, paginationError
	}

	if err != nil {
		return latestResult, err
	}

	return latestResult, nil
}

func (s *S3KMSStore) latestFileKeyNameByKMSKey() string {
	return fmt.Sprintf("__kms_%s__latest.json", strings.Replace(s.kmsKeyAlias, "/", "_", -1))
}

func (s *S3KMSStore) writeLatest(service string, index LatestIndexFile) error {
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

	return s.puts3raw(path, raw)
}
