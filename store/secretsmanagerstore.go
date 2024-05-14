// Secrets Manager Store is maintained by Dan MacTough https://github.com/danmactough. Thanks Dan!
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// We store all Chamber metadata in a stringified JSON format,
// in a field named "_chamber_metadata"
const metadataKey = "_chamber_metadata"

// secretValueObject is the serialized format for storing secrets
// as a SecretsManager SecretValue
type secretValueObject map[string]string

// We use a custom unmarshaller to provide better interoperability
// with Secrets Manager secrets that are created/managed outside Chamber
// For example, when creating secrets for an RDS instance,
// the "port" is stored as a number, so we need to convert it to a string.
// So we handle converting numbers and also booleans to strings.
func (o *secretValueObject) UnmarshalJSON(b []byte) error {
	var v map[string]interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}

	res := secretValueObject{}

	for key, value := range v {
		var s string
		switch value := reflect.ValueOf(value); value.Kind() {
		case reflect.String:
			s = value.String()
		case reflect.Float64:
			s = strconv.FormatFloat(value.Float(), 'f', -1, 64)
		case reflect.Bool:
			s = strconv.FormatBool(value.Bool())
		default:
			s = ""
		}
		res[key] = s
	}
	*o = res
	return nil
}

// secretValueObjectMetadata holds all the metadata for all the secrets
// keyed by the name of the secret
type secretValueObjectMetadata map[string]secretMetadata

// secretMetadata holds all the metadata for a specific version
// of a specific secret
type secretMetadata struct {
	Created   time.Time `json:"created"`
	CreatedBy string    `json:"created_by"`
	Version   int       `json:"version"`
}

// ensure SecretsManagerStore confirms to Store interface
var _ Store = &SecretsManagerStore{}

// SecretsManagerStore implements the Store interface for storing secrets in SSM Parameter
// Store
type SecretsManagerStore struct {
	svc    apiSecretsManager
	stsSvc apiSTS
	config aws.Config
}

// NewSecretsManagerStore creates a new SecretsManagerStore
func NewSecretsManagerStore(ctx context.Context, numRetries int) (*SecretsManagerStore, error) {
	cfg, _, err := getConfig(ctx, numRetries, aws.RetryModeStandard)
	if err != nil {
		return nil, err
	}

	svc := secretsmanager.NewFromConfig(cfg)

	stsSvc := sts.NewFromConfig(cfg)

	return &SecretsManagerStore{
		svc:    svc,
		stsSvc: stsSvc,
		config: cfg,
	}, nil
}

// Write writes a given value to a secret identified by id. If the secret
// already exists, then write a new version.
func (s *SecretsManagerStore) Write(ctx context.Context, id SecretId, value string) error {
	version := 1
	// first read to get the current version
	latest, err := s.readLatest(ctx, id.Service)
	mustCreate := false
	deleteKeyFromSecret := len(value) == 0

	// Failure to readLatest may be a true error or an expected error.
	// We expect that when we write a secret that it may not already exist:
	// that's the secretsmanager.ErrCodeResourceNotFoundException.
	if err != nil {
		// However, if the operation is to deleteKeyFromSecret and there's either
		// a true error or the secret does not yet exist, that's true error because
		// we cannot delete something that does not exist.
		if deleteKeyFromSecret {
			return err
		}
		if err != ErrSecretNotFound {
			var rnfe *types.ResourceNotFoundException
			if errors.As(err, &rnfe) {
				mustCreate = true
			} else {
				return err
			}
		}
	}

	if deleteKeyFromSecret {
		if _, ok := latest[id.Key]; ok {
			delete(latest, id.Key)
		} else {
			return ErrSecretNotFound
		}
		metadata, err := getHydratedMetadata(&latest)
		if err != nil {
			return err
		}
		if _, ok := metadata[id.Key]; ok {
			delete(metadata, id.Key)
		}

		rawMetadata, err := dehydrateMetadata(&metadata)
		if err != nil {
			return err
		}
		latest[metadataKey] = rawMetadata
	} else {
		user, err := s.getCurrentUser(ctx)
		if err != nil {
			return err
		}

		metadata, err := getHydratedMetadata(&latest)
		if err != nil {
			return err
		}

		if keyMetadata, ok := metadata[id.Key]; ok {
			version = keyMetadata.Version + 1
		}

		metadata[id.Key] = secretMetadata{
			Version:   version,
			Created:   time.Now().UTC(),
			CreatedBy: user,
		}

		rawMetadata, err := dehydrateMetadata(&metadata)
		if err != nil {
			return err
		}

		latest[id.Key] = value
		latest[metadataKey] = rawMetadata
	}

	contents, err := json.Marshal(latest)
	if err != nil {
		return err
	}

	if mustCreate {
		createSecretValueInput := &secretsmanager.CreateSecretInput{
			Name:         aws.String(id.Service),
			SecretString: aws.String(string(contents)),
		}
		_, err = s.svc.CreateSecret(ctx, createSecretValueInput)
		if err != nil {
			return err
		}
	} else {
		// Check that rotation is not enabled. We refuse to write to secrets with
		// rotation enabled.
		describeSecretInput := &secretsmanager.DescribeSecretInput{
			SecretId: aws.String(id.Service),
		}
		details, err := s.svc.DescribeSecret(ctx, describeSecretInput)
		if err != nil {
			return err
		}
		if details.RotationEnabled != nil && *details.RotationEnabled {
			return fmt.Errorf("Cannot write to a secret with rotation enabled")
		}

		putSecretValueInput := &secretsmanager.PutSecretValueInput{
			SecretId:      aws.String(id.Service),
			SecretString:  aws.String(string(contents)),
			VersionStages: []string{"AWSCURRENT", "CHAMBER" + fmt.Sprint(version)},
		}
		_, err = s.svc.PutSecretValue(ctx, putSecretValueInput)
		if err != nil {
			return err
		}
	}

	return nil
}

