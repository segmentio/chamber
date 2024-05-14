package store

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/stretchr/testify/assert"
)

type mockSecret struct {
	currentSecret *secretValueObject
	history       map[string]*secretValueObject
}

func mockPutSecretValue(i *secretsmanager.PutSecretValueInput, secrets map[string]mockSecret) (*secretsmanager.PutSecretValueOutput, error) {
	current, ok := secrets[*i.SecretId]
	if !ok {
		return &secretsmanager.PutSecretValueOutput{}, ErrSecretNotFound
	}

	secret, err := jsonToSecretValueObject(*i.SecretString)
	if err != nil {
		return &secretsmanager.PutSecretValueOutput{}, err
	}

	current.currentSecret = &secret
	current.history[uniqueID()] = &secret

	secrets[*i.SecretId] = current

	return &secretsmanager.PutSecretValueOutput{}, nil
}

func mockCreateSecret(i *secretsmanager.CreateSecretInput, secrets map[string]mockSecret) (*secretsmanager.CreateSecretOutput, error) {
	secret, err := jsonToSecretValueObject(*i.SecretString)
	if err != nil {
		return &secretsmanager.CreateSecretOutput{}, err
	}

	current := mockSecret{
		currentSecret: &secret,
		history:       make(map[string]*secretValueObject),
	}
	current.history[uniqueID()] = &secret

	secrets[*i.Name] = current

	return &secretsmanager.CreateSecretOutput{}, nil
}

