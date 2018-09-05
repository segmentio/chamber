package store

import (
	"errors"
)

var _ Store = &NoneStore{}

type NoneStore struct{}

func NewNoneStore() *NoneStore {
	return &NoneStore{}
}

func (s *NoneStore) Write(id SecretId, value string) error {
	return errors.New("Write is not implemented for None Store")
}

func (s *NoneStore) Read(id SecretId, version int) (Secret, error) {
	return Secret{}, errors.New("Not implemented for None Store")
}

func (s *NoneStore) List(service string, includeValues bool) ([]Secret, error) {
	return []Secret{}, nil
}

func (s *NoneStore) ListRaw(service string) ([]RawSecret, error) {
	return []RawSecret{}, nil
}

func (s *NoneStore) History(id SecretId) ([]ChangeEvent, error) {
	return []ChangeEvent{}, nil
}

func (s *NoneStore) Delete(id SecretId) error {
	return errors.New("Not implemented for None Store")
}
