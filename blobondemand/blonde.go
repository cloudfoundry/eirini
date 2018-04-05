package blobondemand

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
)

type InMemoryStore struct {
	data map[string]*bytes.Buffer
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		data: make(map[string]*bytes.Buffer),
	}
}

func (s *InMemoryStore) Put(buf io.Reader) (id string, size int64, err error) {
	sum := sha256.New()
	stored := &bytes.Buffer{}
	if size, err = io.Copy(io.MultiWriter(sum, stored), buf); err != nil {
		return "", 0, err
	}

	id = "sha256:" + hex.EncodeToString(sum.Sum(nil))
	s.data[id] = stored

	return id, size, nil
}

func (s *InMemoryStore) PutWithId(guid string, buf io.Reader) (id string, size int64, err error) {
	sum := sha256.New()
	stored := &bytes.Buffer{}
	if size, err = io.Copy(io.MultiWriter(sum, stored), buf); err != nil {
		return "", 0, err
	}

	id = "sha256:" + hex.EncodeToString(sum.Sum(nil))
	s.data[guid] = stored

	return id, size, nil
}

func (s *InMemoryStore) Has(digest string) bool {
	_, ok := s.data[digest]
	return ok
}

func (s *InMemoryStore) Get(digest string, w io.Writer) error {
	_, err := io.Copy(w, s.data[digest])
	return err
}
