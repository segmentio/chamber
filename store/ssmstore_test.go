package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockParameter struct {
	currentParam *types.Parameter
	history      []types.ParameterHistory
	meta         *types.ParameterMetadata
	tags         map[string]string
}

func mockPutParameter(i *ssm.PutParameterInput, parameters map[string]mockParameter) (*ssm.PutParameterOutput, error) {
	current, ok := parameters[*i.Name]
	if !ok {
		current = mockParameter{
			history: []types.ParameterHistory{},
		}
	}

	current.currentParam = &types.Parameter{
		Name:  i.Name,
		Type:  i.Type,
		Value: i.Value,
	}
	current.meta = &types.ParameterMetadata{
		Description:      i.Description,
		KeyId:            i.KeyId,
		LastModifiedDate: aws.Time(time.Now()),
		LastModifiedUser: aws.String("test"),
		Name:             i.Name,
		Type:             i.Type,
	}
	history := types.ParameterHistory{
		Description:      current.meta.Description,
		KeyId:            current.meta.KeyId,
		LastModifiedDate: current.meta.LastModifiedDate,
		LastModifiedUser: current.meta.LastModifiedUser,
		Name:             current.meta.Name,
		Type:             current.meta.Type,
		Value:            current.currentParam.Value,
	}
	current.history = append(current.history, history)

	parameters[*i.Name] = current

	return &ssm.PutParameterOutput{}, nil
}

func mockGetParameters(i *ssm.GetParametersInput, parameters map[string]mockParameter) (*ssm.GetParametersOutput, error) {
	returnParameters := []types.Parameter{}

	for _, param := range parameters {
		if paramNameInSlice(param.meta.Name, i.Names) {
			if *i.WithDecryption == false {
				returnParameters = append(returnParameters, types.Parameter{
					Name:  param.meta.Name,
					Value: nil,
				})
			} else {
				returnParameters = append(returnParameters, *param.currentParam)
			}
		}
	}

	if len(parameters) == 0 {
		return &ssm.GetParametersOutput{
			Parameters: returnParameters,
		}, ErrSecretNotFound
	}

	return &ssm.GetParametersOutput{
		Parameters: returnParameters,
	}, nil
}

func mockGetParameterHistory(i *ssm.GetParameterHistoryInput, parameters map[string]mockParameter) (*ssm.GetParameterHistoryOutput, error) {
	history := []types.ParameterHistory{}

	param, ok := parameters[*i.Name]
	if !ok {
		return &ssm.GetParameterHistoryOutput{
			NextToken:  nil,
			Parameters: history,
		}, errors.New("parameter not found")
	}

	if *i.WithDecryption == true {
		return &ssm.GetParameterHistoryOutput{
			NextToken:  nil,
			Parameters: param.history,
		}, nil
	}

	for _, hist := range param.history {
		history = append(history, types.ParameterHistory{
			Description:      hist.Description,
			KeyId:            hist.KeyId,
			LastModifiedDate: hist.LastModifiedDate,
			LastModifiedUser: hist.LastModifiedUser,
			Name:             hist.Name,
			Type:             hist.Type,
			Value:            nil,
		})
	}
	return &ssm.GetParameterHistoryOutput{
		NextToken:  nil,
		Parameters: history,
	}, nil
}

func mockDescribeParameters(i *ssm.DescribeParametersInput, parameters map[string]mockParameter) (*ssm.DescribeParametersOutput, error) {
	returnMetadata := []types.ParameterMetadata{}

	for _, param := range parameters {
		match, err := matchFilters(i.Filters, param)
		if err != nil {
			return &ssm.DescribeParametersOutput{}, err
		}
		matchStringFilters, err := matchStringFilters(i.ParameterFilters, param)
		if err != nil {
			return &ssm.DescribeParametersOutput{}, err
		}

		if match && matchStringFilters {
			returnMetadata = append(returnMetadata, *param.meta)
		}
	}

	return &ssm.DescribeParametersOutput{
		Parameters: returnMetadata,
		NextToken:  nil,
	}, nil
}

