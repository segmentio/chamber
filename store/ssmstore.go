package store

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
)

const (
	// DefaultKeyID is the default alias for the KMS key used to encrypt/decrypt secrets
	DefaultKeyID = "alias/parameter_store_key"
)

// validPathKeyFormat is the format that is expected for key names inside parameter store
// when using paths
var validPathKeyFormat = regexp.MustCompile(`^\/[A-Za-z0-9-_]+\/[A-Za-z0-9-_]+$`)

// validKeyFormat is the format that is expected for key names inside parameter store when
// not using paths
var validKeyFormat = regexp.MustCompile(`^[A-Za-z0-9-_]+\.[A-Za-z0-9-_]+$`)

// validBaseFormat is the format that is expected base path.
var validBaseFormat = regexp.MustCompile(`^(/[A-Za-z0-9-_]+)+$`)

// ensure SSMStore confirms to Store interface
var _ Store = &SSMStore{}

// SSMStore implements the Store interface for storing secrets in SSM Parameter
// Store
type SSMStore struct {
	svc      ssmiface.SSMAPI
	base     string
	usePaths bool
}

// NewSSMStore creates a new SSMStore
func NewSSMStore(numRetries int) (*SSMStore, error) {
	region, ok := os.LookupEnv("AWS_REGION")
	if !ok {
		// If region is not set, attempt to determine it via ec2 metadata API
		session := session.New()
		ec2metadataSvc := ec2metadata.New(session)
		region, _ = ec2metadataSvc.Region()
	}

	if regionOverride, ok := os.LookupEnv("CHAMBER_AWS_REGION"); ok {
		region = regionOverride
	}

	ssmSession := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(region),
	}))
	svc := ssm.New(ssmSession, &aws.Config{MaxRetries: aws.Int(numRetries)})

	usePaths := false
	_, ok = os.LookupEnv("CHAMBER_USE_PATHS")
	if ok {
		usePaths = true
	}

	base, ok := os.LookupEnv("CHAMBER_BASE")
	if ok {
		if !validBaseFormat.MatchString(base) {
			return nil, errors.New(fmt.Sprintf("base path is invalid. It should match %s", validBaseFormat))
		}
	} else {
		base = ""
	}

	return &SSMStore{
		svc:      svc,
		base:     base,
		usePaths: usePaths,
	}, nil
}

func (s *SSMStore) KMSKey() string {
	fromEnv, ok := os.LookupEnv("CHAMBER_KMS_KEY_ALIAS")
	if !ok {
		return DefaultKeyID
	}
	if !strings.HasPrefix(fromEnv, "alias/") {
		return fmt.Sprintf("alias/%s", fromEnv)
	}

	return fromEnv
}

// Write writes a given value to a secret identified by id.  If the secret
// already exists, then write a new version.
func (s *SSMStore) Write(id SecretId, value string) error {
	version := 1
	// first read to get the current version
	current, err := s.Read(id, -1)
	if err != nil && err != ErrSecretNotFound {
		return err
	}
	if err == nil {
		version = current.Meta.Version + 1
	}

	putParameterInput := &ssm.PutParameterInput{
		KeyId:       aws.String(s.KMSKey()),
		Name:        aws.String(s.idToName(id)),
		Type:        aws.String("SecureString"),
		Value:       aws.String(value),
		Overwrite:   aws.Bool(true),
		Description: aws.String(strconv.Itoa(version)),
	}

	// This API call returns an empty struct
	_, err = s.svc.PutParameter(putParameterInput)
	if err != nil {
		return err
	}

	return nil
}

// Read reads a secret from the parameter store at a specific version.
// To grab the latest version, use -1 as the version number.
func (s *SSMStore) Read(id SecretId, version int) (Secret, error) {
	if version == -1 {
		return s.readLatest(id)
	}

	return s.readVersion(id, version)
}

