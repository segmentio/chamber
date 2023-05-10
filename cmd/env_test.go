package cmd

import (
	"fmt"
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
		givenName    string
		expectedName string
	}{
		{givenName: "invalid strings", expectedName: "invalid_strings"},
		{givenName: "extremely  invalid  strings", expectedName: "extremely__invalid__strings"},
		{givenName: fmt.Sprintf("\nunbelievably\tinvalid\tstrings\n"), expectedName: "unbelievably_invalid_strings"},
		{givenName: "valid_string", expectedName: "valid_string"},
		{givenName: "validish-string", expectedName: "validish_string"},
		// these strings should not be corrected, simply returned as-is
		{givenName: "1invalidstring", expectedName: "1invalidstring"},
		{givenName: "_1validstring", expectedName: "_1validstring"},
		{givenName: "invalid.string", expectedName: "invalid_string"},
	}

	for _, tt := range tests {
		t.Run("test sanitizing key names", func(t *testing.T) {
			if got := sanitizeKey(tt.givenName); got != tt.expectedName {
				t.Errorf("shellName error: want %q, got %q", tt.expectedName, got)
			}
		})
	}
}
