package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

const (
	// CustomSSMEndpointEnvVar is the name of the environment variable specifying a custom base SSM
	// endpoint.
	CustomSSMEndpointEnvVar = "CHAMBER_AWS_SSM_ENDPOINT"

	// DefaultKeyID is the default alias for the KMS key used to encrypt/decrypt secrets
	DefaultKeyID = "alias/parameter_store_key"

	// DefaultRetryMode is the default retry mode for AWS SDK configurations.
	DefaultRetryMode = aws.RetryModeStandard
)

// validPathKeyFormat is the format that is expected for key names inside parameter store
// when using paths
var validPathKeyFormat = regexp.MustCompile(`^(\/[\w\-\.]+)+$`)

// ensure SSMStore confirms to Store interface
var _ Store = &SSMStore{}

// label check regexp
var labelMatchRegex = regexp.MustCompile(`^(\/[\w\-\.]+)+:(.+)$`)

// SSMStore implements the Store interface for storing secrets in SSM Parameter
// Store
type SSMStore struct {
	svc    apiSSM
	config aws.Config
}

// NewSSMStore creates a new SSMStore
func NewSSMStore(ctx context.Context, numRetries int) (*SSMStore, error) {
	return ssmStoreUsingRetryer(ctx, numRetries, DefaultRetryMode)
}

// NewSSMStoreWithMinThrottleDelay creates a new SSMStore with the aws sdk max retries and min throttle delay are configured.
//
// Deprecated: The AWS SDK no longer supports specifying a minimum throttle delay. Instead, use
// NewSSMStoreWithRetryMode.
func NewSSMStoreWithMinThrottleDelay(ctx context.Context, numRetries int, minThrottleDelay time.Duration) (*SSMStore, error) {
	return ssmStoreUsingRetryer(ctx, numRetries, DefaultRetryMode)
}

// NewSSMStoreWithRetryMode creates a new SSMStore, configuring the underlying AWS SDK with the
// given maximum number of retries and retry mode.
func NewSSMStoreWithRetryMode(ctx context.Context, numRetries int, retryMode aws.RetryMode) (*SSMStore, error) {
	return ssmStoreUsingRetryer(ctx, numRetries, retryMode)
}