// Delete removes a secret from the parameter store. Note this removes all
// versions of the secret.
func (s *SSMStore) Delete(id SecretId) error {
	// first read to ensure parameter present
	_, err := s.Read(id, -1)
	if err != nil {
		return err
	}

	deleteParameterInput := &ssm.DeleteParameterInput{
		Name: aws.String(s.idToName(id)),
	}

	_, err = s.svc.DeleteParameter(deleteParameterInput)
	if err != nil {
		return err
	}

	return nil
}

func (s *SSMStore) readVersion(id SecretId, version int) (Secret, error) {
	getParameterHistoryInput := &ssm.GetParameterHistoryInput{
		Name:           aws.String(s.idToName(id)),
		WithDecryption: aws.Bool(true),
	}

	resp, err := s.svc.GetParameterHistory(getParameterHistoryInput)
	if err != nil {
		return Secret{}, ErrSecretNotFound
	}

	for _, history := range resp.Parameters {
		thisVersion := 0
		if history.Description != nil {
			thisVersion, _ = strconv.Atoi(*history.Description)
		}
		if thisVersion == version {
			return Secret{
				Value: history.Value,
				Meta: SecretMetadata{
					Created:   *history.LastModifiedDate,
					CreatedBy: *history.LastModifiedUser,
					Version:   thisVersion,
					Key:       *history.Name,
				},
			}, nil
		}
	}

	// If we havent found it yet, check the latest version (which
	// doesnt get returned from GetParameterHistory)
	current, err := s.readLatest(id)
	if err != nil {
		return Secret{}, err
	}
	if current.Meta.Version == version {
		return current, nil
	}

	return Secret{}, ErrSecretNotFound
}

func (s *SSMStore) readLatest(id SecretId) (Secret, error) {
	getParametersInput := &ssm.GetParametersInput{
		Names:          []*string{aws.String(s.idToName(id))},
		WithDecryption: aws.Bool(true),
	}

	resp, err := s.svc.GetParameters(getParametersInput)
	if err != nil {
		return Secret{}, ErrSecretNotFound
	}

	if len(resp.Parameters) == 0 {
		return Secret{}, ErrSecretNotFound
	}
	param := resp.Parameters[0]

	// To get metadata, we need to use describe parameters
	describeParametersInput := &ssm.DescribeParametersInput{
		ParameterFilters: []*ssm.ParameterStringFilter{
			{
				Key:    aws.String("Name"),
				Option: aws.String("Equals"),
				Values: []*string{aws.String(s.idToName(id))},
			},
		},
		MaxResults: aws.Int64(1),
	}
	desc, err := s.svc.DescribeParameters(describeParametersInput)
	if err != nil {
		return Secret{}, err
	}
	if len(desc.Parameters) == 0 {
		return Secret{}, ErrSecretNotFound
	}

	secretMeta := parameterMetaToSecretMeta(desc.Parameters[0])

	return Secret{
		Value: param.Value,
		Meta:  secretMeta,
	}, nil
}

// List lists all secrets for a given service.  If includeValues is true,
// then those secrets are decrypted and returned, otherwise only the metadata
// about a secret is returned.
func (s *SSMStore) List(service string, includeValues bool) ([]Secret, error) {
	secrets := map[string]Secret{}

	var nextToken *string
	for {
		var describeParametersInput *ssm.DescribeParametersInput

		describeParametersInput = &ssm.DescribeParametersInput{
			ParameterFilters: []*ssm.ParameterStringFilter{
				{
					Key:    aws.String("Name"),
					Option: aws.String("BeginsWith"),
					Values: []*string{aws.String(s.serviceToSsmSuffixed(service))},
				},
			},
			MaxResults: aws.Int64(50),
			NextToken:  nextToken,
		}

		resp, err := s.svc.DescribeParameters(describeParametersInput)
		if err != nil {
			return nil, err
		}

		for _, meta := range resp.Parameters {
			if !s.validateName(*meta.Name) {
				continue
			}
			secretMeta := parameterMetaToSecretMeta(meta)
			secrets[secretMeta.Key] = Secret{
				Value: nil,
				Meta:  secretMeta,
			}
		}

		if resp.NextToken == nil {
			break
		}

		nextToken = resp.NextToken
	}

	if includeValues {
		secretKeys := keys(secrets)
		for i := 0; i < len(secretKeys); i += 10 {
			batchEnd := i + 10
			if i+10 > len(secretKeys) {
				batchEnd = len(secretKeys)
			}
			batch := secretKeys[i:batchEnd]

			getParametersInput := &ssm.GetParametersInput{
				Names:          stringsToAWSStrings(batch),
				WithDecryption: aws.Bool(true),
			}

			resp, err := s.svc.GetParameters(getParametersInput)
			if err != nil {
				return nil, err
			}

			for _, param := range resp.Parameters {
				secret := secrets[*param.Name]
				secret.Value = param.Value
				secrets[*param.Name] = secret
			}
		}
	}
	return values(secrets), nil
}

