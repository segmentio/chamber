package environ

import (
	"fmt"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/segmentio/chamber/store"
)

// environ is a slice of strings representing the environment, in the form "key=value".
type Environ []string

// Unset an environment variable by key
func (e *Environ) Unset(key string) {
	for i := range *e {
		if strings.HasPrefix((*e)[i], key+"=") {
			(*e)[i] = (*e)[len(*e)-1]
			*e = (*e)[:len(*e)-1]
			break
		}
	}
}

// IsSet returns whether or not a key is currently set in the environ
func (e *Environ) IsSet(key string) bool {
	for i := range *e {
		if strings.HasPrefix((*e)[i], key+"=") {
			return true
		}
	}
	return false
}

// Set adds an environment variable, replacing any existing ones of the same key
func (e *Environ) Set(key, val string) {
	e.Unset(key)
	*e = append(*e, key+"="+val)
}

// Map squashes the list-like environ, taking the latter value when there are
// collisions, like a shell would. Invalid items (e.g., missing `=`) are dropped
func (e *Environ) Map() map[string]string {
	ret := map[string]string{}
	for _, kv := range []string(*e) {
		s := strings.SplitN(kv, "=", 2)
		if len(s) != 2 {
			// drop invalid kv pairs
			// I guess this could happen in theory
			continue
		}
		ret[s[0]] = s[1]
	}
	return ret
}

func fromMap(m map[string]string) Environ {
	e := make([]string, 0, len(m))

	for k, v := range m {
		e = append(e, k+"="+v)
	}
	return Environ(e)
}

// like cmd/list.key, but without the env var lookup
func key(s string, noPaths bool) string {
	sep := "/"
	if noPaths {
		sep = "."
	}
	tokens := strings.Split(s, sep)
	secretKey := tokens[len(tokens)-1]
	return secretKey
}

// transforms a secret key to an env var name, i.e. upppercase, substitute `-` -> `_`
func secretKeyToEnvVarName(k string, noPaths bool) string {
	return strings.Replace(
		strings.ToUpper(
			key(k, noPaths),
		), "-", "_", -1,
	)
}

// load loads environment variables into e from s given a service
// collisions will be populated with any keys that get overwritten
// noPaths enables the behavior as if CHAMBER_NO_PATHS had been set
func (e *Environ) load(s store.Store, service string, collisions *[]string, noPaths bool) error {
	rawSecrets, err := s.ListRaw(strings.ToLower(service))
	if err != nil {
		return err
	}
	envVarKeys := make([]string, 0)
	for _, rawSecret := range rawSecrets {
		envVarKey := secretKeyToEnvVarName(rawSecret.Key, noPaths)

		envVarKeys = append(envVarKeys, envVarKey)

		if e.IsSet(envVarKey) {
			*collisions = append(*collisions, envVarKey)
		}
		e.Set(envVarKey, rawSecret.Value)
	}
	return nil
}

// Load loads environment variables into e from s given a service
// collisions will be populated with any keys that get overwritten
func (e *Environ) Load(s store.Store, service string, collisions *[]string) error {
	return e.load(s, service, collisions, false)
}

// LoadNoPaths is identical to Load, but uses v1-style "."-separated paths
//
// Deprecated like all noPaths functionality
func (e *Environ) LoadNoPaths(s store.Store, service string, collisions *[]string) error {
	return e.load(s, service, collisions, true)
}

type EnvironStrict struct {
	Environ

	Parent        Environ
	ValueExpected string
	Pristine      bool
}

//
type ErrParentMissingKey string

func (e ErrParentMissingKey) Error() string {
	return fmt.Sprintf("parent env missing %s", string(e))
}

type ErrStoreUnexpectedValue struct {
	// store-style key
	Key           string
	ValueExpected string
	ValueActual   string
}

func (e ErrStoreUnexpectedValue) Error() string {
	return fmt.Sprintf("parent env has %s, but was expecting value `%s`, not `%s`", e.Key, e.ValueExpected, e.ValueActual)
}

type ErrStoreMissingKey struct {
	// env-style key
	Key           string
	ValueExpected string
}

func (e ErrStoreMissingKey) Error() string {
	return fmt.Sprintf("parent env was expecting %s=%s, but was not in store", e.Key, e.ValueExpected)
}

func (e *EnvironStrict) load(rawSecrets []store.RawSecret, noPaths bool) error {
	parentMap := e.Parent.Map()
	parentExpects := map[string]struct{}{}
	for k, v := range parentMap {
		if v == e.ValueExpected {
			// TODO: what if this key isn't chamber-compatible but could collide? MY_cool_var vs my-cool-var
			parentExpects[k] = struct{}{}
		}
	}

	var merr error
	for _, rawSecret := range rawSecrets {
		envVarKey := secretKeyToEnvVarName(rawSecret.Key, noPaths)

		parentVal, parentOk := parentMap[envVarKey]
		if !parentOk {
			merr = multierror.Append(merr, ErrParentMissingKey(envVarKey))
			continue
		}
		delete(parentExpects, envVarKey)
		if parentVal != e.ValueExpected {
			merr = multierror.Append(merr,
				ErrStoreUnexpectedValue{Key: envVarKey, ValueExpected: e.ValueExpected, ValueActual: parentVal})
			continue
		}
		e.Set(envVarKey, rawSecret.Value)
	}
	for k, _ := range parentExpects {
		merr = multierror.Append(merr, ErrStoreMissingKey{Key: k, ValueExpected: e.ValueExpected})
	}

	if !e.Pristine {
		// set all values in parent that are not already set in e
		eMap := e.Map()
		for k, v := range parentMap {
			if _, ok := eMap[k]; !ok {
				e.Set(k, v)
			}
		}
	}

	return merr
}

// Load loads environment variables into e from rawSecrets
func (e *EnvironStrict) LoadFromSecrets(rawSecrets []store.RawSecret, noPaths bool) error {
	return e.load(rawSecrets, noPaths)
}
