package cmd

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidations(t *testing.T) {

	// Test Key formats
	validKeyFormat := []string{
		"foo",
		"foo.bar",
		"foo.",
		".foo",
		"foo-bar",
	}

	for _, k := range validKeyFormat {
		t.Run("Key validation should return Nil", func(t *testing.T) {
			result := validateKey(k)
			assert.Nil(t, result)
		})
	}

	invalidKeyFormat := []string{
		"/foo",
		"foo//bar",
		"foo/bar",
	}

	for _, k := range invalidKeyFormat {
		t.Run("Key validation should return Error", func(t *testing.T) {
			result := validateKey(k)
			assert.Error(t, result)
		})
	}

	// Test Service format with PATH
	validServicePathFormat := []string{
		"foo",
		"foo.",
		".foo",
		"foo.bar",
		"foo-bar",
		"foo/bar",
		"foo.bar/foo",
		"foo-bar/foo",
		"foo-bar/foo-bar",
		"foo/bar/foo",
		"foo/bar/foo-bar",
	}

	for _, k := range validServicePathFormat {
		t.Run("Service with PATH validation should return Nil", func(t *testing.T) {
			result := validateService(k)
			assert.Nil(t, result)
		})
	}

	invalidServicePathFormat := []string{
		"foo/",
		"/foo",
		"foo//bar",
	}

	for _, k := range invalidServicePathFormat {
		t.Run("Service with PATH validation should return Error", func(t *testing.T) {
			result := validateService(k)
			assert.Error(t, result)
		})
	}

	// Test Service format without PATH
	os.Setenv("CHAMBER_NO_PATHS", "true")
	validServiceNoPathFormat := []string{
		"foo",
		"foo.",
		".foo",
		"foo.bar",
		"foo-bar",
		"foo-bar.foo",
		"foo-bar.foo-bar",
		"foo.bar.foo",
		"foo.bar.foo-bar",
	}

	for _, k := range validServiceNoPathFormat {
		t.Run("Service without PATH validation should return Nil", func(t *testing.T) {
			result := validateService(k)
			assert.Nil(t, result)
		})
	}

	invalidServiceNoPathFormat := []string{
		"/foo",
		"foo//bar",
		"foo/bar",
	}

	for _, k := range invalidServiceNoPathFormat {
		t.Run("Service without PATH validation should return Error", func(t *testing.T) {
			result := validateService(k)
			assert.Error(t, result)
		})
	}
	os.Unsetenv("CHAMBER_NO_PATHS")
}
