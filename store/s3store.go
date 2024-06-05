package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

const (
	MaximumVersions = 100

	latestObjectName = "__latest.json"
)

// secretObject is the serialized format for storing secrets
// as an s3 object
type secretObject struct {
	Service string                `json:"service"`
	Key     string                `json:"key"`
	Values  map[int]secretVersion `json:"values"`
}

// secretVersion holds all the metadata for a specific version
// of a secret
type secretVersion struct {
	Created   time.Time `json:"created"`
	CreatedBy string    `json:"created_by"`
	Version   int       `json:"version"`
	Value     string    `json:"value"`
}

// latest is used to keep a single object in s3 with all of the
// most recent values for the given service's secrets.  Keeping this
// in a single s3 object allows us to use a single s3 GetObject
// for ListRaw (and thus chamber exec).
type latest struct {
	Latest map[string]string `json:"latest"`
}

var _ Store = &S3Store{}

type S3Store struct {
	svc    apiS3
	stsSvc apiSTS
	bucket string
}

func NewS3StoreWithBucket(ctx context.Context, numRetries int, bucket string) (*S3Store, error) {
	config, _, err := getConfig(ctx, numRetries, aws.RetryModeStandard)
	if err != nil {
		return nil, err
	}

	svc := s3.NewFromConfig(config)

	stsSvc := sts.NewFromConfig(config)

	return &S3Store{
		svc:    svc,
		stsSvc: stsSvc,
		bucket: bucket,
	}, nil
}

func (s *S3Store) Write(ctx context.Context, id SecretId, value string) error {
	index, err := s.readLatest(ctx, id.Service)
	if err != nil {
		return err
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
		ServerSideEncryption: types.ServerSideEncryptionAes256,
		Key:                  aws.String(objPath),
		Body:                 bytes.NewReader(contents),
	}

	_, err = s.svc.PutObject(ctx, putObjectInput)
	if err != nil {
		// TODO: catch specific awserr
		return err
	}

	index.Latest[id.Key] = value
	return s.writeLatest(ctx, id.Service, index)
}

func (s *S3Store) Read(ctx context.Context, id SecretId, version int) (Secret, error) {
	obj, ok, err := s.readObjectById(ctx, id)
	if err != nil {
		return Secret{}, err
	}

	if !ok {
		return Secret{}, ErrSecretNotFound
	}

	if version == -1 {
		version = getLatestVersion(obj.Values)
	}
	val, ok := obj.Values[version]
	if !ok {
		return Secret{}, ErrSecretNotFound
	}

	return Secret{
		Value: aws.String(val.Value),
		Meta: SecretMetadata{
			Created:   val.Created,
			CreatedBy: val.CreatedBy,
			Version:   val.Version,
			Key:       obj.Key,
		},
	}, nil
}

func (s *S3Store) ListServices(ctx context.Context, service string, includeSecretName bool) ([]string, error) {
	return nil, fmt.Errorf("S3 Backend is experimental and does not implement this command")
}