func mockGetParametersByPath(i *ssm.GetParametersByPathInput, parameters map[string]mockParameter) (*ssm.GetParametersByPathOutput, error) {
	returnParameters := []types.Parameter{}

	for _, param := range parameters {
		// Match ParameterFilters
		doesMatchStringFilters, err := matchStringFilters(i.ParameterFilters, param)
		if err != nil {
			return &ssm.GetParametersByPathOutput{}, err
		}

		doesMatchPathFilter := *i.Path == "/" || strings.HasPrefix(*param.meta.Name, *i.Path)

		if doesMatchStringFilters && doesMatchPathFilter {
			returnParameters = append(returnParameters, *param.currentParam)
		}
	}

	return &ssm.GetParametersByPathOutput{
		Parameters: returnParameters,
		NextToken:  nil,
	}, nil
}

func mockDeleteParameter(i *ssm.DeleteParameterInput, parameters map[string]mockParameter) (*ssm.DeleteParameterOutput, error) {
	_, ok := parameters[*i.Name]
	if !ok {
		return &ssm.DeleteParameterOutput{}, errors.New("secret not found")
	}

	delete(parameters, *i.Name)

	return &ssm.DeleteParameterOutput{}, nil
}

func mockAddTagsToResource(i *ssm.AddTagsToResourceInput, parameters map[string]mockParameter) (*ssm.AddTagsToResourceOutput, error) {
	param, ok := parameters[*i.ResourceId]
	if !ok {
		return &ssm.AddTagsToResourceOutput{}, errors.New("secret not found")
	}

	if param.tags == nil {
		param.tags = make(map[string]string)
	}
	for _, tag := range i.Tags {
		param.tags[*tag.Key] = *tag.Value
	}
	parameters[*i.ResourceId] = param

	return &ssm.AddTagsToResourceOutput{}, nil
}

func mockListTagsForResource(i *ssm.ListTagsForResourceInput, parameters map[string]mockParameter) (*ssm.ListTagsForResourceOutput, error) {
	param, ok := parameters[*i.ResourceId]
	if !ok {
		return &ssm.ListTagsForResourceOutput{}, errors.New("secret not found")
	}

	tags := []types.Tag{}
	for key, value := range param.tags {
		tags = append(tags, types.Tag{
			Key:   aws.String(key),
			Value: aws.String(value),
		})
	}

	return &ssm.ListTagsForResourceOutput{
		TagList: tags,
	}, nil
}

func mockRemoveTagsFromResource(i *ssm.RemoveTagsFromResourceInput, parameters map[string]mockParameter) (*ssm.RemoveTagsFromResourceOutput, error) {
	param, ok := parameters[*i.ResourceId]
	if !ok {
		return &ssm.RemoveTagsFromResourceOutput{}, errors.New("secret not found")
	}

	for _, tag := range i.TagKeys {
		delete(param.tags, tag)
	}
	parameters[*i.ResourceId] = param

	return &ssm.RemoveTagsFromResourceOutput{}, nil
}

func paramNameInSlice(name *string, slice []string) bool {
	for _, val := range slice {
		if val == *name {
			return true
		}
	}
	return false
}

func anyPrefixInValue(val *string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(*val, prefix) {
			return true
		}
	}
	return false
}

func pathInSlice(val *string, paths []string) bool {
	tokens := strings.Split(*val, "/")
	if len(tokens) < 2 {
		return false
	}
	matchPath := "/" + tokens[1]
	for _, path := range paths {
		if matchPath == path {
			return true
		}
	}
	return false
}

func matchFilters(filters []types.ParametersFilter, param mockParameter) (bool, error) {
	for _, filter := range filters {
		var compareTo *string
		switch filter.Key {
		case "Name":
			compareTo = param.meta.Name
		case "Type":
			typeString := string(param.meta.Type)
			compareTo = &typeString
		case "KeyId":
			compareTo = param.meta.KeyId
		default:
			return false, errors.New("invalid filter key")
		}
		if !anyPrefixInValue(compareTo, filter.Values) {
			return false, nil
		}
	}
	return true, nil
}

func matchStringFilters(filters []types.ParameterStringFilter, param mockParameter) (bool, error) {
	for _, filter := range filters {
		var compareTo *string
		switch *filter.Key {
		case "Path":
			tokens := strings.Split(*param.meta.Name, "/")
			if len(tokens) < 2 {
				return false, errors.New("path filter used on non path value")
			}
			compareTo = aws.String("/" + tokens[1] + "/")

			if !pathInSlice(compareTo, filter.Values) {
				return false, nil
			}

		case "Name":
			if *filter.Option == "BeginsWith" {
				result := false
				for _, value := range filter.Values {
					if strings.HasPrefix(*param.meta.Name, value) {
						result = true
					}
				}

				return result, nil
			}
		}
	}
	return true, nil
}

