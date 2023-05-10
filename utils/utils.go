package utils

import (
	"strings"
)

// NormalizeService normalizes a provided service to a common format
func NormalizeService(service string) string {
	return strings.ToLower(service)
}

// NormalizeKey normalizes a provided secret key to a common format
func NormalizeKey(key string) string {
	return strings.ToLower(key)
}
