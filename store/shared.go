package store

import (
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
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
		session := session.New()
		ec2metadataSvc := ec2metadata.New(session)
		if regionOverride, err := ec2metadataSvc.Region(); err == nil {
			region = aws.String(regionOverride)
		}
	}

	return retSession, region, nil
}
