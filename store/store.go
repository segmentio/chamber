package store

import (
	"context"
	"errors"
	"time"
)

const (
	// ChamberService is the name of the service reserved for chamber's own use.
	ChamberService = "_chamber"
)

func ReservedService(service string) bool {
	return service == ChamberService
}

const (
	LatestStoreConfigVersion = "1"
)

// StoreConfig holds configuration information for a store. WARNING: Despite
// its public visibility, the contents of this struct are subject to change at
// any time, and are not part of the public interface for chamber.
type StoreConfig struct {
	Version      string   `json:"version"`
	RequiredTags []string `json:"requiredTags,omitempty"`
}

type ChangeEventType int

const (
	Created ChangeEventType = iota
	Updated
)

func (c ChangeEventType) String() string {
	switch c {
	case Created:
		return "Created"
	case Updated:
		return "Updated"
	}
	return "unknown"
}

var (
	// ErrSecretNotFound is returned if the specified secret is not found in the
	// parameter store
	ErrSecretNotFound = errors.New("secret not found")
)

// SecretId is the compound key for a secret.
type SecretId struct {
	Service string
	Key     string
}

// Secret is a secret with metadata.
type Secret struct {
	Value *string
	Meta  SecretMetadata
}

// RawSecret is a secret without any metadata.
type RawSecret struct {
	Value string
	Key   string
}

// SecretMetadata is metadata about a secret.
type SecretMetadata struct {
	Created   time.Time
	CreatedBy string
	Version   int
	Key       string
}

type ChangeEvent struct {
	Type    ChangeEventType
	Time    time.Time
	User    string
	Version int
}

// Store is an interface for a secret store.
type Store interface {
	Config(ctx context.Context) (StoreConfig, error)
	SetConfig(ctx context.Context, config StoreConfig) error
	Write(ctx context.Context, id SecretId, value string) error
	WriteWithTags(ctx context.Context, id SecretId, value string, tags map[string]string) error
	Read(ctx context.Context, id SecretId, version int) (Secret, error)
	WriteTags(ctx context.Context, id SecretId, tags map[string]string, deleteOtherTags bool) error
	ReadTags(ctx context.Context, id SecretId) (map[string]string, error)
	List(ctx context.Context, service string, includeValues bool) ([]Secret, error)
	ListRaw(ctx context.Context, service string) ([]RawSecret, error)
	ListServices(ctx context.Context, service string, includeSecretName bool) ([]string, error)
	History(ctx context.Context, id SecretId) ([]ChangeEvent, error)
	Delete(ctx context.Context, id SecretId) error
	DeleteTags(ctx context.Context, id SecretId, tagKeys []string) error
}

func requiredTags(ctx context.Context, s Store) ([]string, error) {
	config, err := s.Config(ctx)
	if err != nil {
		return nil, err
	}
	return config.RequiredTags, nil
}