// Read reads a secret at a specific version.
// To grab the latest version, use -1 as the version number.
func (s *SecretsManagerStore) Read(ctx context.Context, id SecretId, version int) (Secret, error) {
	if version == -1 {
		latest, err := s.readLatest(ctx, id.Service)
		if err != nil {
			return Secret{}, err
		}

		value, ok := latest[id.Key]
		if !ok {
			return Secret{}, ErrSecretNotFound
		}

		keyMetadata, err := getHydratedKeyMetadata(&latest, &id.Key)
		if err != nil {
			return Secret{}, err
		}

		return Secret{
			Value: &value,
			Meta: SecretMetadata{
				Created:   keyMetadata.Created,
				CreatedBy: keyMetadata.CreatedBy,
				Version:   keyMetadata.Version,
				Key:       id.Key,
			},
		}, nil

	}
	return s.readVersion(ctx, id, version)
}

// Delete removes a secret. Note this removes all versions of the secret. (True?)
func (s *SecretsManagerStore) Delete(ctx context.Context, id SecretId) error {
	// delegate to Write
	return s.Write(ctx, id, "")
}

func (s *SecretsManagerStore) readVersion(ctx context.Context, id SecretId, version int) (Secret, error) {
	listSecretVersionIdsInput := &secretsmanager.ListSecretVersionIdsInput{
		SecretId:          aws.String(id.Service),
		IncludeDeprecated: aws.Bool(false),
	}

	var result Secret
	resp, err := s.svc.ListSecretVersionIds(ctx, listSecretVersionIdsInput)
	if err != nil {
		return Secret{}, err
	}

	for _, history := range resp.Versions {
		h := history
		thisVersion := 0

		getSecretValueInput := &secretsmanager.GetSecretValueInput{
			SecretId:  aws.String(id.Service),
			VersionId: h.VersionId,
		}

		resp, err := s.svc.GetSecretValue(ctx, getSecretValueInput)

		if err != nil {
			return Secret{}, err
		}

		if len(*resp.SecretString) == 0 {
			continue
		}

		var historyItem secretValueObject
		if historyItem, err = jsonToSecretValueObject(*resp.SecretString); err != nil {
			return Secret{}, err
		}

		keyMetadata, err := getHydratedKeyMetadata(&historyItem, &id.Key)
		if err != nil {
			return Secret{}, err
		}

		thisVersion = keyMetadata.Version

		if thisVersion == version {
			thisValue, ok := historyItem[id.Key]
			if !ok {
				return Secret{}, ErrSecretNotFound
			}
			result = Secret{
				Value: &thisValue,
				Meta: SecretMetadata{
					Created:   keyMetadata.Created,
					CreatedBy: keyMetadata.CreatedBy,
					Version:   thisVersion,
					Key:       id.Key,
				},
			}
			break
		}
	}

	if result.Value != nil {
		return result, nil
	}

	return Secret{}, ErrSecretNotFound
}

func (s *SecretsManagerStore) readLatest(ctx context.Context, service string) (secretValueObject, error) {
	getSecretValueInput := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(service),
	}

	resp, err := s.svc.GetSecretValue(ctx, getSecretValueInput)

	if err != nil {
		return secretValueObject{}, err
	}

	if len(*resp.SecretString) == 0 {
		return secretValueObject{}, ErrSecretNotFound
	}

	var obj secretValueObject
	if obj, err = jsonToSecretValueObject(*resp.SecretString); err != nil {
		return secretValueObject{}, err
	}

	return obj, nil
}

// ListServices (not implemented)
func (s *SecretsManagerStore) ListServices(ctx context.Context, service string, includeSecretName bool) ([]string, error) {
	return nil, fmt.Errorf("Secrets Manager Backend is experimental and does not implement this command")
}