func NewTestSSMStore(parameters map[string]mockParameter) *SSMStore {
	return &SSMStore{
		svc: &apiSSMMock{
			AddTagsToResourceFunc: func(ctx context.Context, params *ssm.AddTagsToResourceInput, optFns ...func(*ssm.Options)) (*ssm.AddTagsToResourceOutput, error) {
				return mockAddTagsToResource(params, parameters)
			},
			DeleteParameterFunc: func(ctx context.Context, params *ssm.DeleteParameterInput, optFns ...func(*ssm.Options)) (*ssm.DeleteParameterOutput, error) {
				return mockDeleteParameter(params, parameters)
			},
			DescribeParametersFunc: func(ctx context.Context, params *ssm.DescribeParametersInput, optFns ...func(*ssm.Options)) (*ssm.DescribeParametersOutput, error) {
				return mockDescribeParameters(params, parameters)
			},
			GetParameterHistoryFunc: func(ctx context.Context, params *ssm.GetParameterHistoryInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterHistoryOutput, error) {
				return mockGetParameterHistory(params, parameters)
			},
			GetParametersFunc: func(ctx context.Context, params *ssm.GetParametersInput, optFns ...func(*ssm.Options)) (*ssm.GetParametersOutput, error) {
				return mockGetParameters(params, parameters)
			},
			GetParametersByPathFunc: func(ctx context.Context, params *ssm.GetParametersByPathInput, optFns ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error) {
				return mockGetParametersByPath(params, parameters)
			},
			ListTagsForResourceFunc: func(ctx context.Context, params *ssm.ListTagsForResourceInput, optFns ...func(*ssm.Options)) (*ssm.ListTagsForResourceOutput, error) {
				return mockListTagsForResource(params, parameters)
			},
			PutParameterFunc: func(ctx context.Context, params *ssm.PutParameterInput, optFns ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
				return mockPutParameter(params, parameters)
			},
			RemoveTagsFromResourceFunc: func(ctx context.Context, params *ssm.RemoveTagsFromResourceInput, optFns ...func(*ssm.Options)) (*ssm.RemoveTagsFromResourceOutput, error) {
				return mockRemoveTagsFromResource(params, parameters)
			},
		},
	}
}

func TestNewSSMStore(t *testing.T) {
	t.Run("Using region override should take precedence over other settings", func(t *testing.T) {
		os.Setenv("CHAMBER_AWS_REGION", "us-east-1")
		defer os.Unsetenv("CHAMBER_AWS_REGION")
		os.Setenv("AWS_REGION", "us-west-1")
		defer os.Unsetenv("AWS_REGION")
		os.Setenv("AWS_DEFAULT_REGION", "us-west-2")
		defer os.Unsetenv("AWS_DEFAULT_REGION")

		s, err := NewSSMStore(context.Background(), 1)
		assert.Nil(t, err)
		assert.Equal(t, "us-east-1", s.config.Region)
	})

	t.Run("Should use AWS_REGION if it is set", func(t *testing.T) {
		os.Setenv("AWS_REGION", "us-west-1")
		defer os.Unsetenv("AWS_REGION")

		s, err := NewSSMStore(context.Background(), 1)
		assert.Nil(t, err)
		assert.Equal(t, "us-west-1", s.config.Region)
	})

	t.Run("Should use CHAMBER_AWS_SSM_ENDPOINT if set", func(t *testing.T) {
		os.Setenv("CHAMBER_AWS_SSM_ENDPOINT", "mycustomendpoint")
		defer os.Unsetenv("CHAMBER_AWS_SSM_ENDPOINT")

		s, err := NewSSMStore(context.Background(), 1)
		assert.Nil(t, err)
		ssmClient := s.svc.(*ssm.Client)
		assert.Equal(t, "mycustomendpoint", *ssmClient.Options().BaseEndpoint)
		// default endpoint resolution (v2) uses the client's BaseEndpoint
	})

	t.Run("Should use default AWS SSM endpoint if CHAMBER_AWS_SSM_ENDPOINT not set", func(t *testing.T) {
		s, err := NewSSMStore(context.Background(), 1)
		assert.Nil(t, err)
		ssmClient := s.svc.(*ssm.Client)
		assert.Nil(t, ssmClient.Options().BaseEndpoint)
	})

	t.Run("Should set AWS SDK retry mode to default", func(t *testing.T) {
		s, err := NewSSMStore(context.Background(), 1)
		assert.Nil(t, err)
		assert.Equal(t, DefaultRetryMode, s.config.RetryMode)
	})
}

