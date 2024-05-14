package store

import (
	"context"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
)

func TestGetConfig(t *testing.T) {
	originalEndpoint := os.Getenv(CustomSSMEndpointEnvVar)
	os.Setenv(CustomSSMEndpointEnvVar, "https://example.com/custom-endpoint")
	if originalEndpoint != "" {
		defer os.Setenv(CustomSSMEndpointEnvVar, originalEndpoint)
	} else {
		defer os.Unsetenv(CustomSSMEndpointEnvVar)
	}

	originalRegion := os.Getenv(RegionEnvVar)
	os.Setenv(RegionEnvVar, "us-west-2")
	if originalRegion != "" {
		defer os.Setenv(RegionEnvVar, originalRegion)
	} else {
		defer os.Unsetenv(RegionEnvVar)
	}

	config, region, err := getConfig(context.Background(), 3, aws.RetryModeStandard)

	assert.NoError(t, err)
	assert.Equal(t, "us-west-2", region)

	endpoint, err := config.EndpointResolverWithOptions.ResolveEndpoint("ssm", "us-west-2")
	assert.Equal(t, "https://example.com/custom-endpoint", endpoint.URL)
	assert.Equal(t, aws.EndpointSourceCustom, endpoint.Source)

	assert.Equal(t, 3, config.RetryMaxAttempts)
	assert.Equal(t, aws.RetryModeStandard, config.RetryMode)
}
