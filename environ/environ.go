package environ

import (
	"strings"

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
		envVarKey := strings.ToUpper(key(rawSecret.Key, noPaths))
		envVarKey = strings.Replace(envVarKey, "-", "_", -1)

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
