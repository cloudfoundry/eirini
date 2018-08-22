package auth_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestAuth(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CredHub API Auth Suite")
}

type dummyUaaClient struct {
	ClientId     string
	ClientSecret string
	Username     string
	Password     string
	RefreshToken string
	RevokedToken string

	NewAccessToken  string
	NewRefreshToken string
	Error           error
}

func (d *dummyUaaClient) ClientCredentialGrant(clientId, clientSecret string) (string, error) {
	d.ClientId = clientId
	d.ClientSecret = clientSecret

	return d.NewAccessToken, d.Error
}

func (d *dummyUaaClient) PasswordGrant(clientId, clientSecret, username, password string) (string, string, error) {
	d.ClientId = clientId
	d.ClientSecret = clientSecret
	d.Username = username
	d.Password = password

	return d.NewAccessToken, d.NewRefreshToken, d.Error
}

func (d *dummyUaaClient) RefreshTokenGrant(clientId, clientSecret, refreshToken string) (string, string, error) {
	d.ClientId = clientId
	d.ClientSecret = clientSecret
	d.RefreshToken = refreshToken

	return d.NewAccessToken, d.NewRefreshToken, d.Error
}

func (d *dummyUaaClient) RevokeToken(token string) error {
	d.RevokedToken = token
	return d.Error
}
