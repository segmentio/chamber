package store

import (
	"context"
	"errors"
)

var _ Store = &NullStore{}

type NullStore struct{}

func NewNullStore() *NullStore {
	return &NullStore{}
}

func (s *NullStore) Write(ctx context.Context, id SecretId, value string) error {
	return errors.New("Write is not implemented for Null Store")
}

func (s *NullStore) Read(ctx context.Context, id SecretId, version int) (Secret, error) {
	return Secret{}, errors.New("Not implemented for Null Store")
}

func (s *NullStore) WriteTags(ctx context.Context, id SecretId, tags map[string]string, deleteOtherTags bool) error {
	return errors.New("Not implemented for Null Store")
}

func (s *NullStore) ReadTags(ctx context.Context, id SecretId) (map[string]string, error) {
	return nil, errors.New("Not implemented for Null Store")
}

func (s *NullStore) ListServices(ctx context.Context, service string, includeSecretNames bool) ([]string, error) {
	return nil, nil
}

func (s *NullStore) List(ctx context.Context, service string, includeValues bool) ([]Secret, error) {
	return []Secret{}, nil
}

func (s *NullStore) ListRaw(ctx context.Context, service string) ([]RawSecret, error) {
	return []RawSecret{}, nil
}

func (s *NullStore) History(ctx context.Context, id SecretId) ([]ChangeEvent, error) {
	return []ChangeEvent{}, nil
}

func (s *NullStore) Delete(ctx context.Context, id SecretId) error {
	return errors.New("Not implemented for Null Store")
}

func (s *NullStore) DeleteTags(ctx context.Context, id SecretId, tags []string) error {
	return errors.New("Not implemented for Null Store")
}