func mockGetSecretValue(i *secretsmanager.GetSecretValueInput, secrets map[string]mockSecret) (*secretsmanager.GetSecretValueOutput, error) {
	var version *secretValueObject

	if i.VersionId != nil {
		historyItem, ok := secrets[*i.SecretId].history[*i.VersionId]
		if !ok {
			return &secretsmanager.GetSecretValueOutput{},
				&types.ResourceNotFoundException{
					Message: aws.String("ResourceNotFoundException"),
				}
		}
		version = historyItem
	} else {
		current, ok := secrets[*i.SecretId]
		if !ok {
			return &secretsmanager.GetSecretValueOutput{},
				&types.ResourceNotFoundException{
					Message: aws.String("ResourceNotFoundException"),
				}
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

func mockListSecretVersionIds(i *secretsmanager.ListSecretVersionIdsInput, secrets map[string]mockSecret) (*secretsmanager.ListSecretVersionIdsOutput, error) {
	service, ok := secrets[*i.SecretId]
	if !ok || len(service.history) == 0 {
		return &secretsmanager.ListSecretVersionIdsOutput{}, ErrSecretNotFound
	}

	versions := make([]types.SecretVersionsListEntry, 0)
	for v := range service.history {
		versions = append(versions, types.SecretVersionsListEntry{VersionId: aws.String(v)})
	}

	return &secretsmanager.ListSecretVersionIdsOutput{Versions: versions}, nil
}

func mockDescribeSecret(i *secretsmanager.DescribeSecretInput, outputs map[string]secretsmanager.DescribeSecretOutput) (*secretsmanager.DescribeSecretOutput, error) {
	output, ok := outputs[*i.SecretId]
	if !ok {
		return &secretsmanager.DescribeSecretOutput{RotationEnabled: aws.Bool(false)}, nil
	}
	return &output, nil
}

func mockGetCallerIdentity(_ *sts.GetCallerIdentityInput) (*sts.GetCallerIdentityOutput, error) {
	return &sts.GetCallerIdentityOutput{
		Arn: aws.String("currentuser"),
	}, nil
}

func NewTestSecretsManagerStore(secrets map[string]mockSecret, outputs map[string]secretsmanager.DescribeSecretOutput) *SecretsManagerStore {
	return &SecretsManagerStore{
		svc: &apiSecretsManagerMock{
			CreateSecretFunc: func(ctx context.Context, params *secretsmanager.CreateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error) {
				return mockCreateSecret(params, secrets)
			},
			DescribeSecretFunc: func(ctx context.Context, params *secretsmanager.DescribeSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error) {
				return mockDescribeSecret(params, outputs)
			},
			GetSecretValueFunc: func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
				return mockGetSecretValue(params, secrets)
			},
			ListSecretVersionIdsFunc: func(ctx context.Context, params *secretsmanager.ListSecretVersionIdsInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.ListSecretVersionIdsOutput, error) {
				return mockListSecretVersionIds(params, secrets)
			},
			PutSecretValueFunc: func(ctx context.Context, params *secretsmanager.PutSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.PutSecretValueOutput, error) {
				return mockPutSecretValue(params, secrets)
			},
		},
		stsSvc: &apiSTSMock{
			GetCallerIdentityFunc: func(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
				return mockGetCallerIdentity(params)
			},
		},
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
		defer os.Unsetenv("CHAMBER_AWS_REGION")
		os.Setenv("AWS_REGION", "us-west-1")
		defer os.Unsetenv("AWS_REGION")
		os.Setenv("AWS_DEFAULT_REGION", "us-west-2")
		defer os.Unsetenv("AWS_DEFAULT_REGION")

		s, err := NewSecretsManagerStore(context.Background(), 1)
		assert.Nil(t, err)
		assert.Equal(t, "us-east-1", s.config.Region)
	})

	t.Run("Should use AWS_REGION if it is set", func(t *testing.T) {
		os.Setenv("AWS_REGION", "us-west-1")
		defer os.Unsetenv("AWS_REGION")

		s, err := NewSecretsManagerStore(context.Background(), 1)
		assert.Nil(t, err)
		assert.Equal(t, "us-west-1", s.config.Region)
	})

	t.Run("Should use CHAMBER_AWS_SSM_ENDPOINT if set", func(t *testing.T) {
		os.Setenv("CHAMBER_AWS_SSM_ENDPOINT", "mycustomendpoint")
		defer os.Unsetenv("CHAMBER_AWS_SSM_ENDPOINT")

		s, err := NewSecretsManagerStore(context.Background(), 1)
		assert.Nil(t, err)
		endpoint, err := s.config.EndpointResolverWithOptions.ResolveEndpoint(secretsmanager.ServiceID, "us-west-2")
		assert.Nil(t, err)
		assert.Equal(t, "mycustomendpoint", endpoint.URL)
	})

	t.Run("Should use default AWS SSM endpoint if CHAMBER_AWS_SSM_ENDPOINT not set", func(t *testing.T) {
		s, err := NewSecretsManagerStore(context.Background(), 1)
		assert.Nil(t, err)
		_, err = s.config.EndpointResolverWithOptions.ResolveEndpoint(secretsmanager.ServiceID, "us-west-2")
		var notFoundError *aws.EndpointNotFoundError
		assert.ErrorAs(t, err, &notFoundError)
	})
}

func TestSecretsManagerWrite(t *testing.T) {
	ctx := context.Background()
	secrets := make(map[string]mockSecret)
	outputs := make(map[string]secretsmanager.DescribeSecretOutput)
	store := NewTestSecretsManagerStore(secrets, outputs)

	t.Run("Setting a new key should work", func(t *testing.T) {
		key := "mykey"
		secretId := SecretId{Service: "test", Key: key}
		err := store.Write(ctx, secretId, "value")
		assert.Nil(t, err)
		assert.Contains(t, secrets, secretId.Service)
		assert.Equal(t, "value", (*secrets[secretId.Service].currentSecret)[key])
		keyMetadata, err := getHydratedKeyMetadata(secrets[secretId.Service].currentSecret, &key)
		assert.Nil(t, err)
		assert.Equal(t, 1, keyMetadata.Version)
		assert.Equal(t, 1, len(secrets[secretId.Service].history))
	})

	t.Run("Setting a key twice should create a new version", func(t *testing.T) {
		key := "multipleversions"
		secretId := SecretId{Service: "test", Key: key}
		err := store.Write(ctx, secretId, "value")
		assert.Nil(t, err)
		assert.Contains(t, secrets, secretId.Service)
		assert.Equal(t, "value", (*secrets[secretId.Service].currentSecret)[key])
		keyMetadata, err := getHydratedKeyMetadata(secrets[secretId.Service].currentSecret, &key)
		assert.Nil(t, err)
		assert.Equal(t, 1, keyMetadata.Version)
		assert.Equal(t, 2, len(secrets[secretId.Service].history))

		err = store.Write(ctx, secretId, "newvalue")
		assert.Nil(t, err)
		assert.Contains(t, secrets, secretId.Service)
		assert.Equal(t, "newvalue", (*secrets[secretId.Service].currentSecret)[key])
		keyMetadata, err = getHydratedKeyMetadata(secrets[secretId.Service].currentSecret, &key)
		assert.Nil(t, err)
		assert.Equal(t, 2, keyMetadata.Version)
		assert.Equal(t, 3, len(secrets[secretId.Service].history))
	})

	t.Run("Setting a key on a secret with rotation enabled should fail", func(t *testing.T) {
		service := "rotationtest"
		secrets[service] = mockSecret{}
		outputs[service] = secretsmanager.DescribeSecretOutput{RotationEnabled: aws.Bool(true)}
		secretId := SecretId{Service: service, Key: "doesnotmatter"}
		err := store.Write(ctx, secretId, "value")
		assert.EqualError(t, err, "Cannot write to a secret with rotation enabled")
	})
}

func TestSecretsManagerRead(t *testing.T) {
	ctx := context.Background()
	secrets := make(map[string]mockSecret)
	outputs := make(map[string]secretsmanager.DescribeSecretOutput)
	store := NewTestSecretsManagerStore(secrets, outputs)
	secretId := SecretId{Service: "test", Key: "key"}
	store.Write(ctx, secretId, "value")
	store.Write(ctx, secretId, "second value")
	store.Write(ctx, secretId, "third value")

	t.Run("Reading the latest value should work", func(t *testing.T) {
		s, err := store.Read(ctx, secretId, -1)
		assert.Nil(t, err)
		assert.Equal(t, "third value", *s.Value)
	})

	t.Run("Reading specific versiosn should work", func(t *testing.T) {
		first, err := store.Read(ctx, secretId, 1)
		assert.Nil(t, err)
		assert.Equal(t, "value", *first.Value)

		second, err := store.Read(ctx, secretId, 2)
		assert.Nil(t, err)
		assert.Equal(t, "second value", *second.Value)

		third, err := store.Read(ctx, secretId, 3)
		assert.Nil(t, err)
		assert.Equal(t, "third value", *third.Value)
	})

	t.Run("Reading a non-existent key should give not found err", func(t *testing.T) {
		_, err := store.Read(ctx, SecretId{Service: "test", Key: "nope"}, -1)
		assert.Equal(t, ErrSecretNotFound, err)
	})

	t.Run("Reading a non-existent version should give not found error", func(t *testing.T) {
		_, err := store.Read(ctx, secretId, 30)
		assert.Equal(t, ErrSecretNotFound, err)
	})
}

func TestSecretsManagerList(t *testing.T) {
	ctx := context.Background()
	secrets := make(map[string]mockSecret)
	outputs := make(map[string]secretsmanager.DescribeSecretOutput)
	store := NewTestSecretsManagerStore(secrets, outputs)

	testSecrets := []SecretId{
		{Service: "test", Key: "a"},
		{Service: "test", Key: "b"},
		{Service: "test", Key: "c"},
	}
	for _, secret := range testSecrets {
		store.Write(ctx, secret, "value")
	}

	t.Run("List should return all keys for a service", func(t *testing.T) {
		s, err := store.List(ctx, "test", false)
		assert.Nil(t, err)
		assert.Equal(t, 3, len(s))
		sort.Sort(ByKey(s))
		assert.Equal(t, "a", s[0].Meta.Key)
		assert.Equal(t, "b", s[1].Meta.Key)
		assert.Equal(t, "c", s[2].Meta.Key)
	})

	t.Run("List should not return values if includeValues is false", func(t *testing.T) {
		s, err := store.List(ctx, "test", false)
		assert.Nil(t, err)
		for _, secret := range s {
			assert.Nil(t, secret.Value)
		}
	})

	t.Run("List should return values if includeValues is true", func(t *testing.T) {
		s, err := store.List(ctx, "test", true)
		assert.Nil(t, err)
		for _, secret := range s {
			assert.Equal(t, "value", *secret.Value)
		}
	})

	t.Run("List should only return exact matches on service name", func(t *testing.T) {
		store.Write(ctx, SecretId{Service: "match", Key: "a"}, "val")
		store.Write(ctx, SecretId{Service: "matchlonger", Key: "a"}, "val")

		s, err := store.List(ctx, "match", false)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(s))
		assert.Equal(t, "a", s[0].Meta.Key)
	})
}

