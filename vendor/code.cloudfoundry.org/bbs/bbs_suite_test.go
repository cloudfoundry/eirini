package bbs_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestBbs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bbs Suite")
}