func TestNewSSMStoreWithRetryMode(t *testing.T) {
	t.Run("Should configure AWS SDK max attempts and retry mode", func(t *testing.T) {
		s, err := NewSSMStoreWithRetryMode(context.Background(), 2, aws.RetryModeAdaptive)
		assert.Nil(t, err)
		assert.Equal(t, 2, s.config.RetryMaxAttempts)
		assert.Equal(t, aws.RetryModeAdaptive, s.config.RetryMode)
	})
}

func TestListRawWithPaths(t *testing.T) {
	ctx := context.Background()
	parameters := map[string]mockParameter{}
	store := NewTestSSMStore(parameters)

	secrets := []SecretId{
		{Service: "test", Key: "a"},
		{Service: "test", Key: "b"},
		{Service: "test", Key: "c"},
	}
	for _, secret := range secrets {
		require.NoError(t, store.Write(ctx, secret, "value"))
	}

	t.Run("ListRaw should return all keys and values for a service", func(t *testing.T) {
		s, err := store.ListRaw(ctx, "test")
		assert.Nil(t, err)
		assert.Equal(t, 3, len(s))
		sort.Sort(ByKeyRaw(s))
		assert.Equal(t, "/test/a", s[0].Key)
		assert.Equal(t, "/test/b", s[1].Key)
		assert.Equal(t, "/test/c", s[2].Key)

		assert.Equal(t, "value", s[0].Value)
		assert.Equal(t, "value", s[1].Value)
		assert.Equal(t, "value", s[2].Value)
	})

	t.Run("List should only return exact matches on service name", func(t *testing.T) {
		require.NoError(t, store.Write(ctx, SecretId{Service: "match", Key: "a"}, "val"))
		require.NoError(t, store.Write(ctx, SecretId{Service: "matchlonger", Key: "a"}, "val"))

		s, err := store.ListRaw(ctx, "match")
		assert.Nil(t, err)
		assert.Equal(t, 1, len(s))
		assert.Equal(t, "/match/a", s[0].Key)
	})
}

func TestWritePaths(t *testing.T) {
	ctx := context.Background()
	parameters := map[string]mockParameter{}
	store := NewTestSSMStore(parameters)

	t.Run("Setting a new key should work", func(t *testing.T) {
		secretId := SecretId{Service: "test", Key: "mykey"}
		err := store.Write(ctx, secretId, "value")
		assert.Nil(t, err)
		assert.Contains(t, parameters, store.idToName(secretId))
		assert.Equal(t, "value", *parameters[store.idToName(secretId)].currentParam.Value)
		assert.Equal(t, "1", *parameters[store.idToName(secretId)].meta.Description)
		assert.Equal(t, 1, len(parameters[store.idToName(secretId)].history))
	})

	t.Run("Setting a key twice should create a new version", func(t *testing.T) {
		secretId := SecretId{Service: "test", Key: "multipleversions"}
		err := store.Write(ctx, secretId, "value")
		assert.Nil(t, err)
		err = store.Write(ctx, secretId, "newvalue")
		assert.Nil(t, err)

		assert.Contains(t, parameters, store.idToName(secretId))
		assert.Equal(t, "newvalue", *parameters[store.idToName(secretId)].currentParam.Value)
		assert.Equal(t, "2", *parameters[store.idToName(secretId)].meta.Description)
		assert.Equal(t, 2, len(parameters[store.idToName(secretId)].history))
	})
}

func TestWriteWithTags(t *testing.T) {
	ctx := context.Background()
	parameters := map[string]mockParameter{}
	store := NewTestSSMStore(parameters)

	t.Run("Setting a new key with tags should work", func(t *testing.T) {
		secretId := SecretId{Service: "test", Key: "mykey"}
		tags := map[string]string{
			"tag1": "value1",
			"tag2": "value2",
		}
		err := store.WriteWithTags(ctx, secretId, "value", tags)
		assert.Nil(t, err)
		assert.Contains(t, parameters, store.idToName(secretId))

		paramTags := parameters[store.idToName(secretId)].tags
		assert.Equal(t, "value1", paramTags["tag1"])
		assert.Equal(t, "value2", paramTags["tag2"])
	})

	t.Run("Setting a existing key with tags should fail", func(t *testing.T) {
		secretId := SecretId{Service: "test", Key: "mykey"}
		tags := map[string]string{
			"tag3": "value3",
		}
		err := store.WriteWithTags(ctx, secretId, "newvalue", tags)
		assert.Error(t, err)

		assert.Contains(t, parameters, store.idToName(secretId))
		assert.Equal(t, "value", *parameters[store.idToName(secretId)].currentParam.Value) // unchanged
	})
}

