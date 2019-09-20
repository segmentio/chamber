package environ

import (
	"testing"

	"github.com/hashicorp/go-multierror"
	"github.com/segmentio/chamber/store"
	"github.com/stretchr/testify/assert"
)

func TestEnvironStrict(t *testing.T) {
	cases := []struct {
		name           string
		e              EnvironStrict
		secrets        map[string]string
		expectedEnvMap map[string]string
		expectedErr    error
	}{
		{
			name: "parent ⊃ secrets (!pristine)",
			e: EnvironStrict{
				ValueExpected: "chamberme",

				Parent: fromMap(map[string]string{
					"HOME":     "/tmp",
					"USERNAME": "chamberme",
					"PASSWORD": "chamberme",
				}),
			},
			secrets: map[string]string{
				"username": "root",
				"password": "hunter22",
			},
			expectedEnvMap: map[string]string{
				"HOME":     "/tmp",
				"USERNAME": "root",
				"PASSWORD": "hunter22",
			},
		},

		{
			name: "parent ⊃ secrets with unfilled (!pristine)",
			e: EnvironStrict{
				ValueExpected: "chamberme",

				Parent: fromMap(map[string]string{
					"HOME":     "/tmp",
					"USERNAME": "chamberme",
					"PASSWORD": "chamberme",
					"EXTRA":    "chamberme",
				}),
			},
			secrets: map[string]string{
				"username": "root",
				"password": "hunter22",
			},
			expectedErr: &multierror.Error{Errors: []error{ErrStoreMissingKey{Key: "EXTRA", ValueExpected: "chamberme"}}},
		},

		{
			name: "parent ⊃ secrets (pristine)",
			e: EnvironStrict{
				ValueExpected: "chamberme",
				Pristine:      true,

				Parent: fromMap(map[string]string{
					"HOME":     "/tmp",
					"USERNAME": "chamberme",
					"PASSWORD": "chamberme",
				}),
			},
			secrets: map[string]string{
				"username": "root",
				"password": "hunter22",
			},
			expectedEnvMap: map[string]string{
				"USERNAME": "root",
				"PASSWORD": "hunter22",
			},
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
			err := tc.e.load(rawSecrets, false)
			if err != nil {
				assert.EqualValues(t, tc.expectedErr, err)
			} else {
				assert.EqualValues(t, tc.expectedEnvMap, tc.e.Map())
			}
		})
	}
}
