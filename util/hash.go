package util

// don't judge; we'll fix it later, we promise

import (
	"crypto/sha256"
	"encoding/hex"
)

const MAX_K8S_LABEL_LENGTH = 50

//go:generate counterfeiter . Hasher
type Hasher interface {
	Hash(s string) (string, error)
}

type TruncatedSHA256Hasher struct {
}

func (h *TruncatedSHA256Hasher) Hash(s string) (string, error) {
	sha := sha256.New()
	_, err := sha.Write([]byte(s))
	if err != nil {
		return "", err
	}

	hash := hex.EncodeToString(sha.Sum(nil))
	return hash[:MAX_K8S_LABEL_LENGTH], nil
}