// History returns a list of events that have occured regarding the given
// secret.
func (s *SSMStore) History(id SecretId) ([]ChangeEvent, error) {
	events := []ChangeEvent{}

	getParameterHistoryInput := &ssm.GetParameterHistoryInput{
		Name:           aws.String(s.idToName(id)),
		WithDecryption: aws.Bool(false),
	}

	resp, err := s.svc.GetParameterHistory(getParameterHistoryInput)
	if err != nil {
		return events, ErrSecretNotFound
	}

	for _, history := range resp.Parameters {
		// Disregard error here, if Atoi fails (secret created outside of
		// Chamber), then we use version 0
		version := 0
		if history.Description != nil {
			version, _ = strconv.Atoi(*history.Description)
		}
		events = append(events, ChangeEvent{
			Type:    getChangeType(version),
			Time:    *history.LastModifiedDate,
			User:    *history.LastModifiedUser,
			Version: version,
		})
	}

	// The current version is not included in the GetParameterHistory response
	current, err := s.Read(id, -1)
	if err != nil {
		return events, err
	}

	events = append(events, ChangeEvent{
		Type:    getChangeType(current.Meta.Version),
		Time:    current.Meta.Created,
		User:    current.Meta.CreatedBy,
		Version: current.Meta.Version,
	})
	return events, nil
}

func (s *SSMStore) serviceToSsm(service string) string {
	if s.base == "" && !s.usePaths {
		return service
	}
	return fmt.Sprintf("%s/%s", s.base, service)
}

func (s *SSMStore) serviceToSsmSuffixed(service string) string {
	if s.usePaths {
		return s.serviceToSsm(service) + "/"
	}
	return s.serviceToSsm(service) + "."
}

func (s *SSMStore) idToName(id SecretId) string {
	return s.serviceToSsmSuffixed(id.Service) + id.Key
}

func (s *SSMStore) validateName(name string) bool {
	nameToMatch := name
	if s.base != "" {
		stripPath := s.base
		if !s.usePaths {
			stripPath = s.base + "/"
		}
		if !strings.HasPrefix(name, stripPath) {
			return false
		}
		nameToMatch = name[len(stripPath):]
	}
	if s.usePaths {
		return validPathKeyFormat.MatchString(nameToMatch)
	}
	return validKeyFormat.MatchString(nameToMatch)
}

func parameterMetaToSecretMeta(p *ssm.ParameterMetadata) SecretMetadata {
	version := 0
	if p.Description != nil {
		version, _ = strconv.Atoi(*p.Description)
	}
	return SecretMetadata{
		Created:   *p.LastModifiedDate,
		CreatedBy: *p.LastModifiedUser,
		Version:   version,
		Key:       *p.Name,
	}
}

func keys(m map[string]Secret) []string {
	keys := []string{}
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func values(m map[string]Secret) []Secret {
	values := []Secret{}
	for _, v := range m {
		values = append(values, v)
	}
	return values
}

func stringsToAWSStrings(slice []string) []*string {
	ret := []*string{}
	for _, s := range slice {
		ret = append(ret, aws.String(s))
	}
	return ret
}

func getChangeType(version int) ChangeEventType {
	if version == 1 {
		return Created
	}
	return Updated
}
