package cmd

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/cenkalti/backoff"
	chamberstore "github.com/segmentio/chamber/store"
)

type listSecretsOperation struct {
	service     string
	secretStore *chamberstore.SSMStore
	rawSecrets  *[]chamberstore.RawSecret
}

func (op *listSecretsOperation) listSecrets() error {
	rawSecrets, err := op.secretStore.ListRaw(op.service)

	if err == nil {
		fmt.Println("chamber: read some secrets: ", len(rawSecrets))
		*op.rawSecrets = rawSecrets
		return nil
	}

	// if it's not an AWS error - return a permanent error
	awsErr, isAwserr := err.(awserr.Error)
	if !isAwserr {
		fmt.Println("chamber: gladly_secrets_list: got permanent error: ", err)
		return backoff.Permanent(err)
	}

	// if it's an AWS error but not the one we're expecting -
	// return a permanent error
	if awsErr.Code() != "NoCredentialProviders" || !strings.Contains(awsErr.Message(), "no valid providers in chain") {
		fmt.Println("chamber: gladly_secrets_list: got permanent error from AWS: ", err)
		return backoff.Permanent(err)
	}

	fmt.Println("chamber: gladly_secrets_list: encountered retryable error: ", err)
	// we got the NoCredentialProviders error that may indicate the race condition issue
	// return the error as non-permanent for possible retry
	return err
}

func ListRaw(secretStore *chamberstore.SSMStore, service string) ([]chamberstore.RawSecret, error) {
	rawSecrets := []chamberstore.RawSecret{}

	operation := listSecretsOperation{
		secretStore: secretStore,
		service:     service,
		rawSecrets:  &rawSecrets,
	}

	if err := backoff.Retry(operation.listSecrets, backoff.NewExponentialBackOff()); err != nil {
		return nil, err
	}

	return *operation.rawSecrets, nil
}
