package store

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
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
var validPathKeyFormat = regexp.MustCompile(`^\/[A-Za-z0-9-_/]+$`)

// validKeyFormat is the format that is expected for key names inside parameter store when
// not using paths
var validKeyFormat = regexp.MustCompile(`^[A-Za-z0-9-_.]+$`)

// ensure SSMStore confirms to Store interface
var _ Store = &SSMStore{}

// SSMStore implements the Store interface for storing secrets in SSM Parameter
// Store
type SSMStore struct {
	svc      ssmiface.SSMAPI
	usePaths bool
}

// NewSSMStore creates a new SSMStore
func NewSSMStore(numRetries int) *SSMStore {
	var region *string

	if regionOverride, ok := os.LookupEnv("CHAMBER_AWS_REGION"); ok {
		region = aws.String(regionOverride)
	}
	ssmSession := session.Must(session.NewSession(&aws.Config{
		Region: region,
	}))

	// If region is still not set, attempt to determine it via ec2 metadata API
	region = nil
	if aws.StringValue(ssmSession.Config.Region) == "" {
		session := session.New()
		ec2metadataSvc := ec2metadata.New(session)
		if regionOverride, err := ec2metadataSvc.Region(); err == nil {
			region = aws.String(regionOverride)
		}
	}
	svc := ssm.New(ssmSession, &aws.Config{
		MaxRetries: aws.Int(numRetries),
		Region:     region,
	})

	usePaths := true
	_, ok := os.LookupEnv("CHAMBER_NO_PATHS")
	if ok {
		usePaths = false
	}

	return &SSMStore{
		svc:      svc,
		usePaths: usePaths,
	}
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
	var parameter *ssm.ParameterMetadata
	var describeParametersInput *ssm.DescribeParametersInput

	// To get metadata, we need to use describe parameters

	if s.usePaths {
		// There is no way to use describe parameters to get a single key
		// if that key uses paths, so instead get all the keys for a path,
		// then find the one you are looking for :(
		describeParametersInput = &ssm.DescribeParametersInput{
			ParameterFilters: []*ssm.ParameterStringFilter{
				{
					Key:    aws.String("Path"),
					Option: aws.String("OneLevel"),
					Values: []*string{aws.String(basePath(s.idToName(id)))},
				},
			},
		}
	} else {
		describeParametersInput = &ssm.DescribeParametersInput{
			Filters: []*ssm.ParametersFilter{
				{
					Key:    aws.String("Name"),
					Values: []*string{aws.String(s.idToName(id))},
				},
			},
			MaxResults: aws.Int64(1),
		}
	}
	if err := s.svc.DescribeParametersPages(describeParametersInput, func(o *ssm.DescribeParametersOutput, lastPage bool) bool {
		for _, param := range o.Parameters {
			if *param.Name == s.idToName(id) {
				parameter = param
			}
		}
		return !lastPage
	}); err != nil {
		return Secret{}, err
	}

	if parameter == nil {
		return Secret{}, ErrSecretNotFound
	}

	secretMeta := parameterMetaToSecretMeta(parameter)

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

		if s.usePaths {
			describeParametersInput = &ssm.DescribeParametersInput{
				ParameterFilters: []*ssm.ParameterStringFilter{
					{
						Key:    aws.String("Path"),
						Option: aws.String("OneLevel"),
						Values: []*string{aws.String("/" + service)},
					},
				},
				MaxResults: aws.Int64(50),
				NextToken:  nextToken,
			}
		} else {
			describeParametersInput = &ssm.DescribeParametersInput{
				Filters: []*ssm.ParametersFilter{
					{
						Key:    aws.String("Name"),
						Values: []*string{aws.String(service + ".")},
					},
				},
				MaxResults: aws.Int64(50),
				NextToken:  nextToken,
			}
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

// ListRaw lists all secrets keys and values for a given service. Does not include any
// other meta-data. Uses faster AWS APIs with much higher rate-limits. Suitable for
// use in production environments.
func (s *SSMStore) ListRaw(service string) ([]RawSecret, error) {
	if s.usePaths {
		secrets := map[string]RawSecret{}
		var nextToken *string
		for {
			getParametersByPathInput := &ssm.GetParametersByPathInput{
				MaxResults:     aws.Int64(10),
				NextToken:      nextToken,
				Path:           aws.String("/" + service + "/"),
				WithDecryption: aws.Bool(true),
			}

			resp, err := s.svc.GetParametersByPath(getParametersByPathInput)
			if err != nil {
				// If the error is an access-denied exception
				awsErr, isAwserr := err.(awserr.Error)
				if isAwserr {
					if awsErr.Code() == "AccessDeniedException" && strings.Contains(awsErr.Message(), "is not authorized to perform: ssm:GetParametersByPath on resource") {
						// Fall-back to using the old list method in case some users haven't updated their IAM permissions yet, but warn about it and
						// tell them to fix their permissions
						fmt.Fprintf(
							os.Stderr,
							"Warning: %s\nFalling-back to using ssm:DescribeParameters. This may cause delays or failures due to AWS rate-limiting.\n"+
								"This is behavior deprecated and will be removed in a future version of chamber. Please update your IAM permissions to grant ssm:GetParametersByPath.\n\n",
							awsErr)

						// Delegate to List
						return s.listRawViaList(service)
					}
				}

				return nil, err
			}

			for _, param := range resp.Parameters {
				if !s.validateName(*param.Name) {
					continue
				}

				secrets[*param.Name] = RawSecret{
					Value: *param.Value,
					Key:   *param.Name,
				}
			}

			if resp.NextToken == nil {
				break
			}

			nextToken = resp.NextToken
		}

		rawSecrets := make([]RawSecret, len(secrets))
		i := 0
		for _, rawSecret := range secrets {
			rawSecrets[i] = rawSecret
			i += 1
		}
		return rawSecrets, nil

	}

	// Delete to List (which uses the DescribeParameters API)
	return s.listRawViaList(service)
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

func (s *SSMStore) listRawViaList(service string) ([]RawSecret, error) {
	// Delegate to List
	secrets, err := s.List(service, true)

	if err != nil {
		return nil, err
	}

	rawSecrets := make([]RawSecret, len(secrets))
	for i, secret := range secrets {
		rawSecrets[i] = RawSecret{
			Key: secret.Meta.Key,

			// This dereference is safe because we trust List to have given us the values
			// that we asked-for
			Value: *secret.Value,
		}
	}

	return rawSecrets, nil
}

func (s *SSMStore) idToName(id SecretId) string {
	if s.usePaths {
		return fmt.Sprintf("/%s/%s", id.Service, id.Key)
	}

	return fmt.Sprintf("%s.%s", id.Service, id.Key)
}

func (s *SSMStore) validateName(name string) bool {
	if s.usePaths {
		return validPathKeyFormat.MatchString(name)
	}
	return validKeyFormat.MatchString(name)
}

func basePath(key string) string {
	pathParts := strings.Split(key, "/")
	if len(pathParts) == 1 {
		return pathParts[0]
	}
	end := len(pathParts) - 1
	return strings.Join(pathParts[0:end], "/")
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
