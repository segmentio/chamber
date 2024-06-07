package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateKey(t *testing.T) {
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
}

func TestValidateKey_Invalid(t *testing.T) {
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
}

func TestValidateService_Path(t *testing.T) {
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
}

func TestValidateService_Path_Invalid(t *testing.T) {
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
}

func TestValidateService_PathLabel(t *testing.T) {
	validServicePathFormatWithLabel := []string{
		"foo",
		"foo/bar:-current-",
		"foo.bar/foo:current",
		"foo-bar/foo:current",
		"foo-bar/foo-bar:current",
		"foo/bar/foo:current",
		"foo/bar/foo-bar:current",
		"foo/bar/foo-bar",
	}

	for _, k := range validServicePathFormatWithLabel {
		t.Run("Service with PATH validation and label should return Nil", func(t *testing.T) {
			result := validateServiceWithLabel(k)
			assert.Nil(t, result)
		})
	}
}

func TestValidateService_PathLabel_Invalid(t *testing.T) {
	invalidServicePathFormatWithLabel := []string{
		"foo:current$",
		"foo.:",
		":foo/bar:current",
		"foo.bar:cur|rent",
	}

	for _, k := range invalidServicePathFormatWithLabel {
		t.Run("Service with PATH validation and label should return Error", func(t *testing.T) {
			result := validateServiceWithLabel(k)
			assert.Error(t, result)
		})
	}
}
