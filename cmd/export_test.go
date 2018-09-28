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
			"simple",
			map[string]string{"foo": "bar"},
			[]string{`FOO="bar"`},
		},
		{
			"escaped dollar",
			map[string]string{"foo": "bar", "baz": "$qux"},
			[]string{`FOO="bar"`, `BAZ="\$qux"`},
		},
		{
			"escaped quote",
			map[string]string{"foo": "bar", "baz": `"qux"`},
			[]string{`FOO="bar"`, `BAZ="\"qux\""`},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			err := exportAsEnvFile(test.params, buf)
			assert.Nil(t, err)
			assert.ElementsMatch(t, test.output, strings.Split(strings.Trim(buf.String(), "\n"), "\n"))
		})
	}
}
