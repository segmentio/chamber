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
	return normalizeEnvVarName(key(k, noPaths))
}

func normalizeEnvVarName(k string) string {
	return strings.Replace(strings.ToUpper(k), "-", "_", -1)
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

// LoadStrict loads all services from s in strict mode: env vars in e with value equal to valueExpected
// are the only ones substituted. If there are any env vars in s that are also in e, but don't have their value
// set to valueExpected, this is an error.
func (e *Environ) LoadStrict(s store.Store, valueExpected string, pristine bool, services ...string) error {
	return e.loadStrict(s, valueExpected, pristine, false, services...)
}

// LoadNoPathsStrict is identical to LoadStrict, but uses v1-style "."-separated paths
//
// Deprecated like all noPaths functionality
func (e *Environ) LoadStrictNoPaths(s store.Store, valueExpected string, pristine bool, services ...string) error {
	return e.loadStrict(s, valueExpected, pristine, true, services...)
}

func (e *Environ) loadStrict(s store.Store, valueExpected string, pristine bool, noPaths bool, services ...string) error {
	for _, service := range services {
		rawSecrets, err := s.ListRaw(strings.ToLower(service))
		if err != nil {
			return err
		}
		err = e.loadStrictOne(rawSecrets, valueExpected, pristine, noPaths)
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *Environ) loadStrictOne(rawSecrets []store.RawSecret, valueExpected string, pristine bool, noPaths bool) error {
	var merr error
	parentMap := e.Map()
	parentExpects := map[string]struct{}{}
	for k, v := range parentMap {
		if v == valueExpected {
			if k != normalizeEnvVarName(k) {
				merr = multierror.Append(merr, ErrExpectedKeyUnnormalized{Key: k, ValueExpected: valueExpected})
				continue
			}
			// TODO: what if this key isn't chamber-compatible but could collide? MY_cool_var vs my-cool-var
			parentExpects[k] = struct{}{}
		}
	}

	envVarKeysAdded := map[string]struct{}{}
	for _, rawSecret := range rawSecrets {
		envVarKey := secretKeyToEnvVarName(rawSecret.Key, noPaths)

		parentVal, parentOk := parentMap[envVarKey]
		// skip injecting secrets that are not present in the parent
		if !parentOk {
			continue
		}
		delete(parentExpects, envVarKey)
		if parentVal != valueExpected {
			merr = multierror.Append(merr,
				ErrStoreUnexpectedValue{Key: envVarKey, ValueExpected: valueExpected, ValueActual: parentVal})
			continue
		}
		envVarKeysAdded[envVarKey] = struct{}{}
		e.Set(envVarKey, rawSecret.Value)
	}
	for k, _ := range parentExpects {
		merr = multierror.Append(merr, ErrStoreMissingKey{Key: k, ValueExpected: valueExpected})
	}

	if pristine {
		// unset all envvars that were in the parent env but not in store
		for k, _ := range parentMap {
			if _, ok := envVarKeysAdded[k]; !ok {
				e.Unset(k)
			}
		}
	}

	return merr
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

type ErrExpectedKeyUnnormalized struct {
	Key           string
	ValueExpected string
}

func (e ErrExpectedKeyUnnormalized) Error() string {
	return fmt.Sprintf("parent env has key `%s` with expected value `%s`, but key is not normalized like `%s`, so would never get substituted",
		e.Key, e.ValueExpected, normalizeEnvVarName(e.Key))
}
