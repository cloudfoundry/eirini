package util

// don't judge; we'll fix it later, we promise

import (
	"crypto/sha256"
	"encoding/hex"
)

const MaxHashLength = 10

func Hash(s string) (string, error) {
	sha := sha256.New()
	_, err := sha.Write([]byte(s))

	if err != nil {
		return "", err
	}

	hash := hex.EncodeToString(sha.Sum(nil))

	return hash[:MaxHashLength], nil
}