func (s *S3Store) List(ctx context.Context, service string, includeValues bool) ([]Secret, error) {
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

func (s *S3Store) ListRaw(ctx context.Context, service string) ([]RawSecret, error) {
	index, err := s.readLatest(ctx, service)
	if err != nil {
		return []RawSecret{}, err
	}

	secrets := []RawSecret{}
	for key, value := range index.Latest {
		s := RawSecret{
			Key:   fmt.Sprintf("/%s/%s", service, key),
			Value: value,
		}
		secrets = append(secrets, s)

	}

	return secrets, nil
}

func (s *S3Store) History(ctx context.Context, id SecretId) ([]ChangeEvent, error) {
	obj, ok, err := s.readObjectById(ctx, id)
	if err != nil {
		return []ChangeEvent{}, err
	}

	if !ok {
		return []ChangeEvent{}, ErrSecretNotFound
	}

	events := []ChangeEvent{}

	for ix, secretVersion := range obj.Values {
		events = append(events, ChangeEvent{
			Type:    getChangeType(ix),
			Time:    secretVersion.Created,
			User:    secretVersion.CreatedBy,
			Version: secretVersion.Version,
		})
	}

	// Sort events by version
	sort.Slice(events, func(i, j int) bool {
		return events[i].Version < events[j].Version
	})
	return events, nil
}

func (s *S3Store) Delete(ctx context.Context, id SecretId) error {
	index, err := s.readLatest(ctx, id.Service)
	if err != nil {
		return err
	}

	if _, ok := index.Latest[id.Key]; ok {
		delete(index.Latest, id.Key)
	}

	if err := s.deleteObjectById(ctx, id); err != nil {
		return err
	}

	return s.writeLatest(ctx, id.Service, index)
}

// getCurrentUser uses the STS API to get the current caller identity,
// so that secret value changes can be correctly attributed to the right
// aws user/role
func (s *S3Store) getCurrentUser(ctx context.Context) (string, error) {
	resp, err := s.stsSvc.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", err
	}

	return *resp.Arn, nil
}

func (s *S3Store) deleteObjectById(ctx context.Context, id SecretId) error {
	path := getObjectPath(id)
	return s.deleteObject(ctx, path)
}

func (s *S3Store) deleteObject(ctx context.Context, path string) error {
	deleteObjectInput := &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	}

	_, err := s.svc.DeleteObject(ctx, deleteObjectInput)
	return err
}

func (s *S3Store) readObject(ctx context.Context, path string) (secretObject, bool, error) {
	getObjectInput := &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	}

	resp, err := s.svc.GetObject(ctx, getObjectInput)
	if err != nil {
		// handle specific AWS errors
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

func (s *S3Store) readObjectById(ctx context.Context, id SecretId) (secretObject, bool, error) {
	path := getObjectPath(id)
	return s.readObject(ctx, path)
}

func (s *S3Store) puts3raw(ctx context.Context, path string, contents []byte) error {
	putObjectInput := &s3.PutObjectInput{
		Bucket:               aws.String(s.bucket),
		ServerSideEncryption: types.ServerSideEncryptionAes256,
		Key:                  aws.String(path),
		Body:                 bytes.NewReader(contents),
	}

	_, err := s.svc.PutObject(ctx, putObjectInput)
	return err
}

func (s *S3Store) readLatest(ctx context.Context, service string) (latest, error) {
	path := fmt.Sprintf("%s/%s", service, latestObjectName)

	getObjectInput := &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	}

	resp, err := s.svc.GetObject(ctx, getObjectInput)
	if err != nil {
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			// Index doesn't exist yet, return an empty index
			return latest{Latest: map[string]string{}}, nil
		}
		return latest{}, err
	}

	raw, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return latest{}, err
	}

	var index latest
	if err := json.Unmarshal(raw, &index); err != nil {
		return latest{}, err
	}

	return index, nil
}

func (s *S3Store) writeLatest(ctx context.Context, service string, index latest) error {
	path := fmt.Sprintf("%s/%s", service, latestObjectName)

	raw, err := json.Marshal(index)
	if err != nil {
		return err
	}

	return s.puts3raw(ctx, path, raw)
}

func stringInSlice(val string, sl []string) bool {
	for _, v := range sl {
		if v == val {
			return true
		}
	}
	return false
}

func getObjectPath(id SecretId) string {
	return fmt.Sprintf("%s/%s.json", id.Service, id.Key)
}

func getLatestVersion(m map[int]secretVersion) int {
	max := 0
	for k := range m {
		if k > max {
			max = k
		}
	}
	return max
}

func pruneOldVersions(m map[int]secretVersion) {
	newest := getLatestVersion(m)

	for version := range m {
		if version < newest-MaximumVersions {
			delete(m, version)
		}
	}
}
