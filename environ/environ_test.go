package environ

import (
	"testing"

	"github.com/segmentio/chamber/store"
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
			err := tc.e.loadStrictOne(rawSecrets, strictVal, tc.pristine, false)
			if err != nil {
				assert.EqualValues(t, tc.expectedErr, err)
			} else {
				assert.EqualValues(t, tc.expectedEnvMap, tc.e.Map())
			}
		})
	}
}
