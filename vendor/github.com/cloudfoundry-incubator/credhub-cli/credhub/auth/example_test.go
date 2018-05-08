package auth_test

import (
	"fmt"

	"github.com/cloudfoundry-incubator/credhub-cli/credhub"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub/auth"
)

func ExampleOAuth() {
	_ = func() {
		// To retrieve the access token from the CredHub client, use type assertion
		ch, err := credhub.New(
			"http://example.com",
			credhub.Auth(auth.UaaPassword("client-id", "client-secret", "username", "password")),
		)
		if err != nil {
			panic("couldn't connect to credhub")
		}

		oauth, ok := ch.Auth.(*auth.OAuthStrategy)
		if !ok {
			panic("Not using UAA")
		}

		fmt.Println("Before logging out: ", oauth.AccessToken())
		oauth.Logout()
		fmt.Println("After logging out: ", oauth.AccessToken())
		// Sample Output:
		// Before logging out: some-access-token
		// After logging out:
	}
}
