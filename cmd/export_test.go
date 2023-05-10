package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExportDotenv(t *testing.T) {
	tests := []struct {
		name   string
		params map[string]string
		output []string
	}{
		{
			name:   "simple string, simple test",
			params: map[string]string{"foo": "bar"},
			output: []string{`FOO="bar"`},
		},
		{
			name:   "literal dollar signs should be properly escaped",
			params: map[string]string{"foo": "bar", "baz": `$qux`},
			output: []string{`FOO="bar"`, `BAZ="\$qux"`},
		},
		{
			name:   "double quotes should be fully escaped",
			params: map[string]string{"foo": "bar", "baz": `"qux"`},
			output: []string{`FOO="bar"`, `BAZ="\"qux\""`},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			err := exportAsEnvFile(test.params, buf)

			assert.Nil(t, err)
			assert.ElementsMatch(t, test.output, strings.Split(strings.TrimSpace(buf.String()), "\n"))
		})
	}
}