func TestSecretsManagerListRaw(t *testing.T) {
	ctx := context.Background()
	secrets := make(map[string]mockSecret)
	outputs := make(map[string]secretsmanager.DescribeSecretOutput)
	store := NewTestSecretsManagerStore(secrets, outputs)

	testSecrets := []SecretId{
		{Service: "test", Key: "a"},
		{Service: "test", Key: "b"},
		{Service: "test", Key: "c"},
	}
	for _, secret := range testSecrets {
		store.Write(ctx, secret, "value")
	}

	t.Run("ListRaw should return all keys and values for a service", func(t *testing.T) {
		s, err := store.ListRaw(ctx, "test")
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
		store.Write(ctx, SecretId{Service: "match", Key: "a"}, "val")
		store.Write(ctx, SecretId{Service: "matchlonger", Key: "a"}, "val")

		s, err := store.ListRaw(ctx, "match")
		sort.Sort(ByKeyRaw(s))
		s = s[1:]
		assert.Nil(t, err)
		assert.Equal(t, 1, len(s))
		assert.Equal(t, "a", s[0].Key)
	})
}

func TestSecretsManagerHistory(t *testing.T) {
	ctx := context.Background()
	secrets := make(map[string]mockSecret)
	outputs := make(map[string]secretsmanager.DescribeSecretOutput)
	store := NewTestSecretsManagerStore(secrets, outputs)

	testSecrets := []SecretId{
		{Service: "test", Key: "new"},
		{Service: "test", Key: "update"},
		{Service: "test", Key: "update"},
		{Service: "test", Key: "update"},
	}

	for _, s := range testSecrets {
		store.Write(ctx, s, "value")
	}

	t.Run("History for a non-existent key should return not found error", func(t *testing.T) {
		_, err := store.History(ctx, SecretId{Service: "test", Key: "nope"})
		assert.Equal(t, ErrSecretNotFound, err)
	})

	t.Run("History should return a single created event for new keys", func(t *testing.T) {
		events, err := store.History(ctx, SecretId{Service: "test", Key: "new"})
		assert.Nil(t, err)
		assert.Equal(t, 1, len(events))
		assert.Equal(t, Created, events[0].Type)
	})

	t.Run("History should return create followed by updates for keys that have been updated", func(t *testing.T) {
		events, err := store.History(ctx, SecretId{Service: "test", Key: "update"})
		assert.Nil(t, err)
		assert.Equal(t, 3, len(events))
		assert.Equal(t, Created, events[0].Type)
		assert.Equal(t, Updated, events[1].Type)
		assert.Equal(t, Updated, events[2].Type)
	})
}

func TestSecretsManagerDelete(t *testing.T) {
	ctx := context.Background()
	secrets := make(map[string]mockSecret)
	outputs := make(map[string]secretsmanager.DescribeSecretOutput)
	store := NewTestSecretsManagerStore(secrets, outputs)

	secretId := SecretId{Service: "test", Key: "key"}
	store.Write(ctx, secretId, "value")

	t.Run("Deleting secret should work", func(t *testing.T) {
		err := store.Delete(ctx, secretId)
		assert.Nil(t, err)
		err = store.Delete(ctx, secretId)
		assert.Equal(t, ErrSecretNotFound, err)
	})

	t.Run("Deleting missing secret should fail", func(t *testing.T) {
		err := store.Delete(ctx, SecretId{Service: "test", Key: "nonkey"})
		assert.Equal(t, ErrSecretNotFound, err)
	})
}

func uniqueID() string {
	uuid := make([]byte, 16)
	io.ReadFull(rand.Reader, uuid)
	return fmt.Sprintf("%x", uuid)
}
