package store

import (
	"errors"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/stretchr/testify/assert"
)

type mockSSMClient struct {
	ssmiface.SSMAPI
	parameters map[string]mockParameter
}

type mockParameter struct {
	currentParam *ssm.Parameter
	history      []*ssm.ParameterHistory
	meta         *ssm.ParameterMetadata
}

func (m *mockSSMClient) PutParameter(i *ssm.PutParameterInput) (*ssm.PutParameterOutput, error) {
	current, ok := m.parameters[*i.Name]
	if !ok {
		current = mockParameter{
			history: []*ssm.ParameterHistory{},
		}
	}

	if current.currentParam != nil {
		history := &ssm.ParameterHistory{
			Description:      current.meta.Description,
			KeyId:            current.meta.KeyId,
			LastModifiedDate: current.meta.LastModifiedDate,
			LastModifiedUser: current.meta.LastModifiedUser,
			Name:             current.meta.Name,
			Type:             current.meta.Type,
			Value:            current.currentParam.Value,
		}
		current.history = append(current.history, history)
	}

	current.currentParam = &ssm.Parameter{
		Name:  i.Name,
		Type:  i.Type,
		Value: i.Value,
	}
	current.meta = &ssm.ParameterMetadata{
		Description:      i.Description,
		KeyId:            i.KeyId,
		LastModifiedDate: aws.Time(time.Now()),
		LastModifiedUser: aws.String("test"),
		Name:             i.Name,
		Type:             i.Type,
	}
	m.parameters[*i.Name] = current

	return &ssm.PutParameterOutput{}, nil
}

func (m *mockSSMClient) GetParameters(i *ssm.GetParametersInput) (*ssm.GetParametersOutput, error) {
	parameters := []*ssm.Parameter{}

	for _, param := range m.parameters {
		if paramNameInSlice(param.meta.Name, i.Names) {
			if *i.WithDecryption == false {
				parameters = append(parameters, &ssm.Parameter{
					Name:  param.meta.Name,
					Value: nil,
				})
			} else {
				parameters = append(parameters, param.currentParam)
			}
		}
	}

	if len(parameters) == 0 {
		return &ssm.GetParametersOutput{
			Parameters: parameters,
		}, ErrSecretNotFound
	}

	return &ssm.GetParametersOutput{
		Parameters: parameters,
	}, nil
}