func TestWriteWithRequiredTags(t *testing.T) {
	ctx := context.Background()
	storeConfigMockParameter := storeConfigMockParameter(`{"version":"1","requiredTags":["key1", "key2"]}`)
	parameters := map[string]mockParameter{
		*storeConfigMockParameter.meta.Name: storeConfigMockParameter,
	}
	store := NewTestSSMStore(parameters)

	t.Run("Setting a new key with required tags should work", func(t *testing.T) {
		secretId := SecretId{Service: "test", Key: "mykey1"}
		tags := map[string]string{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		}
		err := store.WriteWithTags(ctx, secretId, "value", tags)
		assert.NoError(t, err)
		assert.Contains(t, parameters, store.idToName(secretId))

		paramTags := parameters[store.idToName(secretId)].tags
		assert.Equal(t, "value1", paramTags["key1"])
		assert.Equal(t, "value2", paramTags["key2"])
		assert.Equal(t, "value3", paramTags["key3"])
	})
	t.Run("Setting a new key without required tags should fail", func(t *testing.T) {
		secretId := SecretId{Service: "test", Key: "mykey2"}
		tags := map[string]string{
			"key2": "value2",
			"key3": "value3",
		}
		err := store.WriteWithTags(ctx, secretId, "value", tags)
		assert.Error(t, err)
	})
}

