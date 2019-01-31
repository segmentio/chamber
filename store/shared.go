package store

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
)

func getSession(numRetries int, region string) (*session.Session, string, error) {
	retSession, err := session.NewSessionWithOptions(
		session.Options{
			Config: aws.Config{
				Region:     aws.String(region),
				MaxRetries: aws.Int(numRetries),
			},
			SharedConfigState: session.SharedConfigEnable,
		},
	)
	if err != nil {
		return nil, "", err
	}

	// If region is still not set, attempt to determine it via ec2 metadata API
	if aws.StringValue(retSession.Config.Region) == "" {
		session := session.New()
		ec2metadataSvc := ec2metadata.New(session)
		if regionOverride, err := ec2metadataSvc.Region(); err == nil {
			region = regionOverride
		}
	}

	return retSession, region, nil
}