func (m *mockSSMClient) GetParameterHistory(i *ssm.GetParameterHistoryInput) (*ssm.GetParameterHistoryOutput, error) {
	history := []*ssm.ParameterHistory{}

	param, ok := m.parameters[*i.Name]
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
		history = append(history, &ssm.ParameterHistory{
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

func (m *mockSSMClient) DescribeParameters(i *ssm.DescribeParametersInput) (*ssm.DescribeParametersOutput, error) {
	parameters := []*ssm.ParameterMetadata{}

	for _, param := range m.parameters {
		match, err := matchFilters(i.Filters, param)
		if err != nil {
			return &ssm.DescribeParametersOutput{}, err
		}
		matchStringFilters, err := matchStringFilters(i.ParameterFilters, param)
		if err != nil {
			return &ssm.DescribeParametersOutput{}, err
		}

		if match && matchStringFilters {
			parameters = append(parameters, param.meta)
		}
	}

	return &ssm.DescribeParametersOutput{
		Parameters: parameters,
		NextToken:  nil,
	}, nil
}

func (m *mockSSMClient) GetParametersByPath(i *ssm.GetParametersByPathInput) (*ssm.GetParametersByPathOutput, error) {
	parameters := []*ssm.Parameter{}

	for _, param := range m.parameters {
		// Match ParameterFilters
		doesMatchStringFilters, err := matchStringFilters(i.ParameterFilters, param)
		if err != nil {
			return &ssm.GetParametersByPathOutput{}, err
		}

		doesMatchPathFilter := *i.Path == "/" || strings.HasPrefix(*param.meta.Name, *i.Path)

		if doesMatchStringFilters && doesMatchPathFilter {
			parameters = append(parameters, param.currentParam)
		}
	}

	return &ssm.GetParametersByPathOutput{
		Parameters: parameters,
		NextToken:  nil,
	}, nil
}

func (m *mockSSMClient) GetParametersByPathPages(i *ssm.GetParametersByPathInput, fn func(*ssm.GetParametersByPathOutput, bool) bool) error {
	o, err := m.GetParametersByPath(i)
	if err != nil {
		return err
	}
	fn(o, true)
	return nil
}

func (m *mockSSMClient) DescribeParametersPages(i *ssm.DescribeParametersInput, fn func(*ssm.DescribeParametersOutput, bool) bool) error {
	o, err := m.DescribeParameters(i)
	if err != nil {
		return err
	}
	fn(o, true)
	return nil
}

func (m *mockSSMClient) DeleteParameter(i *ssm.DeleteParameterInput) (*ssm.DeleteParameterOutput, error) {
	_, ok := m.parameters[*i.Name]
	if !ok {
		return &ssm.DeleteParameterOutput{}, errors.New("secret not found")
	}

	delete(m.parameters, *i.Name)

	return &ssm.DeleteParameterOutput{}, nil
}

func paramNameInSlice(name *string, slice []*string) bool {
	for _, val := range slice {
		if *val == *name {
			return true
		}
	}
	return false
}

func prefixInSlice(val *string, prefixes []*string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(*val, *prefix) {
			return true
		}
	}
	return false
}

func pathInSlice(val *string, paths []*string) bool {
	tokens := strings.Split(*val, "/")
	if len(tokens) < 2 {
		return false
	}
	matchPath := "/" + tokens[1]
	for _, path := range paths {
		if matchPath == *path {
			return true
		}
	}
	return false
}

func matchFilters(filters []*ssm.ParametersFilter, param mockParameter) (bool, error) {
	for _, filter := range filters {
		var compareTo *string
		switch *filter.Key {
		case "Name":
			compareTo = param.meta.Name
		case "Type":
			compareTo = param.meta.Type
		case "KeyId":
			compareTo = param.meta.KeyId
		default:
			return false, errors.New("invalid filter key")
		}
		if !prefixInSlice(compareTo, filter.Values) {
			return false, nil
		}
	}
	return true, nil
}

func matchStringFilters(filters []*ssm.ParameterStringFilter, param mockParameter) (bool, error) {
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
					if strings.HasPrefix(*param.meta.Name, *value) {
						result = true
					}
				}

				return result, nil
			}
		}
	}
	return true, nil
}

func NewTestSSMStore(mock ssmiface.SSMAPI) *SSMStore {
	return &SSMStore{
		svc: mock,
	}
}

func TestNewSSMStore(t *testing.T) {
	t.Run("Using region override should take precedence over other settings", func(t *testing.T) {
		os.Setenv("CHAMBER_AWS_REGION", "us-east-1")
		os.Setenv("AWS_REGION", "us-west-1")
		os.Setenv("AWS_DEFAULT_REGION", "us-west-2")

		s, err := NewSSMStore(1)
		assert.Nil(t, err)
		assert.Equal(t, "us-east-1", aws.StringValue(s.svc.(*ssm.SSM).Config.Region))
		os.Unsetenv("CHAMBER_AWS_REGION")
		os.Unsetenv("AWS_REGION")
		os.Unsetenv("AWS_DEFAULT_REGION")
	})

	t.Run("Should use AWS_REGION if it is set", func(t *testing.T) {
		os.Setenv("AWS_REGION", "us-west-1")

		s, err := NewSSMStore(1)
		assert.Nil(t, err)
		assert.Equal(t, "us-west-1", aws.StringValue(s.svc.(*ssm.SSM).Config.Region))

		os.Unsetenv("AWS_REGION")
	})

}