func TestReadPaths(t *testing.T) {
	ctx := context.Background()
	parameters := map[string]mockParameter{}
	store := NewTestSSMStore(parameters)
	secretId := SecretId{Service: "test", Key: "key"}
	require.NoError(t, store.Write(ctx, secretId, "value"))
	require.NoError(t, store.Write(ctx, secretId, "second value"))
	require.NoError(t, store.Write(ctx, secretId, "third value"))

	t.Run("Reading the latest value should work", func(t *testing.T) {
		s, err := store.Read(ctx, secretId, -1)
		assert.Nil(t, err)
		assert.Equal(t, "third value", *s.Value)
	})

	t.Run("Reading specific versions should work", func(t *testing.T) {
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

func TestListPaths(t *testing.T) {
	ctx := context.Background()
	parameters := map[string]mockParameter{}
	store := NewTestSSMStore(parameters)

	secrets := []SecretId{
		{Service: "test", Key: "a"},
		{Service: "test", Key: "b"},
		{Service: "test", Key: "c"},
	}
	for _, secret := range secrets {
		require.NoError(t, store.Write(ctx, secret, "value"))
	}

	t.Run("List should return all keys for a service", func(t *testing.T) {
		s, err := store.List(ctx, "test", false)
		assert.Nil(t, err)
		assert.Equal(t, 3, len(s))
		sort.Sort(ByKey(s))
		assert.Equal(t, "/test/a", s[0].Meta.Key)
		assert.Equal(t, "/test/b", s[1].Meta.Key)
		assert.Equal(t, "/test/c", s[2].Meta.Key)
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
		require.NoError(t, store.Write(ctx, SecretId{Service: "match", Key: "a"}, "val"))
		require.NoError(t, store.Write(ctx, SecretId{Service: "matchlonger", Key: "a"}, "val"))

		s, err := store.List(ctx, "match", false)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(s))
		assert.Equal(t, "/match/a", s[0].Meta.Key)
	})
}

func TestHistoryPaths(t *testing.T) {
	ctx := context.Background()
	parameters := map[string]mockParameter{}
	store := NewTestSSMStore(parameters)

	secrets := []SecretId{
		{Service: "test", Key: "new"},
		{Service: "test", Key: "update"},
		{Service: "test", Key: "update"},
		{Service: "test", Key: "update"},
	}

	for _, s := range secrets {
		require.NoError(t, store.Write(ctx, s, "value"))
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

func TestDeletePaths(t *testing.T) {
	ctx := context.Background()
	parameters := map[string]mockParameter{}
	store := NewTestSSMStore(parameters)

	secretId := SecretId{Service: "test", Key: "key"}
	require.NoError(t, store.Write(ctx, secretId, "value"))

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

func TestWriteTags(t *testing.T) {
	ctx := context.Background()
	parameters := map[string]mockParameter{}
	store := NewTestSSMStore(parameters)
	secretId := SecretId{Service: "test", Key: "key"}
	require.NoError(t, store.Write(ctx, secretId, "value"))

	t.Run("Writing tags should work", func(t *testing.T) {
		tags := map[string]string{
			"tag1": "value1",
			"tag2": "value2",
		}
		err := store.WriteTags(ctx, secretId, tags, false)

		assert.Nil(t, err)
		assert.Contains(t, parameters, store.idToName(secretId))
		paramTags := parameters[store.idToName(secretId)].tags
		assert.Equal(t, "value1", paramTags["tag1"])
		assert.Equal(t, "value2", paramTags["tag2"])
	})

	t.Run("Writing tags should overwrite existing tags", func(t *testing.T) {
		tags := map[string]string{
			"tag1": "newvalue1",
		}
		err := store.WriteTags(ctx, secretId, tags, false)

		assert.Nil(t, err)
		assert.Contains(t, parameters, store.idToName(secretId))
		paramTags := parameters[store.idToName(secretId)].tags
		assert.Equal(t, "newvalue1", paramTags["tag1"])
		assert.Equal(t, "value2", paramTags["tag2"])
	})

	t.Run("Writing tags should delete other tags when desired", func(t *testing.T) {
		tags := map[string]string{
			"tag1": "newervalue1",
		}
		err := store.WriteTags(ctx, secretId, tags, true)

		assert.Nil(t, err)
		assert.Contains(t, parameters, store.idToName(secretId))
		paramTags := parameters[store.idToName(secretId)].tags
		assert.Equal(t, "newervalue1", paramTags["tag1"])
		_, ok := paramTags["tag2"]
		assert.False(t, ok)
	})

	t.Run("Writing tags to a non-existent key should give not found err", func(t *testing.T) {
		tags := map[string]string{
			"tag3": "value3",
		}
		err := store.WriteTags(ctx, SecretId{Service: "test", Key: "nope"}, tags, false)
		assert.Equal(t, ErrSecretNotFound, err)
	})
}

func TestWriteTagsWithRequiredTags(t *testing.T) {
	ctx := context.Background()
	storeConfigMockParameter := storeConfigMockParameter(`{"version":"1","requiredTags":["key1", "key2"]}`)
	parameters := map[string]mockParameter{
		*storeConfigMockParameter.meta.Name: storeConfigMockParameter,
	}
	store := NewTestSSMStore(parameters)
	secretId := SecretId{Service: "test", Key: "key"}
	require.NoError(t, store.WriteWithTags(ctx, secretId, "value", map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}))

	t.Run("Writing tags can omit required tags when not deleting others", func(t *testing.T) {
		tags := map[string]string{
			"key3": "value3a",
		}
		err := store.WriteTags(ctx, secretId, tags, false)
		assert.NoError(t, err)
		assert.Contains(t, parameters, store.idToName(secretId))
		paramTags := parameters[store.idToName(secretId)].tags
		assert.Equal(t, "value1", paramTags["key1"])
		assert.Equal(t, "value2", paramTags["key2"])
		assert.Equal(t, "value3a", paramTags["key3"])
	})

	t.Run("Writing tags can include required tags when not deleting others", func(t *testing.T) {
		tags := map[string]string{
			"key1": "value1a",
		}
		err := store.WriteTags(ctx, secretId, tags, false)
		assert.NoError(t, err)
		assert.Contains(t, parameters, store.idToName(secretId))
		paramTags := parameters[store.idToName(secretId)].tags
		assert.Equal(t, "value1a", paramTags["key1"])
		assert.Equal(t, "value2", paramTags["key2"])
		assert.Equal(t, "value3a", paramTags["key3"])
	})

	t.Run("Writing tags must not omit present required tags when deleting others", func(t *testing.T) {
		tags := map[string]string{
			"key1": "value1b",
			// skipping required key2
			"key4": "value4",
		}
		err := store.WriteTags(ctx, secretId, tags, true)
		assert.Error(t, err)
		assert.Contains(t, parameters, store.idToName(secretId))
		paramTags := parameters[store.idToName(secretId)].tags
		assert.Equal(t, "value1a", paramTags["key1"])
		assert.Equal(t, "value2", paramTags["key2"])
		assert.Equal(t, "value3a", paramTags["key3"])
		_, ok := paramTags["key4"]
		assert.False(t, ok)
	})

	t.Run("Writing tags must include all present required tags when deleting others", func(t *testing.T) {
		tags := map[string]string{
			"key1": "value1b",
			"key2": "value2b",
			// key3 to be deleted
			"key4": "value4",
		}
		err := store.WriteTags(ctx, secretId, tags, true)
		assert.NoError(t, err)
		assert.Contains(t, parameters, store.idToName(secretId))
		paramTags := parameters[store.idToName(secretId)].tags
		assert.Equal(t, "value1b", paramTags["key1"])
		assert.Equal(t, "value2b", paramTags["key2"])
		assert.Equal(t, "value4", paramTags["key4"])
		_, ok := paramTags["key3"]
		assert.False(t, ok)
	})
}

func TestReadTags(t *testing.T) {
	ctx := context.Background()
	parameters := map[string]mockParameter{}
	store := NewTestSSMStore(parameters)
	secretId := SecretId{Service: "test", Key: "key"}
	require.NoError(t, store.Write(ctx, secretId, "value"))

	t.Run("Reading tags should work", func(t *testing.T) {
		tags := map[string]string{
			"tag1": "value1",
			"tag2": "value2",
		}
		require.NoError(t, store.WriteTags(ctx, secretId, tags, false))

		readTags, err := store.ReadTags(ctx, secretId)
		assert.Nil(t, err)
		assert.Equal(t, tags, readTags)
	})

	t.Run("Reading tags for a non-existent key should give not found err", func(t *testing.T) {
		_, err := store.ReadTags(ctx, SecretId{Service: "test", Key: "nope"})
		assert.Equal(t, ErrSecretNotFound, err)
	})
}

func TestDeleteTags(t *testing.T) {
	ctx := context.Background()
	parameters := map[string]mockParameter{}
	store := NewTestSSMStore(parameters)
	secretId := SecretId{Service: "test", Key: "key"}
	require.NoError(t, store.Write(ctx, secretId, "value"))

	t.Run("Deleting tags should work", func(t *testing.T) {
		tags := map[string]string{
			"tag1": "value1",
			"tag2": "value2",
		}
		require.NoError(t, store.WriteTags(ctx, secretId, tags, false))

		err := store.DeleteTags(ctx, secretId, []string{"tag1"})
		assert.Nil(t, err)
		readTags, err := store.ReadTags(ctx, secretId)
		assert.Nil(t, err)
		assert.Equal(t, map[string]string{"tag2": "value2"}, readTags)
	})

	t.Run("Deleting tags should work for non-existent tags", func(t *testing.T) {
		tags := map[string]string{
			"tag2": "value2",
		} // state from previous test

		err := store.DeleteTags(ctx, secretId, []string{"tag3"})
		assert.Nil(t, err)
		readTags, err := store.ReadTags(ctx, secretId)
		assert.Nil(t, err)
		assert.Equal(t, tags, readTags)
	})

	t.Run("Deleting tags for a non-existent key should give not found err", func(t *testing.T) {
		err := store.DeleteTags(ctx, SecretId{Service: "test", Key: "nope"}, []string{"tag1"})
		assert.Equal(t, ErrSecretNotFound, err)
	})
}

func TestDeleteTagsWithRequiredTags(t *testing.T) {
	ctx := context.Background()
	storeConfigMockParameter := storeConfigMockParameter(`{"version":"1","requiredTags":["key1", "key2"]}`)
	parameters := map[string]mockParameter{
		*storeConfigMockParameter.meta.Name: storeConfigMockParameter,
	}
	store := NewTestSSMStore(parameters)
	secretId := SecretId{Service: "test", Key: "key"}
	require.NoError(t, store.WriteWithTags(ctx, secretId, "value", map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}))

	t.Run("Deleting non-required tags should work", func(t *testing.T) {
		err := store.DeleteTags(ctx, secretId, []string{"key3"})
		assert.NoError(t, err)
		readTags, err := store.ReadTags(ctx, secretId)
		assert.NoError(t, err)
		assert.Equal(t, map[string]string{"key1": "value1", "key2": "value2"}, readTags)
	})

	t.Run("Deleting required tags should fail", func(t *testing.T) {
		err := store.DeleteTags(ctx, secretId, []string{"key2"})
		assert.Error(t, err)
		readTags, err := store.ReadTags(ctx, secretId)
		assert.NoError(t, err)
		assert.Equal(t, map[string]string{"key1": "value1", "key2": "value2"}, readTags)
	})
}

func TestValidations(t *testing.T) {
	parameters := map[string]mockParameter{}
	pathStore := NewTestSSMStore(parameters)

	validPathFormat := []string{
		"/foo",
		"/foo.",
		"/.foo",
		"/foo.bar",
		"/foo-bar",
		"/foo/bar",
		"/foo.bar/foo",
		"/foo-bar/foo",
		"/foo-bar/foo-bar",
		"/foo/bar/foo",
		"/foo/bar/foo-bar",
	}

	for _, k := range validPathFormat {
		t.Run("Path Validation should return true", func(t *testing.T) {
			result := pathStore.validateName(k)
			assert.True(t, result)
		})
	}

	invalidPathFormat := []string{
		"/foo//bar",
		"foo//bar",
		"foo/bar",
		"foo/b",
		"foo",
	}

	for _, k := range invalidPathFormat {
		t.Run("Path Validation should return false", func(t *testing.T) {
			result := pathStore.validateName(k)
			assert.False(t, result)
		})
	}

	// Valid Path with Label
	validPathWithLabelFormat := [][]string{
		{"/foo", "/foo", ""},
		{"/foo.", "/foo.", ""},
		{"/foo.", "/foo.", ""},
		{"/.foo:blue", "/.foo", "blue"},
		{"/foo.bar:v30", "/foo.bar", "v30"},
		{"/foo-bar:v90", "/foo-bar", "v90"},
		{"/foo/bar:current", "/foo/bar", "current"},
		{"/foo.bar/foo:yellow", "/foo.bar/foo", "yellow"},
		{"/foo-bar/foo:v30:current", "/foo-bar/foo", "v30:current"},
		{"/foo-bar/foo-bar:v10", "/foo-bar/foo-bar", "v10"},
		{"/foo/bar/foo:90", "/foo/bar/foo", "90"},
		{"/foo/bar/foo-bar:90-10", "/foo/bar/foo-bar", "90-10"},
	}

	for _, k := range validPathWithLabelFormat {
		t.Run("Path Validation with Label should return true", func(t *testing.T) {
			result, label := parseServiceLabel(k[0])
			assert.Equal(t, result, k[1])
			assert.Equal(t, label, k[2])
		})
	}
}

type ByKey []Secret

func (a ByKey) Len() int           { return len(a) }
func (a ByKey) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByKey) Less(i, j int) bool { return a[i].Meta.Key < a[j].Meta.Key }

type ByKeyRaw []RawSecret

func (a ByKeyRaw) Len() int           { return len(a) }
func (a ByKeyRaw) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByKeyRaw) Less(i, j int) bool { return a[i].Key < a[j].Key }

func TestSSMStoreConfig(t *testing.T) {
	storeConfigMockParameter := storeConfigMockParameter(`{"version":"2","requiredTags":["key1", "key2"]}`)
	parameters := map[string]mockParameter{
		*storeConfigMockParameter.meta.Name: storeConfigMockParameter,
	}
	store := NewTestSSMStore(parameters)

	config, err := store.Config(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, "2", config.Version)
	assert.Equal(t, []string{"key1", "key2"}, config.RequiredTags)
}

func TestSSMStoreConfig_Missing(t *testing.T) {
	parameters := map[string]mockParameter{}
	store := NewTestSSMStore(parameters)

	config, err := store.Config(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, LatestStoreConfigVersion, config.Version)
	assert.Empty(t, config.RequiredTags)
}

func TestSSMStoreSetConfig(t *testing.T) {
	parameters := map[string]mockParameter{}
	store := NewTestSSMStore(parameters)

	config := StoreConfig{
		Version:      "2.1",
		RequiredTags: []string{"key1.1", "key2.1"},
	}
	err := store.SetConfig(context.Background(), config)

	assert.NoError(t, err)

	config, err = store.Config(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, "2.1", config.Version)
	assert.Equal(t, []string{"key1.1", "key2.1"}, config.RequiredTags)
}

func storeConfigMockParameter(content string) mockParameter {
	storeConfigName := fmt.Sprintf("/%s/%s", ChamberService, storeConfigKey)
	return mockParameter{
		currentParam: &types.Parameter{
			Name:  aws.String(storeConfigName),
			Type:  types.ParameterTypeSecureString,
			Value: aws.String(content),
		},
		meta: &types.ParameterMetadata{
			Name:             aws.String(storeConfigName),
			Description:      aws.String("1"),
			LastModifiedDate: aws.Time(time.Now()),
			LastModifiedUser: aws.String("test"),
		},
	}
}
