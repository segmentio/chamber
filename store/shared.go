package store

import (
	"context"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/kevinburke/isec2"
)

const (
	RegionEnvVar            = "CHAMBER_AWS_REGION"
	CustomSSMEndpointEnvVar = "CHAMBER_AWS_SSM_ENDPOINT"
)

func getSession(numRetries int) (*session.Session, *string, error) {
	var region *string

	endpointResolver := func(service, region string, optFns ...func(*endpoints.Options)) (endpoints.ResolvedEndpoint, error) {
		customSsmEndpoint, ok := os.LookupEnv(CustomSSMEndpointEnvVar)
		if ok {
			return endpoints.ResolvedEndpoint{
				URL: customSsmEndpoint,
			}, nil
		}

		return endpoints.DefaultResolver().EndpointFor(service, region, optFns...)
	}

	if regionOverride, ok := os.LookupEnv(RegionEnvVar); ok {
		region = aws.String(regionOverride)
	}
	retSession, err := session.NewSessionWithOptions(
		session.Options{
			Config: aws.Config{
				Region:           region,
				MaxRetries:       aws.Int(numRetries),
				EndpointResolver: endpoints.ResolverFunc(endpointResolver),
			},
			SharedConfigState: session.SharedConfigEnable,
		},
	)
	if err != nil {
		return nil, nil, err
	}

	// If region is still not set, attempt to determine it via ec2 metadata API
	if aws.StringValue(retSession.Config.Region) == "" {
		ec2ctx, ec2cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		runningEC2, ec2err := isec2.IsEC2(ec2ctx)
		ec2cancel()
		if ec2err == nil && runningEC2 {
			session := session.New()
			ec2metadataSvc := ec2metadata.New(session)
			if regionOverride, err := ec2metadataSvc.Region(); err == nil {
				region = aws.String(regionOverride)
			}
		}
	}

	return retSession, region, nil
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
