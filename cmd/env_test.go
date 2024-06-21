package cmd

import (
	"testing"
)

func Test_validateShellName(t *testing.T) {
	tests := []struct {
		name       string
		str        string
		shouldFail bool
	}{
		{name: "strings with spaces should fail", str: "invalid strings", shouldFail: true},
		{name: "strings with only underscores should pass", str: "valid_string", shouldFail: false},
		{name: "strings with dashes should fail", str: "validish-string", shouldFail: true},
		{name: "strings that start with numbers should fail", str: "1invalidstring", shouldFail: true},
		{name: "strings that start with underscores should pass", str: "_1validstring", shouldFail: false},
		{name: "strings that contain periods should fail", str: "invalid.string", shouldFail: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateShellName(tt.str); (err != nil) != tt.shouldFail {
				t.Errorf("validateShellName error: %v, expect wantErr %v", err, tt.shouldFail)
			}
		})
	}
}

func Test_sanitizeKey(t *testing.T) {
	tests := []struct {
		given    string
		expected string
	}{
		{given: "invalid strings", expected: "invalid_strings"},
		{given: "extremely  invalid  strings", expected: "extremely__invalid__strings"},
		{given: "\nunbelievably\tinvalid\tstrings\n", expected: "unbelievably_invalid_strings"},
		{given: "valid_string", expected: "valid_string"},
		{given: "validish-string", expected: "validish_string"},
		{given: "valid.string", expected: "valid_string"},
		// the following two strings should not be corrected, simply returned as-is.
		{given: "1invalidstring", expected: "1invalidstring"},
		{given: "_1validstring", expected: "_1validstring"},
	}

	for _, tt := range tests {
		t.Run("test sanitizing key names", func(t *testing.T) {
			if got := sanitizeKey(tt.given); got != tt.expected {
				t.Errorf("shellName error: want %q, got %q", tt.expected, got)
			}
		})
	}
}

func Test_doubleQuoteEscape(t *testing.T) {
	tests := []struct {
		given    string
		expected string
	}{
		{given: "ordinary string", expected: "ordinary string"},
		{given: `string\with\backslashes`, expected: `string\\with\\backslashes`},
		{given: "string\nwith\nnewlines", expected: `string\nwith\nnewlines`},
		{given: "string\rwith\rcarriage returns", expected: `string\rwith\rcarriage returns`},
		{given: `string"with"quotation marks`, expected: `string\"with\"quotation marks`},
		{given: `string!with!excl`, expected: `string!with!excl`}, // do not escape !
		{given: `string$with$dollar signs`, expected: `string\$with\$dollar signs`},
	}

	for _, tt := range tests {
		t.Run("test sanitizing key names", func(t *testing.T) {
			if got := doubleQuoteEscape(tt.given); got != tt.expected {
				t.Errorf("shellName error: want %q, got %q", tt.expected, got)
			}
		})
	}
}
