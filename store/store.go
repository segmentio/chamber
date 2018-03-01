package store

import (
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
	Id    SecretId
	Value *string
	Meta  SecretMetadata
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
	Write(id SecretId, value string) error
	Read(id SecretId, version int) (Secret, error)
	List(service string, includeValues bool) ([]Secret, error)
	History(id SecretId) ([]ChangeEvent, error)
	Delete(id SecretId) error
}
