package inmemorygenerator

import (
	"go.uber.org/zap"
)

// InMemoryGenerator represents a secret generator that generates everything
// by itself, using no 3rd party tools
type InMemoryGenerator struct {
	Bits      int    // Key bits
	Expiry    int    // Expiration (days)
	Algorithm string // Algorithm type

	log *zap.SugaredLogger
}

// NewInMemoryGenerator creates a default InMemoryGenerator
func NewInMemoryGenerator(log *zap.SugaredLogger) *InMemoryGenerator {
	return &InMemoryGenerator{Bits: 2048, Expiry: 365, Algorithm: "rsa", log: log}
}
