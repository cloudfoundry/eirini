package util

import "math/rand"

const letters = "abcdefghijklmnopqrstuvwxyz1234567890"

func RandomString() string {
	b := make([]byte, 10)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
