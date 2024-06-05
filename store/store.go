package store

import (
	"context"
	"errors"
	"time"
)

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

type SecretId struct {
	Service string
	Key     string
}

type Secret struct {
	Value *string
	Meta  SecretMetadata
}

// A secret without any metadata
type RawSecret struct {
	Value string
	Key   string
}

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

type Store interface {
	Write(ctx context.Context, id SecretId, value string) error
	Read(ctx context.Context, id SecretId, version int) (Secret, error)
	List(ctx context.Context, service string, includeValues bool) ([]Secret, error)
	ListRaw(ctx context.Context, service string) ([]RawSecret, error)
	ListServices(ctx context.Context, service string, includeSecretName bool) ([]string, error)
	History(ctx context.Context, id SecretId) ([]ChangeEvent, error)
	Delete(ctx context.Context, id SecretId) error
}
