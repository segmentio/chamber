package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeService(t *testing.T) {
	testCases := []struct {
		service  string
		expected string
	}{
		{"service", "service"},
		{"service-with-hyphens", "service-with-hyphens"},
		{"service_with_underscores", "service_with_underscores"},
		{"UPPERCASE_SERVICE", "uppercase_service"},
		{"mIXedcase-SERvice", "mixedcase-service"},
		{".complex/service-CASE", ".complex/service-case"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.service, func(t *testing.T) {
			assert.Equal(t, testCase.expected, NormalizeService(testCase.service))
		})
	}
}

func TestNormalizeKey(t *testing.T) {
	testCases := []struct {
		key      string
		expected string
	}{
		{"key", "key"},
		{"key-with-hyphens", "key-with-hyphens"},
		{"key_with_underscores", "key_with_underscores"},
		{"UPPERCASE_KEY", "uppercase_key"},
		{"mIXedcase-Key", "mixedcase-key"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.key, func(t *testing.T) {
			assert.Equal(t, testCase.expected, NormalizeKey(testCase.key))
		})
	}
}
