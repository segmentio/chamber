package store

import (
	"context"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
)

const (
	RegionEnvVar = "CHAMBER_AWS_REGION"
)

func getConfig(ctx context.Context, numRetries int, retryMode aws.RetryMode) (aws.Config, string, error) {
	var region string
	if regionOverride, ok := os.LookupEnv(RegionEnvVar); ok {
		region = regionOverride
	}

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithRetryMaxAttempts(numRetries),
		config.WithRetryMode(retryMode),
	)
	if err != nil {
		return aws.Config{}, "", err
	}

	// If region is still not set, attempt to determine it via ec2 metadata API
	if cfg.Region == "" {
		imdsConfig, err := config.LoadDefaultConfig(ctx)
		if err != nil {
			ec2metadataSvc := imds.NewFromConfig(imdsConfig)
			if regionOverride, err := ec2metadataSvc.GetRegion(ctx, &imds.GetRegionInput{}); err == nil {
				region = regionOverride.Region
			}
		}
	}

	return cfg, region, err
}

func uniqueStringSlice(slice []string) []string {
	unique := make(map[string]struct{}, len(slice))
	j := 0
	for _, value := range slice {
		if _, ok := unique[value]; ok {
			continue
		}
		unique[value] = struct{}{}
		slice[j] = value
		j++
	}
	return slice[:j]
}
