package environ

import (
	"sort"
	"testing"

	"github.com/segmentio/chamber/v2/store"
	"github.com/stretchr/testify/assert"
)

func TestEnvironStrict(t *testing.T) {
	cases := []struct {
		name string
		e    Environ
		// default: "chamberme"
		strictVal      string
		pristine       bool
		secrets        map[string]string
		expectedEnvMap map[string]string
		expectedErr    error
	}{
		{
			name: "parent ⊃ secrets (!pristine)",
			e: fromMap(map[string]string{
				"HOME":        "/tmp",
				"DB_USERNAME": "chamberme",
				"DB_PASSWORD": "chamberme",
			}),
			secrets: map[string]string{
				"db_username": "root",
				"db_password": "hunter22",
			},
			expectedEnvMap: map[string]string{
				"HOME":        "/tmp",
				"DB_USERNAME": "root",
				"DB_PASSWORD": "hunter22",
			},
		},

		{
			name: "parent ⊃ secrets with unfilled (!pristine)",
			e: fromMap(map[string]string{
				"HOME":        "/tmp",
				"DB_USERNAME": "chamberme",
				"DB_PASSWORD": "chamberme",
				"EXTRA":       "chamberme",
			}),
			secrets: map[string]string{
				"db_username": "root",
				"db_password": "hunter22",
			},
			expectedErr: ErrStoreMissingKey{Key: "EXTRA", ValueExpected: "chamberme"},
		},

		{
			name: "parent ⊃ secrets (pristine)",
			e: fromMap(map[string]string{
				"HOME":        "/tmp",
				"DB_USERNAME": "chamberme",
				"DB_PASSWORD": "chamberme",
			}),
			pristine: true,
			secrets: map[string]string{
				"db_username": "root",
				"db_password": "hunter22",
			},
			expectedEnvMap: map[string]string{
				"DB_USERNAME": "root",
				"DB_PASSWORD": "hunter22",
			},
		},

		{
			name: "parent with unnormalized key name",
			e: fromMap(map[string]string{
				"HOME":        "/tmp",
				"DB_username": "chamberme",
				"DB_PASSWORD": "chamberme",
			}),
			pristine: true,
			secrets: map[string]string{
				"db_username": "root",
				"db_password": "hunter22",
			},
			expectedErr: ErrExpectedKeyUnnormalized{Key: "DB_username", ValueExpected: "chamberme"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rawSecrets := make([]store.RawSecret, 0, len(tc.secrets))
			for k, v := range tc.secrets {
				rawSecrets = append(rawSecrets, store.RawSecret{
					Key:   k,
					Value: v,
				})
			}
			strictVal := tc.strictVal
			if strictVal == "" {
				strictVal = "chamberme"
			}
			err := tc.e.loadStrictOne(rawSecrets, strictVal, tc.pristine)
			if err != nil {
				assert.EqualValues(t, tc.expectedErr, err)
			} else {
				assert.EqualValues(t, tc.expectedEnvMap, tc.e.Map())
			}
		})
	}
}

func TestMap(t *testing.T) {
	cases := []struct {
		name string
		in   Environ
		out  map[string]string
	}{
		{
			"basic",
			Environ([]string{
				"k=v",
			}),
			map[string]string{
				"k": "v",
			},
		},
		{
			"dropping malformed",
			Environ([]string{
				"k=v",
				// should work
				"k2=",
			}),
			map[string]string{
				"k":  "v",
				"k2": "",
			},
		},
		{
			"squash",
			Environ([]string{
				"k=v1",
				"k=v2",
			}),
			map[string]string{
				"k": "v2",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := tc.in.Map()
			assert.EqualValues(t, m, tc.out)
		})
	}
}

func TestFromMap(t *testing.T) {
	cases := []struct {
		name string
		in   map[string]string
		out  Environ
	}{
		{
			"basic",
			map[string]string{
				"k1": "v1",
				"k2": "v2",
			},
			Environ([]string{
				"k1=v1",
				"k2=v2",
			}),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := fromMap(tc.in)
			// maps order is non-deterministic
			sort.Strings(e)
			assert.EqualValues(t, e, tc.out)
		})
	}
}
