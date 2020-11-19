package store

import (
	"errors"
)

var _ Store = &NullStore{}

type NullStore struct{}

func NewNullStore() *NullStore {
	return &NullStore{}
}

func (s *NullStore) Write(id SecretId, value string) error {
	return errors.New("Write is not implemented for Null Store")
}

func (s *NullStore) WriteInclude(id SecretId, includeService string) error {
	return errors.New("WriteInclude is not implemented for Null Store")
}

func (s *NullStore) Read(id SecretId, version int) (Secret, error) {
	return Secret{}, errors.New("Not implemented for Null Store")
}

func (s *NullStore) ListServices(service string, includeSecretNames bool) ([]string, error) {
	return nil, nil
}

func (s *NullStore) List(service string, includeValues bool) ([]Secret, error) {
	return []Secret{}, nil
}

func (s *NullStore) ListRaw(service string) ([]RawSecret, error) {
	return []RawSecret{}, nil
}

func (s *NullStore) History(id SecretId) ([]ChangeEvent, error) {
	return []ChangeEvent{}, nil
}

func (s *NullStore) Delete(id SecretId) error {
	return errors.New("Not implemented for Null Store")
}
