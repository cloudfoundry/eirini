package taskworkpool_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestTaskworkpool(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Taskworkpool Suite")
}