func ssmStoreUsingRetryer(ctx context.Context, numRetries int, retryMode aws.RetryMode) (*SSMStore, error) {
	cfg, _, err := getConfig(ctx, numRetries, retryMode)
	if err != nil {
		return nil, err
	}
	customSsmEndpoint, ok := os.LookupEnv(CustomSSMEndpointEnvVar)
	if ok {
		cfg.BaseEndpoint = aws.String(customSsmEndpoint)
	}

	svc := ssm.NewFromConfig(cfg)

	return &SSMStore{
		svc:    svc,
		config: cfg,
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

const (
	storeConfigKey = "store-config"
)

var (
	storeConfigID = SecretId{
		Service: ChamberService,
		Key:     storeConfigKey,
	}
)

func (s *SSMStore) Config(ctx context.Context) (StoreConfig, error) {
	configSecret, err := s.readLatest(ctx, storeConfigID)
	if err != nil {
		if err == ErrSecretNotFound {
			return StoreConfig{
				Version: LatestStoreConfigVersion,
			}, nil
		} else {
			return StoreConfig{}, err
		}
	}

	var config StoreConfig
	if err := json.Unmarshal([]byte(*configSecret.Value), &config); err != nil {
		return StoreConfig{}, fmt.Errorf("failed to unmarshal store config: %w", err)
	}
	return config, nil
}

func (s *SSMStore) SetConfig(ctx context.Context, config StoreConfig) error {
	configBytes, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal store config: %w", err)
	}

	err = s.write(ctx, storeConfigID, string(configBytes), nil)
	if err != nil {
		return fmt.Errorf("failed to write store config: %w", err)
	}
	return nil
}

// Write writes a given value to a secret identified by id.  If the secret
// already exists, then write a new version.
func (s *SSMStore) Write(ctx context.Context, id SecretId, value string) error {
	return s.write(ctx, id, value, nil)
}

func (s *SSMStore) WriteWithTags(ctx context.Context, id SecretId, value string, tags map[string]string) error {
	return s.write(ctx, id, value, tags)
}

func (s *SSMStore) write(ctx context.Context, id SecretId, value string, tags map[string]string) error {
	version := 1
	// first read to get the current version
	current, err := s.Read(ctx, id, -1)
	if err != nil && err != ErrSecretNotFound {
		return err
	}
	if err == nil {
		version = current.Meta.Version + 1
	}

	if len(tags) > 0 && version != 1 {
		return errors.New("tags on write only supported for new secrets")
	}

	err = s.checkForRequiredTags(ctx, tags, version)
	if err != nil {
		return err
	}

	putParameterInput := &ssm.PutParameterInput{
		KeyId:       aws.String(s.KMSKey()),
		Name:        aws.String(s.idToName(id)),
		Type:        types.ParameterTypeSecureString,
		Value:       aws.String(value),
		Overwrite:   aws.Bool(true),
		Description: aws.String(strconv.Itoa(version)),
	}

	// This API call returns an empty struct
	_, err = s.svc.PutParameter(ctx, putParameterInput)
	if err != nil {
		return err
	}

	if len(tags) > 0 {
		if err := s.WriteTags(ctx, id, tags, false); err != nil {
			return fmt.Errorf("failed to write tags on successfully created secret: %w", err)
		}
	}

	return nil
}

func (s *SSMStore) checkForRequiredTags(ctx context.Context, tags map[string]string, version int) error {
	if version != 1 {
		return nil
	}
	requiredTags, err := requiredTags(ctx, s)
	if err != nil {
		return err
	}

	var missingTags []string
	for _, requiredTag := range requiredTags {
		if _, ok := tags[requiredTag]; !ok {
			missingTags = append(missingTags, requiredTag)
		}
	}
	if len(missingTags) > 0 {
		return fmt.Errorf("required tags %v are missing", missingTags)
	}

	return nil
}

// Read reads a secret from the parameter store at a specific version.
// To grab the latest version, use -1 as the version number.
func (s *SSMStore) Read(ctx context.Context, id SecretId, version int) (Secret, error) {
	if version == -1 {
		return s.readLatest(ctx, id)
	}

	return s.readVersion(ctx, id, version)
}

func (s *SSMStore) WriteTags(ctx context.Context, id SecretId, tags map[string]string, deleteOtherTags bool) error {
	if deleteOtherTags {
		// list the current tags
		currentTags, err := s.ReadTags(ctx, id)
		if err != nil {
			return err
		}

		// fail if any required tags are already present but not being written, because they'd be deleted
		// (a required tag that hasn't been set yet may be left out)
		err = s.checkForPresentRequiredTags(ctx, currentTags, tags)
		if err != nil {
			return err
		}

		// remove any tags that are not being written
		var tagKeysToRemove []string
		for k := range currentTags {
			if _, ok := tags[k]; !ok {
				tagKeysToRemove = append(tagKeysToRemove, k)
			}
		}

		if len(tagKeysToRemove) > 0 {
			if err := s.DeleteTags(ctx, id, tagKeysToRemove); err != nil {
				return err
			}
		}
	}

	// add or update tags
	addTags := make([]types.Tag, len(tags))
	i := 0
	for k, v := range tags {
		addTags[i] = types.Tag{
			Key:   aws.String(k),
			Value: aws.String(v),
		}
		i++
	}

	addTagsInput := &ssm.AddTagsToResourceInput{
		ResourceType: types.ResourceTypeForTaggingParameter,
		ResourceId:   aws.String(s.idToName(id)),
		Tags:         addTags,
	}
	_, err := s.svc.AddTagsToResource(ctx, addTagsInput)
	if err != nil {
		return err
	}

	return nil
}

// checkForPresentRequiredTags returns an error if the given map of tags is
// missing any required tags that are already present (in currentTags). This is
// a problem only for a tag write command where any tags not being written are
// to be deleted ("delete other tags"), because that would cause some required
// tags to be deleted. Instead, the caller has to explicitly provide values for
// all required tags, even if they aren't changing.
func (s *SSMStore) checkForPresentRequiredTags(ctx context.Context, currentTags map[string]string, tags map[string]string) error {
	requiredTags, err := requiredTags(ctx, s)
	if err != nil {
		return err
	}
	var missingTags []string
	for _, requiredTag := range requiredTags {
		_, alreadyPresent := currentTags[requiredTag]
		_, beingUpdated := tags[requiredTag]
		if alreadyPresent && !beingUpdated {
			// this required tag is present already but isn't being rewritten, which
			// is a problem when "delete other tags" is set
			missingTags = append(missingTags, requiredTag)
		}
	}
	if len(missingTags) > 0 {
		return fmt.Errorf("required tags %v are already present, so they must be rewritten", missingTags)
	}

	return nil
}

func (s *SSMStore) ReadTags(ctx context.Context, id SecretId) (map[string]string, error) {
	input := &ssm.ListTagsForResourceInput{
		ResourceType: types.ResourceTypeForTaggingParameter,
		ResourceId:   aws.String(s.idToName(id)),
	}
	resp, err := s.svc.ListTagsForResource(ctx, input)
	if err != nil {
		var iri *types.InvalidResourceId
		if errors.As(err, &iri) {
			return nil, ErrSecretNotFound
		}
		return nil, err
	}

	tags := make(map[string]string, len(resp.TagList))
	for _, tag := range resp.TagList {
		tags[*tag.Key] = *tag.Value
	}
	return tags, nil
}

// Delete removes a secret from the parameter store. Note this removes all
// versions of the secret.
func (s *SSMStore) Delete(ctx context.Context, id SecretId) error {
	// first read to ensure parameter present
	_, err := s.Read(ctx, id, -1)
	if err != nil {
		return err
	}

	deleteParameterInput := &ssm.DeleteParameterInput{
		Name: aws.String(s.idToName(id)),
	}

	_, err = s.svc.DeleteParameter(ctx, deleteParameterInput)
	if err != nil {
		return err
	}

	return nil
}

func (s *SSMStore) DeleteTags(ctx context.Context, id SecretId, tagKeys []string) error {
	err := s.checkIfDeletingRequiredTags(ctx, tagKeys)
	if err != nil {
		return err
	}

	removeTagsInput := &ssm.RemoveTagsFromResourceInput{
		ResourceType: types.ResourceTypeForTaggingParameter,
		ResourceId:   aws.String(s.idToName(id)),
		TagKeys:      tagKeys,
	}
	_, err = s.svc.RemoveTagsFromResource(ctx, removeTagsInput)
	if err != nil {
		return err
	}

	return nil
}

func (s *SSMStore) checkIfDeletingRequiredTags(ctx context.Context, tagKeys []string) error {
	requiredTags, err := requiredTags(ctx, s)
	if err != nil {
		return err
	}
	tags := make(map[string]any)
	for _, key := range tagKeys {
		tags[key] = struct{}{}
	}

	var foundTags []string
	for _, requiredTag := range requiredTags {
		if _, ok := tags[requiredTag]; ok {
			foundTags = append(foundTags, requiredTag)
		}
	}
	if len(foundTags) > 0 {
		return fmt.Errorf("required tags %v may not be deleted", foundTags)
	}

	return nil
}

func (s *SSMStore) readVersion(ctx context.Context, id SecretId, version int) (Secret, error) {
	getParameterHistoryInput := &ssm.GetParameterHistoryInput{
		Name:           aws.String(s.idToName(id)),
		WithDecryption: aws.Bool(true),
	}

	var result Secret
	paginator := ssm.NewGetParameterHistoryPaginator(s.svc, getParameterHistoryInput)
	for paginator.HasMorePages() {
		o, err := paginator.NextPage(ctx)
		if err != nil {
			return Secret{}, ErrSecretNotFound
		}
		for _, history := range o.Parameters {
			thisVersion := 0
			if history.Description != nil {
				thisVersion, _ = strconv.Atoi(*history.Description)
			}
			if thisVersion == version {
				result = Secret{
					Value: history.Value,
					Meta: SecretMetadata{
						Created:   *history.LastModifiedDate,
						CreatedBy: *history.LastModifiedUser,
						Version:   thisVersion,
						Key:       *history.Name,
					},
				}
				break
			}
		}
	}
	if result.Value != nil {
		return result, nil
	}

	return Secret{}, ErrSecretNotFound
}

func (s *SSMStore) readLatest(ctx context.Context, id SecretId) (Secret, error) {
	getParametersInput := &ssm.GetParametersInput{
		Names:          []string{s.idToName(id)},
		WithDecryption: aws.Bool(true),
	}

	resp, err := s.svc.GetParameters(ctx, getParametersInput)
	if err != nil {
		return Secret{}, err
	}

	if len(resp.Parameters) == 0 {
		return Secret{}, ErrSecretNotFound
	}
	param := resp.Parameters[0]
	var parameter *types.ParameterMetadata

	// To get metadata, we need to use describe parameters

	// There is no way to use describe parameters to get a single key
	// if that key uses paths, so instead get all the keys for a path,
	// then find the one you are looking for :(
	describeParametersInput := &ssm.DescribeParametersInput{
		ParameterFilters: []types.ParameterStringFilter{
			{
				Key:    aws.String("Path"),
				Option: aws.String("OneLevel"),
				Values: []string{basePath(s.idToName(id))},
			},
		},
	}
	paginator := ssm.NewDescribeParametersPaginator(s.svc, describeParametersInput)
	for paginator.HasMorePages() {
		o, err := paginator.NextPage(ctx)
		if err != nil {
			return Secret{}, err
		}
		for _, param := range o.Parameters {
			if *param.Name == s.idToName(id) {
				parameter = &param
				break
			}
		}
	}

	if parameter == nil {
		return Secret{}, ErrSecretNotFound
	}

	secretMeta := parameterMetaToSecretMeta(*parameter)

	return Secret{
		Value: param.Value,
		Meta:  secretMeta,
	}, nil
}

func (s *SSMStore) ListServices(ctx context.Context, service string, includeSecretName bool) ([]string, error) {
	secrets := map[string]Secret{}

	describeParametersInput := &ssm.DescribeParametersInput{
		MaxResults: aws.Int32(50),
		ParameterFilters: []types.ParameterStringFilter{
			{
				Key:    aws.String("Name"),
				Option: aws.String("BeginsWith"),
				Values: []string{"/" + service},
			},
		},
	}

	paginator := ssm.NewDescribeParametersPaginator(s.svc, describeParametersInput)
	for paginator.HasMorePages() {
		resp, err := paginator.NextPage(ctx)
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
	}

	if includeSecretName {
		return keys(secrets), nil
	}

	var services []string
	for key := range secrets {
		services = append(services, serviceName(key))
	}

	return uniqueStringSlice(services), nil
}

// List lists all secrets for a given service.  If includeValues is true,
// then those secrets are decrypted and returned, otherwise only the metadata
// about a secret is returned.
func (s *SSMStore) List(ctx context.Context, serviceName string, includeValues bool) ([]Secret, error) {
	secrets := map[string]Secret{}

	var describeParametersInput *ssm.DescribeParametersInput

	service, _ := parseServiceLabel(serviceName)

	describeParametersInput = &ssm.DescribeParametersInput{
		ParameterFilters: []types.ParameterStringFilter{
			{
				Key:    aws.String("Path"),
				Option: aws.String("OneLevel"),
				Values: []string{"/" + service},
			},
		},
	}

	paginator := ssm.NewDescribeParametersPaginator(s.svc, describeParametersInput)
	for paginator.HasMorePages() {
		resp, err := paginator.NextPage(ctx)
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
				Names:          batch,
				WithDecryption: aws.Bool(true),
			}

			resp, err := s.svc.GetParameters(ctx, getParametersInput)
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
func (s *SSMStore) ListRaw(ctx context.Context, serviceName string) ([]RawSecret, error) {
	service, label := parseServiceLabel(serviceName)
	secrets := map[string]RawSecret{}
	getParametersByPathInput := &ssm.GetParametersByPathInput{
		Path:           aws.String("/" + service + "/"),
		WithDecryption: aws.Bool(true),
	}
	if label != "" {
		getParametersByPathInput.ParameterFilters = []types.ParameterStringFilter{
			{
				Key:    aws.String("Label"),
				Option: aws.String("Equals"),
				Values: []string{label},
			},
		}
	}

	paginator := ssm.NewGetParametersByPathPaginator(s.svc, getParametersByPathInput)
	for paginator.HasMorePages() {
		resp, err := paginator.NextPage(ctx)
		if err != nil {
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
	}

	rawSecrets := make([]RawSecret, len(secrets))
	i := 0
	for _, rawSecret := range secrets {
		rawSecrets[i] = rawSecret
		i += 1
	}
	return rawSecrets, nil
}

// History returns a list of events that have occurred regarding the given
// secret.
func (s *SSMStore) History(ctx context.Context, id SecretId) ([]ChangeEvent, error) {
	events := []ChangeEvent{}

	getParameterHistoryInput := &ssm.GetParameterHistoryInput{
		Name:           aws.String(s.idToName(id)),
		WithDecryption: aws.Bool(false),
	}

	paginator := ssm.NewGetParameterHistoryPaginator(s.svc, getParameterHistoryInput)
	for paginator.HasMorePages() {
		o, err := paginator.NextPage(ctx)
		if err != nil {
			return events, ErrSecretNotFound
		}

		for _, history := range o.Parameters {
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
	}

	return events, nil
}

func (s *SSMStore) idToName(id SecretId) string {
	return fmt.Sprintf("/%s/%s", id.Service, id.Key)
}

func (s *SSMStore) validateName(name string) bool {
	return validPathKeyFormat.MatchString(name)
}

func basePath(key string) string {
	pathParts := strings.Split(key, "/")
	if len(pathParts) == 1 {
		return pathParts[0]
	}
	end := len(pathParts) - 1
	return strings.Join(pathParts[0:end], "/")
}

func serviceName(key string) string {
	pathParts := strings.Split(key, "/")
	if len(pathParts) == 1 {
		return pathParts[0]
	}
	end := len(pathParts) - 1
	return strings.Join(pathParts[1:end], "/")
}

func parameterMetaToSecretMeta(p types.ParameterMetadata) SecretMetadata {
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

func getChangeType(version int) ChangeEventType {
	if version == 1 {
		return Created
	}
	return Updated
}

func parseServiceLabel(serviceAndLabel string) (string, string) {
	if labelMatchRegex.MatchString(serviceAndLabel) {
		i := strings.Index(serviceAndLabel, ":")

		if i > -1 {
			service := serviceAndLabel[:i]
			label := serviceAndLabel[i+1:]
			return service, label
		}

		return serviceAndLabel, ""
	}

	return serviceAndLabel, ""
}
