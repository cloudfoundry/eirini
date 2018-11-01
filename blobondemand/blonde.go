package blobondemand

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/ioutil"
)

type InMemoryStore struct {
	data map[string][]byte
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		data: make(map[string][]byte),
	}
}

func (s *InMemoryStore) Put(buf io.Reader) (string, int64, error) {
	data, err := ioutil.ReadAll(buf)
	if err != nil {
		return "", 0, err
	}
	id, size, err := getSha256(data)
	if err != nil {
		return "", 0, err
	}

	s.data[id] = data

	return id, size, nil
}

func getSha256(data []byte) (string, int64, error) {
	sha := sha256.New()
	size, err := sha.Write(data)
	if err != nil {
		return "", 0, err
	}

	id := "sha256:" + hex.EncodeToString(sha.Sum([]byte{}))

	return id, int64(size), nil
}

func (s *InMemoryStore) Has(digest string) bool {
	_, ok := s.data[digest]
	return ok
}

func (s *InMemoryStore) Get(digest string, w io.Writer) error {
	if _, ok := s.data[digest]; !ok {
		return nil
	}
	_, err := io.Copy(w, bytes.NewBuffer(s.data[digest]))
	return err
}
