package store

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/aws/aws-sdk-go/service/secretsmanager/secretsmanageriface"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
	"github.com/stretchr/testify/assert"
)

type mockSecretsManagerClient struct {
	secretsmanageriface.SecretsManagerAPI
	secrets map[string]mockSecret
	outputs map[string]secretsmanager.DescribeSecretOutput
}

type mockSecret struct {
	currentSecret *secretValueObject
	history       map[string]*secretValueObject
}

func (m *mockSecretsManagerClient) PutSecretValue(i *secretsmanager.PutSecretValueInput) (*secretsmanager.PutSecretValueOutput, error) {
	current, ok := m.secrets[*i.SecretId]
	if !ok {
		return &secretsmanager.PutSecretValueOutput{}, ErrSecretNotFound
	}

	secret, err := jsonToSecretValueObject(*i.SecretString)
	if err != nil {
		return &secretsmanager.PutSecretValueOutput{}, err
	}

	current.currentSecret = &secret
	current.history[uniqueID()] = &secret

	m.secrets[*i.SecretId] = current

	return &secretsmanager.PutSecretValueOutput{}, nil
}

func (m *mockSecretsManagerClient) CreateSecret(i *secretsmanager.CreateSecretInput) (*secretsmanager.CreateSecretOutput, error) {
	secret, err := jsonToSecretValueObject(*i.SecretString)
	if err != nil {
		return &secretsmanager.CreateSecretOutput{}, err
	}

	current := mockSecret{
		currentSecret: &secret,
		history:       make(map[string]*secretValueObject),
	}
	current.history[uniqueID()] = &secret

	m.secrets[*i.Name] = current

	return &secretsmanager.CreateSecretOutput{}, nil
}

func (m *mockSecretsManagerClient) GetSecretValue(i *secretsmanager.GetSecretValueInput) (*secretsmanager.GetSecretValueOutput, error) {
	var version *secretValueObject

	if i.VersionId != nil {
		historyItem, ok := m.secrets[*i.SecretId].history[*i.VersionId]
		if !ok {
			return &secretsmanager.GetSecretValueOutput{}, awserr.New(secretsmanager.ErrCodeResourceNotFoundException, secretsmanager.ErrCodeResourceNotFoundException, ErrSecretNotFound)
		}
		version = historyItem
	} else {
		current, ok := m.secrets[*i.SecretId]
		if !ok {
			return &secretsmanager.GetSecretValueOutput{}, awserr.New(secretsmanager.ErrCodeResourceNotFoundException, secretsmanager.ErrCodeResourceNotFoundException, ErrSecretNotFound)
		}
		version = current.currentSecret
	}

	s, err := json.Marshal(version)
	if err != nil {
		panic(err)
	}

	return &secretsmanager.GetSecretValueOutput{
		SecretString: aws.String(string(s)),
	}, nil
}

func (m *mockSecretsManagerClient) ListSecretVersionIds(i *secretsmanager.ListSecretVersionIdsInput) (*secretsmanager.ListSecretVersionIdsOutput, error) {
	service, ok := m.secrets[*i.SecretId]
	if !ok || len(service.history) == 0 {
		return &secretsmanager.ListSecretVersionIdsOutput{}, ErrSecretNotFound
	}

	Versions := make([]*secretsmanager.SecretVersionsListEntry, 0)
	for v := range service.history {
		Versions = append(Versions, &secretsmanager.SecretVersionsListEntry{VersionId: aws.String(v)})
	}

	return &secretsmanager.ListSecretVersionIdsOutput{Versions: Versions}, nil
}

func (m *mockSecretsManagerClient) DescribeSecret(i *secretsmanager.DescribeSecretInput) (*secretsmanager.DescribeSecretOutput, error) {
	output, ok := m.outputs[*i.SecretId]
	if !ok {
		return &secretsmanager.DescribeSecretOutput{RotationEnabled: aws.Bool(false)}, nil
	}
	return &output, nil
}

type mockSTSClient struct {
	stsiface.STSAPI
}

func (s *mockSTSClient) GetCallerIdentity(_ *sts.GetCallerIdentityInput) (*sts.GetCallerIdentityOutput, error) {
	return &sts.GetCallerIdentityOutput{
		Arn: aws.String("currentuser"),
	}, nil
}

func NewTestSecretsManagerStore(mock secretsmanageriface.SecretsManagerAPI) *SecretsManagerStore {
	stsSvc := &mockSTSClient{}
	return &SecretsManagerStore{
		svc:    mock,
		stsSvc: stsSvc,
	}
}

