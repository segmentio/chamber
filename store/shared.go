package store

import (
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
)

const (
	RegionEnvVar = "CHAMBER_AWS_REGION"
)

func getSession(numRetries int) (*session.Session, *string) {
	var region *string

	if regionOverride, ok := os.LookupEnv(RegionEnvVar); ok {
		region = aws.String(regionOverride)
	}
	retSession := session.Must(session.NewSessionWithOptions(
		session.Options{
			Config: aws.Config{
				Region:     region,
				MaxRetries: aws.Int(numRetries),
			},
			SharedConfigState: session.SharedConfigEnable,
		},
	))

	// If region is still not set, attempt to determine it via ec2 metadata API
	if aws.StringValue(retSession.Config.Region) == "" {
		session := session.New()
		ec2metadataSvc := ec2metadata.New(session)
		if regionOverride, err := ec2metadataSvc.Region(); err == nil {
			region = aws.String(regionOverride)
		}
	}

	return retSession, region
}