// List lists all secrets for a given service.  If includeValues is true,
// then those secrets are decrypted and returned, otherwise only the metadata
// about a secret is returned.
func (s *SecretsManagerStore) List(ctx context.Context, serviceName string, includeValues bool) ([]Secret, error) {
	secrets := map[string]Secret{}

	latest, err := s.readLatest(ctx, serviceName)
	if err != nil {
		return nil, err
	}

	metadata, err := getHydratedMetadata(&latest)
	if err != nil {
		return nil, err
	}

	for key, value := range latest {
		if key == metadataKey {
			continue
		}

		keyMetadata, ok := metadata[key]
		if !ok {
			keyMetadata = secretMetadata{}
		}

		secret := Secret{
			Value: nil,
			Meta: SecretMetadata{
				Created:   keyMetadata.Created,
				CreatedBy: keyMetadata.CreatedBy,
				Version:   keyMetadata.Version,
				Key:       key,
			},
		}
		if includeValues {
			v := value
			secret.Value = &v
		}
		secrets[key] = secret
	}

	return values(secrets), nil
}

// ListRaw lists all secrets keys and values for a given service. Does not include any
// other metadata. Suitable for use in production environments.
func (s *SecretsManagerStore) ListRaw(ctx context.Context, serviceName string) ([]RawSecret, error) {
	latest, err := s.readLatest(ctx, serviceName)
	if err != nil {
		return nil, err
	}

	rawSecrets := make([]RawSecret, len(latest))
	i := 0
	for key, value := range latest {
		// v := value
		rawSecrets[i] = RawSecret{
			Value: value,
			Key:   key,
		}
		i++
	}
	return rawSecrets, nil
}

// History returns a list of events that have occurred regarding the given
// secret.
func (s *SecretsManagerStore) History(ctx context.Context, id SecretId) ([]ChangeEvent, error) {
	events := []ChangeEvent{}

	listSecretVersionIdsInput := &secretsmanager.ListSecretVersionIdsInput{
		SecretId:          aws.String(id.Service),
		IncludeDeprecated: aws.Bool(false),
	}

	resp, err := s.svc.ListSecretVersionIds(ctx, listSecretVersionIdsInput)
	if err != nil {
		return events, err
	}

	// m is a temporary map to allow us to (1) deduplicate ChangeEvents, since
	// saving the secret only increments the Version of the Key being created or
	// modified, and (2) sort the ChangeEvents by Version
	m := make(map[int]*ChangeEvent)

	for _, history := range resp.Versions {
		h := history
		getSecretValueInput := &secretsmanager.GetSecretValueInput{
			SecretId:  aws.String(id.Service),
			VersionId: h.VersionId,
		}

		resp, err := s.svc.GetSecretValue(ctx, getSecretValueInput)

		if err != nil {
			return events, err
		}

		if len(*resp.SecretString) == 0 {
			continue
		}

		var historyItem secretValueObject
		if historyItem, err = jsonToSecretValueObject(*resp.SecretString); err != nil {
			return events, err
		}

		metadata, err := getHydratedMetadata(&historyItem)
		if err != nil {
			return nil, err
		}

		keyMetadata, ok := metadata[id.Key]
		if !ok {
			continue
		}

		thisVersion := keyMetadata.Version

		// This is where we deduplicate
		if _, ok := m[thisVersion]; !ok {
			m[thisVersion] = &ChangeEvent{
				Type:    getChangeType(thisVersion),
				Time:    keyMetadata.Created,
				User:    keyMetadata.CreatedBy,
				Version: thisVersion,
			}
		}
	}

	if len(m) == 0 {
		return events, ErrSecretNotFound
	}

	keys := make([]int, 0)
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	for _, k := range keys {
		events = append(events, *m[k])
	}
	return events, nil
}

func (s *SecretsManagerStore) getCurrentUser(ctx context.Context) (string, error) {
	resp, err := s.stsSvc.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", err
	}

	return *resp.Arn, nil
}

func getHydratedMetadata(raw *secretValueObject) (secretValueObjectMetadata, error) {
	r := *raw
	rawMetadata, ok := r[metadataKey]
	if !ok {
		return secretValueObjectMetadata{}, nil
	}
	return rehydrateMetadata(&rawMetadata)
}

func getHydratedKeyMetadata(raw *secretValueObject, key *string) (secretMetadata, error) {
	metadata, err := getHydratedMetadata(raw)
	if err != nil {
		return secretMetadata{}, err
	}

	keyMetadata, ok := metadata[*key]
	if !ok {
		return secretMetadata{}, nil
	}
	return keyMetadata, nil
}

func rehydrateMetadata(rawMetadata *string) (secretValueObjectMetadata, error) {
	var metadata secretValueObjectMetadata
	err := json.Unmarshal([]byte(*rawMetadata), &metadata)
	if err != nil {
		return secretValueObjectMetadata{}, err
	}
	return metadata, nil
}

func dehydrateMetadata(metadata *secretValueObjectMetadata) (string, error) {
	rawMetadata, err := json.Marshal(metadata)
	if err != nil {
		return "", err
	}
	return string(rawMetadata), nil
}

func jsonToSecretValueObject(s string) (secretValueObject, error) {
	var obj secretValueObject
	err := json.Unmarshal([]byte(s), &obj)
	if err != nil {
		return secretValueObject{}, err
	}
	return obj, nil
}
