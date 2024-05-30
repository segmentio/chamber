package store

import (
	"context"
	"errors"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/stretchr/testify/assert"
)

type mockParameter struct {
	currentParam *types.Parameter
	history      []types.ParameterHistory
	meta         *types.ParameterMetadata
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

func NewTestSSMStore(parameters map[string]mockParameter, usePaths bool) *SSMStore {
	return &SSMStore{
		usePaths: usePaths,
		svc: &apiSSMMock{
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
			PutParameterFunc: func(ctx context.Context, params *ssm.PutParameterInput, optFns ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
				return mockPutParameter(params, parameters)
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

		s, err := NewSSMStore(1)
		assert.Nil(t, err)
		assert.Equal(t, "us-east-1", s.config.Region)
	})

	t.Run("Should use AWS_REGION if it is set", func(t *testing.T) {
		os.Setenv("AWS_REGION", "us-west-1")
		defer os.Unsetenv("AWS_REGION")

		s, err := NewSSMStore(1)
		assert.Nil(t, err)
		assert.Equal(t, "us-west-1", s.config.Region)
	})

	t.Run("Should use CHAMBER_AWS_SSM_ENDPOINT if set", func(t *testing.T) {
		os.Setenv("CHAMBER_AWS_SSM_ENDPOINT", "mycustomendpoint")
		defer os.Unsetenv("CHAMBER_AWS_SSM_ENDPOINT")

		s, err := NewSSMStore(1)
		assert.Nil(t, err)
		endpoint, err := s.config.EndpointResolverWithOptions.ResolveEndpoint(ssm.ServiceID, "us-west-2")
		assert.Nil(t, err)
		assert.Equal(t, "mycustomendpoint", endpoint.URL)
	})

	t.Run("Should use default AWS SSM endpoint if CHAMBER_AWS_SSM_ENDPOINT not set", func(t *testing.T) {
		s, err := NewSSMStore(1)
		assert.Nil(t, err)
		_, err = s.config.EndpointResolverWithOptions.ResolveEndpoint(ssm.ServiceID, "us-west-2")
		var notFoundError *aws.EndpointNotFoundError
		assert.ErrorAs(t, err, &notFoundError)
	})

	t.Run("Should set AWS SDK retry mode to default", func(t *testing.T) {
		s, err := NewSSMStore(1)
		assert.Nil(t, err)
		assert.Equal(t, DefaultRetryMode, s.config.RetryMode)
	})

}

func TestNewSSMStoreWithRetryMode(t *testing.T) {
	t.Run("Should configure AWS SDK max attempts and retry mode", func(t *testing.T) {
		s, err := NewSSMStoreWithRetryMode(2, aws.RetryModeAdaptive)
		assert.Nil(t, err)
		assert.Equal(t, 2, s.config.RetryMaxAttempts)
		assert.Equal(t, aws.RetryModeAdaptive, s.config.RetryMode)
	})
}

func TestWrite(t *testing.T) {
	parameters := map[string]mockParameter{}
	store := NewTestSSMStore(parameters, false)

	t.Run("Setting a new key should work", func(t *testing.T) {
		secretId := SecretId{Service: "test", Key: "mykey"}
		err := store.Write(secretId, "value")
		assert.Nil(t, err)
		assert.Contains(t, parameters, store.idToName(secretId))
		assert.Equal(t, "value", *parameters[store.idToName(secretId)].currentParam.Value)
		assert.Equal(t, "1", *parameters[store.idToName(secretId)].meta.Description)
		assert.Equal(t, 1, len(parameters[store.idToName(secretId)].history))
	})

	t.Run("Setting a key twice should create a new version", func(t *testing.T) {
		secretId := SecretId{Service: "test", Key: "multipleversions"}
		err := store.Write(secretId, "value")
		assert.Nil(t, err)
		err = store.Write(secretId, "newvalue")
		assert.Nil(t, err)

		assert.Contains(t, parameters, store.idToName(secretId))
		assert.Equal(t, "newvalue", *parameters[store.idToName(secretId)].currentParam.Value)
		assert.Equal(t, "2", *parameters[store.idToName(secretId)].meta.Description)
		assert.Equal(t, 2, len(parameters[store.idToName(secretId)].history))
	})
}

func TestRead(t *testing.T) {
	parameters := map[string]mockParameter{}
	store := NewTestSSMStore(parameters, false)
	secretId := SecretId{Service: "test", Key: "key"}
	store.Write(secretId, "value")
	store.Write(secretId, "second value")
	store.Write(secretId, "third value")

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

func TestList(t *testing.T) {
	parameters := map[string]mockParameter{}
	store := NewTestSSMStore(parameters, false)

	secrets := []SecretId{
		{Service: "test", Key: "a"},
		{Service: "test", Key: "b"},
		{Service: "test", Key: "c"},
	}
	for _, secret := range secrets {
		store.Write(secret, "value")
	}

	t.Run("List should return all keys for a service", func(t *testing.T) {
		s, err := store.List("test", false)
		assert.Nil(t, err)
		assert.Equal(t, 3, len(s))
		sort.Sort(ByKey(s))
		assert.Equal(t, "test.a", s[0].Meta.Key)
		assert.Equal(t, "test.b", s[1].Meta.Key)
		assert.Equal(t, "test.c", s[2].Meta.Key)
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
		store.Write(SecretId{Service: "match", Key: "a"}, "val")
		store.Write(SecretId{Service: "matchlonger", Key: "a"}, "val")

		s, err := store.List("match", false)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(s))
		assert.Equal(t, "match.a", s[0].Meta.Key)
	})
}

func TestListRaw(t *testing.T) {
	parameters := map[string]mockParameter{}
	store := NewTestSSMStore(parameters, false)

	secrets := []SecretId{
		{Service: "test", Key: "a"},
		{Service: "test", Key: "b"},
		{Service: "test", Key: "c"},
	}
	for _, secret := range secrets {
		store.Write(secret, "value")
	}

	t.Run("ListRaw should return all keys and values for a service", func(t *testing.T) {
		s, err := store.ListRaw("test")
		assert.Nil(t, err)
		assert.Equal(t, 3, len(s))
		sort.Sort(ByKeyRaw(s))
		assert.Equal(t, "test.a", s[0].Key)
		assert.Equal(t, "test.b", s[1].Key)
		assert.Equal(t, "test.c", s[2].Key)

		assert.Equal(t, "value", s[0].Value)
		assert.Equal(t, "value", s[1].Value)
		assert.Equal(t, "value", s[2].Value)
	})

	t.Run("List should only return exact matches on service name", func(t *testing.T) {
		store.Write(SecretId{Service: "match", Key: "a"}, "val")
		store.Write(SecretId{Service: "matchlonger", Key: "a"}, "val")

		s, err := store.ListRaw("match")
		assert.Nil(t, err)
		assert.Equal(t, 1, len(s))
		assert.Equal(t, "match.a", s[0].Key)
	})
}

func TestListRawWithPaths(t *testing.T) {
	parameters := map[string]mockParameter{}
	store := NewTestSSMStore(parameters, true)

	secrets := []SecretId{
		{Service: "test", Key: "a"},
		{Service: "test", Key: "b"},
		{Service: "test", Key: "c"},
	}
	for _, secret := range secrets {
		store.Write(secret, "value")
	}

	t.Run("ListRaw should return all keys and values for a service", func(t *testing.T) {
		s, err := store.ListRaw("test")
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
		store.Write(SecretId{Service: "match", Key: "a"}, "val")
		store.Write(SecretId{Service: "matchlonger", Key: "a"}, "val")

		s, err := store.ListRaw("match")
		assert.Nil(t, err)
		assert.Equal(t, 1, len(s))
		assert.Equal(t, "/match/a", s[0].Key)
	})
}

func TestHistory(t *testing.T) {
	parameters := map[string]mockParameter{}
	store := NewTestSSMStore(parameters, false)

	secrets := []SecretId{
		{Service: "test", Key: "new"},
		{Service: "test", Key: "update"},
		{Service: "test", Key: "update"},
		{Service: "test", Key: "update"},
	}

	for _, s := range secrets {
		store.Write(s, "value")
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

	t.Run("Histor should return create followed by updates for keys that have been updated", func(t *testing.T) {
		events, err := store.History(SecretId{Service: "test", Key: "update"})
		assert.Nil(t, err)
		assert.Equal(t, 3, len(events))
		assert.Equal(t, Created, events[0].Type)
		assert.Equal(t, Updated, events[1].Type)
		assert.Equal(t, Updated, events[2].Type)
	})
}

func TestWritePaths(t *testing.T) {
	parameters := map[string]mockParameter{}
	store := NewTestSSMStore(parameters, true)

	t.Run("Setting a new key should work", func(t *testing.T) {
		secretId := SecretId{Service: "test", Key: "mykey"}
		err := store.Write(secretId, "value")
		assert.Nil(t, err)
		assert.Contains(t, parameters, store.idToName(secretId))
		assert.Equal(t, "value", *parameters[store.idToName(secretId)].currentParam.Value)
		assert.Equal(t, "1", *parameters[store.idToName(secretId)].meta.Description)
		assert.Equal(t, 1, len(parameters[store.idToName(secretId)].history))
	})

	t.Run("Setting a key twice should create a new version", func(t *testing.T) {
		secretId := SecretId{Service: "test", Key: "multipleversions"}
		err := store.Write(secretId, "value")
		assert.Nil(t, err)
		err = store.Write(secretId, "newvalue")
		assert.Nil(t, err)

		assert.Contains(t, parameters, store.idToName(secretId))
		assert.Equal(t, "newvalue", *parameters[store.idToName(secretId)].currentParam.Value)
		assert.Equal(t, "2", *parameters[store.idToName(secretId)].meta.Description)
		assert.Equal(t, 2, len(parameters[store.idToName(secretId)].history))
	})
}

func TestReadPaths(t *testing.T) {
	parameters := map[string]mockParameter{}
	store := NewTestSSMStore(parameters, true)
	secretId := SecretId{Service: "test", Key: "key"}
	store.Write(secretId, "value")
	store.Write(secretId, "second value")
	store.Write(secretId, "third value")

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

func TestListPaths(t *testing.T) {
	parameters := map[string]mockParameter{}
	store := NewTestSSMStore(parameters, true)

	secrets := []SecretId{
		{Service: "test", Key: "a"},
		{Service: "test", Key: "b"},
		{Service: "test", Key: "c"},
	}
	for _, secret := range secrets {
		store.Write(secret, "value")
	}

	t.Run("List should return all keys for a service", func(t *testing.T) {
		s, err := store.List("test", false)
		assert.Nil(t, err)
		assert.Equal(t, 3, len(s))
		sort.Sort(ByKey(s))
		assert.Equal(t, "/test/a", s[0].Meta.Key)
		assert.Equal(t, "/test/b", s[1].Meta.Key)
		assert.Equal(t, "/test/c", s[2].Meta.Key)
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
		store.Write(SecretId{Service: "match", Key: "a"}, "val")
		store.Write(SecretId{Service: "matchlonger", Key: "a"}, "val")

		s, err := store.List("match", false)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(s))
		assert.Equal(t, "/match/a", s[0].Meta.Key)
	})
}

func TestHistoryPaths(t *testing.T) {
	parameters := map[string]mockParameter{}
	store := NewTestSSMStore(parameters, true)

	secrets := []SecretId{
		{Service: "test", Key: "new"},
		{Service: "test", Key: "update"},
		{Service: "test", Key: "update"},
		{Service: "test", Key: "update"},
	}

	for _, s := range secrets {
		store.Write(s, "value")
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

	t.Run("Histor should return create followed by updates for keys that have been updated", func(t *testing.T) {
		events, err := store.History(SecretId{Service: "test", Key: "update"})
		assert.Nil(t, err)
		assert.Equal(t, 3, len(events))
		assert.Equal(t, Created, events[0].Type)
		assert.Equal(t, Updated, events[1].Type)
		assert.Equal(t, Updated, events[2].Type)
	})
}

func TestDelete(t *testing.T) {
	parameters := map[string]mockParameter{}
	store := NewTestSSMStore(parameters, false)

	secretId := SecretId{Service: "test", Key: "key"}
	store.Write(secretId, "value")

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

func TestValidations(t *testing.T) {
	parameters := map[string]mockParameter{}
	pathStore := NewTestSSMStore(parameters, true)

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

	parameters = map[string]mockParameter{}
	noPathStore := NewTestSSMStore(parameters, false)
	noPathStore.usePaths = false

	validNoPathFormat := []string{
		"foo",
		"foo.",
		".foo",
		"foo.bar",
		"foo-bar",
		"foo-bar.foo",
		"foo-bar.foo-bar",
		"foo.bar.foo",
		"foo.bar.foo-bar",
	}

	for _, k := range validNoPathFormat {
		t.Run("Validation should return true", func(t *testing.T) {
			result := noPathStore.validateName(k)
			assert.True(t, result)
		})
	}

	invalidNoPathFormat := []string{
		"/foo",
		"foo/bar",
		"foo//bar",
	}

	for _, k := range invalidNoPathFormat {
		t.Run("Validation should return false", func(t *testing.T) {
			result := noPathStore.validateName(k)
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