func TestWrite(t *testing.T) {
	mock := &mockSSMClient{parameters: map[string]mockParameter{}}
	store := NewTestSSMStore(mock)

	t.Run("Setting a new key should work", func(t *testing.T) {
		secretId := SecretId{Service: "test", Key: "mykey"}
		err := store.Write(secretId, "value")
		assert.Nil(t, err)
		assert.Contains(t, mock.parameters, store.idToName(secretId))
		assert.Equal(t, "value", *mock.parameters[store.idToName(secretId)].currentParam.Value)
		assert.Equal(t, "1", *mock.parameters[store.idToName(secretId)].meta.Description)
	})

	t.Run("Setting a key twice should create a new version", func(t *testing.T) {
		secretId := SecretId{Service: "test", Key: "multipleversions"}
		err := store.Write(secretId, "value")
		assert.Nil(t, err)
		err = store.Write(secretId, "newvalue")
		assert.Nil(t, err)

		assert.Contains(t, mock.parameters, store.idToName(secretId))
		assert.Equal(t, "newvalue", *mock.parameters[store.idToName(secretId)].currentParam.Value)
		assert.Equal(t, "2", *mock.parameters[store.idToName(secretId)].meta.Description)
		assert.Equal(t, 1, len(mock.parameters[store.idToName(secretId)].history))
	})
}

func TestRead(t *testing.T) {
	mock := &mockSSMClient{parameters: map[string]mockParameter{}}
	store := NewTestSSMStore(mock)
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
	mock := &mockSSMClient{parameters: map[string]mockParameter{}}
	store := NewTestSSMStore(mock)

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
	mock := &mockSSMClient{parameters: map[string]mockParameter{}}
	store := NewTestSSMStore(mock)

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
	mock := &mockSSMClient{parameters: map[string]mockParameter{}}
	store := NewTestSSMStoreWithPaths(mock)

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
	mock := &mockSSMClient{parameters: map[string]mockParameter{}}
	store := NewTestSSMStore(mock)

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

func NewTestSSMStoreWithPaths(mock ssmiface.SSMAPI) *SSMStore {
	return &SSMStore{
		svc:      mock,
		usePaths: true,
	}
}

func TestWritePaths(t *testing.T) {
	mock := &mockSSMClient{parameters: map[string]mockParameter{}}
	store := NewTestSSMStoreWithPaths(mock)

	t.Run("Setting a new key should work", func(t *testing.T) {
		secretId := SecretId{Service: "test", Key: "mykey"}
		err := store.Write(secretId, "value")
		assert.Nil(t, err)
		assert.Contains(t, mock.parameters, store.idToName(secretId))
		assert.Equal(t, "value", *mock.parameters[store.idToName(secretId)].currentParam.Value)
		assert.Equal(t, "1", *mock.parameters[store.idToName(secretId)].meta.Description)
	})

	t.Run("Setting a key twice should create a new version", func(t *testing.T) {
		secretId := SecretId{Service: "test", Key: "multipleversions"}
		err := store.Write(secretId, "value")
		assert.Nil(t, err)
		err = store.Write(secretId, "newvalue")
		assert.Nil(t, err)

		assert.Contains(t, mock.parameters, store.idToName(secretId))
		assert.Equal(t, "newvalue", *mock.parameters[store.idToName(secretId)].currentParam.Value)
		assert.Equal(t, "2", *mock.parameters[store.idToName(secretId)].meta.Description)
		assert.Equal(t, 1, len(mock.parameters[store.idToName(secretId)].history))
	})
}

func TestReadPaths(t *testing.T) {
	mock := &mockSSMClient{parameters: map[string]mockParameter{}}
	store := NewTestSSMStoreWithPaths(mock)
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
	mock := &mockSSMClient{parameters: map[string]mockParameter{}}
	store := NewTestSSMStoreWithPaths(mock)

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
	mock := &mockSSMClient{parameters: map[string]mockParameter{}}
	store := NewTestSSMStoreWithPaths(mock)

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
	mock := &mockSSMClient{parameters: map[string]mockParameter{}}
	store := NewTestSSMStore(mock)

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
	mock := &mockSSMClient{parameters: map[string]mockParameter{}}
	pathStore := NewTestSSMStore(mock)
	pathStore.usePaths = true

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

	noPathStore := NewTestSSMStore(mock)
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
}

type ByKey []Secret

func (a ByKey) Len() int           { return len(a) }
func (a ByKey) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByKey) Less(i, j int) bool { return a[i].Meta.Key < a[j].Meta.Key }

type ByKeyRaw []RawSecret

func (a ByKeyRaw) Len() int           { return len(a) }
func (a ByKeyRaw) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByKeyRaw) Less(i, j int) bool { return a[i].Key < a[j].Key }