func TestSecretValueObjectUnmarshalling(t *testing.T) {
	t.Run("Unmarshalling JSON to a SecretValueObject converts non-string values", func(t *testing.T) {
		const j = `
		{
			"dbInstanceIdentifier": "database-1",
			"port": 3306,
			"isPhony": true,
			"empty": null,
			"nested": {
				"foo": "bar"
			},
			"array": [1,2,3]
		}
		`
		obj, err := jsonToSecretValueObject(j)
		assert.Nil(t, err)
		assert.Equal(t, "database-1", obj["dbInstanceIdentifier"])
		assert.Equal(t, "3306", obj["port"])
		assert.Equal(t, "true", obj["isPhony"])
		assert.Equal(t, "", obj["empty"])
		assert.Equal(t, "", obj["nested"])
		assert.Equal(t, "", obj["array"])
	})
}

func TestNewSecretsManagerStore(t *testing.T) {
	t.Run("Using region override should take precedence over other settings", func(t *testing.T) {
		os.Setenv("CHAMBER_AWS_REGION", "us-east-1")
		os.Setenv("AWS_REGION", "us-west-1")
		os.Setenv("AWS_DEFAULT_REGION", "us-west-2")

		s, err := NewSecretsManagerStore(1)
		assert.Nil(t, err)
		assert.Equal(t, "us-east-1", aws.StringValue(s.svc.(*secretsmanager.SecretsManager).Config.Region))
		os.Unsetenv("CHAMBER_AWS_REGION")
		os.Unsetenv("AWS_REGION")
		os.Unsetenv("AWS_DEFAULT_REGION")
	})

	t.Run("Should use AWS_REGION if it is set", func(t *testing.T) {
		os.Setenv("AWS_REGION", "us-west-1")

		s, err := NewSecretsManagerStore(1)
		assert.Nil(t, err)
		assert.Equal(t, "us-west-1", aws.StringValue(s.svc.(*secretsmanager.SecretsManager).Config.Region))

		os.Unsetenv("AWS_REGION")
	})

	t.Run("Should use CHAMBER_AWS_SSM_ENDPOINT if set", func(t *testing.T) {
		os.Setenv("CHAMBER_AWS_SSM_ENDPOINT", "mycustomendpoint")

		s, err := NewSecretsManagerStore(1)
		assert.Nil(t, err)
		endpoint, err := s.svc.(*secretsmanager.SecretsManager).Config.EndpointResolver.EndpointFor(endpoints.SecretsmanagerServiceID, endpoints.UsWest2RegionID)
		assert.Nil(t, err)
		assert.Equal(t, "mycustomendpoint", endpoint.URL)

		os.Unsetenv("CHAMBER_AWS_SSM_ENDPOINT")
	})

	t.Run("Should use default AWS SSM endpoint if CHAMBER_AWS_SSM_ENDPOINT not set", func(t *testing.T) {
		s, err := NewSecretsManagerStore(1)
		assert.Nil(t, err)
		endpoint, err := s.svc.(*secretsmanager.SecretsManager).Config.EndpointResolver.EndpointFor(endpoints.SecretsmanagerServiceID, endpoints.UsWest2RegionID)
		assert.Nil(t, err)
		assert.Equal(t, "https://secretsmanager.us-west-2.amazonaws.com", endpoint.URL)
	})
}

