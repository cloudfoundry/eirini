package credhub_test

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/cloudfoundry-incubator/credhub-cli/credhub/auth"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestCredhub(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CredHub API Client Suite")
}

type DummyAuth struct {
	Config   auth.Config
	Request  *http.Request
	Response *http.Response
	Error    error
}

func (d *DummyAuth) Do(req *http.Request) (*http.Response, error) {
	d.Request = req

	return d.Response, d.Error
}

func (d *DummyAuth) Builder() auth.Builder {
	return func(config auth.Config) (auth.Strategy, error) {
		return d, nil
	}
}

var _ auth.Strategy = new(DummyAuth)

type NotMarshallable struct{}

func (u *NotMarshallable) MarshalJSON() ([]byte, error) {
	return nil, errors.New("I cannot be marshalled")
}

var _ json.Marshaler = new(NotMarshallable)
