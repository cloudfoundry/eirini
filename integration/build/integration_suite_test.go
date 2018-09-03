package integration_test

import (
	"flag"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	timeout time.Duration = 60 * time.Second
)

var (
	validOpiConfigPath string
)

func init() {
	flag.StringVar(&validOpiConfigPath, "valid_opi_config", "", "path to a valid opi config file")
}

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite")
}