func TestSecretsManagerWrite(t *testing.T) {
	mock := &mockSecretsManagerClient{secrets: map[string]mockSecret{}, outputs: map[string]secretsmanager.DescribeSecretOutput{}}
	store := NewTestSecretsManagerStore(mock)

	t.Run("Setting a new key should work", func(t *testing.T) {
		key := "mykey"
		secretId := SecretId{Service: "test", Key: key}
		err := store.Write(secretId, "value", map[string]string{})
		assert.Nil(t, err)
		assert.Contains(t, mock.secrets, secretId.Service)
		assert.Equal(t, "value", (*mock.secrets[secretId.Service].currentSecret)[key])
		keyMetadata, err := getHydratedKeyMetadata(mock.secrets[secretId.Service].currentSecret, &key)
		assert.Nil(t, err)
		assert.Equal(t, 1, keyMetadata.Version)
		assert.Equal(t, 1, len(mock.secrets[secretId.Service].history))
	})

	t.Run("Setting a key twice should create a new version", func(t *testing.T) {
		key := "multipleversions"
		secretId := SecretId{Service: "test", Key: key}
		err := store.Write(secretId, "value", map[string]string{})
		assert.Nil(t, err)
		assert.Contains(t, mock.secrets, secretId.Service)
		assert.Equal(t, "value", (*mock.secrets[secretId.Service].currentSecret)[key])
		keyMetadata, err := getHydratedKeyMetadata(mock.secrets[secretId.Service].currentSecret, &key)
		assert.Nil(t, err)
		assert.Equal(t, 1, keyMetadata.Version)
		assert.Equal(t, 2, len(mock.secrets[secretId.Service].history))

		err = store.Write(secretId, "newvalue", map[string]string{})
		assert.Nil(t, err)
		assert.Contains(t, mock.secrets, secretId.Service)
		assert.Equal(t, "newvalue", (*mock.secrets[secretId.Service].currentSecret)[key])
		keyMetadata, err = getHydratedKeyMetadata(mock.secrets[secretId.Service].currentSecret, &key)
		assert.Nil(t, err)
		assert.Equal(t, 2, keyMetadata.Version)
		assert.Equal(t, 3, len(mock.secrets[secretId.Service].history))
	})

	t.Run("Setting a key on a secret with rotation enabled should fail", func(t *testing.T) {
		service := "rotationtest"
		mock.secrets[service] = mockSecret{}
		mock.outputs[service] = secretsmanager.DescribeSecretOutput{RotationEnabled: aws.Bool(true)}
		secretId := SecretId{Service: service, Key: "doesnotmatter"}
		err := store.Write(secretId, "value", map[string]string{})
		assert.EqualError(t, err, "Cannot write to a secret with rotation enabled")
	})
}

func TestSecretsManagerRead(t *testing.T) {
	mock := &mockSecretsManagerClient{secrets: map[string]mockSecret{}}
	store := NewTestSecretsManagerStore(mock)
	secretId := SecretId{Service: "test", Key: "key"}
	store.Write(secretId, "value", map[string]string{})
	store.Write(secretId, "second value", map[string]string{})
	store.Write(secretId, "third value", map[string]string{})

	t.Run("Reading the latest value should work", func(t *testing.T) {
		s, err := store.Read(secretId, -1)
		assert.Nil(t, err)
		assert.Equal(t, "third value", *s.Value)
	})

	t.Run("Reading specific versiosn should work", func(t *testing.T) {
		first, err := store.Read(secretId, 1)
		assert.Nil(t, err)
		assert.Equal(t, "value", *first.Value)

		second, err := store.Read(secretId, 2)
		assert.Nil(t, err)
		assert.Equal(t, "second value", *second.Value)

		third, err := store.Read(secretId, 3)
		assert.Nil(t, err)
		assert.Equal(t, "third value", *third.Value)
	})

	t.Run("Reading a non-existent key should give not found err", func(t *testing.T) {
		_, err := store.Read(SecretId{Service: "test", Key: "nope"}, -1)
		assert.Equal(t, ErrSecretNotFound, err)
	})

	t.Run("Reading a non-existent version should give not found error", func(t *testing.T) {
		_, err := store.Read(secretId, 30)
		assert.Equal(t, ErrSecretNotFound, err)
	})
}

func TestSecretsManagerList(t *testing.T) {
	mock := &mockSecretsManagerClient{secrets: map[string]mockSecret{}}
	store := NewTestSecretsManagerStore(mock)

	secrets := []SecretId{
		{Service: "test", Key: "a"},
		{Service: "test", Key: "b"},
		{Service: "test", Key: "c"},
	}
	for _, secret := range secrets {
		store.Write(secret, "value", map[string]string{})
	}

	t.Run("List should return all keys for a service", func(t *testing.T) {
		s, err := store.List("test", false)
		assert.Nil(t, err)
		assert.Equal(t, 3, len(s))
		sort.Sort(ByKey(s))
		assert.Equal(t, "a", s[0].Meta.Key)
		assert.Equal(t, "b", s[1].Meta.Key)
		assert.Equal(t, "c", s[2].Meta.Key)
	})

	t.Run("List should not return values if includeValues is false", func(t *testing.T) {
		s, err := store.List("test", false)
		assert.Nil(t, err)
		for _, secret := range s {
			assert.Nil(t, secret.Value)
		}
	})

	t.Run("List should return values if includeValues is true", func(t *testing.T) {
		s, err := store.List("test", true)
		assert.Nil(t, err)
		for _, secret := range s {
			assert.Equal(t, "value", *secret.Value)
		}
	})

	t.Run("List should only return exact matches on service name", func(t *testing.T) {
		store.Write(SecretId{Service: "match", Key: "a"}, "val", map[string]string{})
		store.Write(SecretId{Service: "matchlonger", Key: "a"}, "val", map[string]string{})

		s, err := store.List("match", false)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(s))
		assert.Equal(t, "a", s[0].Meta.Key)
	})
}

func TestSecretsManagerListRaw(t *testing.T) {
	mock := &mockSecretsManagerClient{secrets: map[string]mockSecret{}}
	store := NewTestSecretsManagerStore(mock)

	secrets := []SecretId{
		{Service: "test", Key: "a"},
		{Service: "test", Key: "b"},
		{Service: "test", Key: "c"},
	}
	for _, secret := range secrets {
		store.Write(secret, "value", map[string]string{})
	}

	t.Run("ListRaw should return all keys and values for a service", func(t *testing.T) {
		s, err := store.ListRaw("test")
		assert.Nil(t, err)
		sort.Sort(ByKeyRaw(s))
		s = s[1:]
		assert.Equal(t, 3, len(s))
		assert.Equal(t, "a", s[0].Key)
		assert.Equal(t, "b", s[1].Key)
		assert.Equal(t, "c", s[2].Key)

		assert.Equal(t, "value", s[0].Value)
		assert.Equal(t, "value", s[1].Value)
		assert.Equal(t, "value", s[2].Value)
	})

	t.Run("List should only return exact matches on service name", func(t *testing.T) {
		store.Write(SecretId{Service: "match", Key: "a"}, "val", map[string]string{})
		store.Write(SecretId{Service: "matchlonger", Key: "a"}, "val", map[string]string{})

		s, err := store.ListRaw("match")
		sort.Sort(ByKeyRaw(s))
		s = s[1:]
		assert.Nil(t, err)
		assert.Equal(t, 1, len(s))
		assert.Equal(t, "a", s[0].Key)
	})
}

func TestSecretsManagerHistory(t *testing.T) {
	mock := &mockSecretsManagerClient{secrets: map[string]mockSecret{}}
	store := NewTestSecretsManagerStore(mock)

	secrets := []SecretId{
		{Service: "test", Key: "new"},
		{Service: "test", Key: "update"},
		{Service: "test", Key: "update"},
		{Service: "test", Key: "update"},
	}

	for _, s := range secrets {
		store.Write(s, "value", map[string]string{})
	}

	t.Run("History for a non-existent key should return not found error", func(t *testing.T) {
		_, err := store.History(SecretId{Service: "test", Key: "nope"})
		assert.Equal(t, ErrSecretNotFound, err)
	})

	t.Run("History should return a single created event for new keys", func(t *testing.T) {
		events, err := store.History(SecretId{Service: "test", Key: "new"})
		assert.Nil(t, err)
		assert.Equal(t, 1, len(events))
		assert.Equal(t, Created, events[0].Type)
	})

	t.Run("History should return create followed by updates for keys that have been updated", func(t *testing.T) {
		events, err := store.History(SecretId{Service: "test", Key: "update"})
		assert.Nil(t, err)
		assert.Equal(t, 3, len(events))
		assert.Equal(t, Created, events[0].Type)
		assert.Equal(t, Updated, events[1].Type)
		assert.Equal(t, Updated, events[2].Type)
	})
}

func TestSecretsManagerDelete(t *testing.T) {
	mock := &mockSecretsManagerClient{secrets: map[string]mockSecret{}}
	store := NewTestSecretsManagerStore(mock)

	secretId := SecretId{Service: "test", Key: "key"}
	store.Write(secretId, "value", map[string]string{})

	t.Run("Deleting secret should work", func(t *testing.T) {
		err := store.Delete(secretId)
		assert.Nil(t, err)
		err = store.Delete(secretId)
		assert.Equal(t, ErrSecretNotFound, err)
	})

	t.Run("Deleting missing secret should fail", func(t *testing.T) {
		err := store.Delete(SecretId{Service: "test", Key: "nonkey"})
		assert.Equal(t, ErrSecretNotFound, err)
	})
}

func uniqueID() string {
	uuid := make([]byte, 16)
	io.ReadFull(rand.Reader, uuid)
	return fmt.Sprintf("%x", uuid)
}
