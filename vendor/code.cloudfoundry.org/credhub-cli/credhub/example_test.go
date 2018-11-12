package credhub_test

import (
	"encoding/json"
	"fmt"

	"code.cloudfoundry.org/credhub-cli/credhub"
	"code.cloudfoundry.org/credhub-cli/credhub/auth"
	"code.cloudfoundry.org/credhub-cli/credhub/credentials/generate"
)

func ExampleCredHub() {
	_ = func() {
		// Use a CredHub server on "https://example.com" using UAA password grant
		ch, err := credhub.New("https://example.com",
			credhub.SkipTLSValidation(true),
			credhub.Auth(auth.UaaPassword("credhub_cli", "", "username", "password")))

		if err != nil {
			panic("credhub client configured incorrectly: " + err.Error())
		}

		authUrl, err := ch.AuthURL()
		if err != nil {
			panic("couldn't fetch authurl")
		}

		fmt.Println("CredHub server: ", ch.ApiURL)
		fmt.Println("Auth server: ", authUrl)

		// Retrieve a password stored at "/my/password"
		password, err := ch.GetLatestPassword("/my/password")
		if err != nil {
			panic("password not found")
		}

		fmt.Println("My password: ", password.Value)

		// Manually refresh the access token
		uaa, ok := ch.Auth.(*auth.OAuthStrategy) // This works because we authenticated with auth.UaaPasswordGrant
		if !ok {
			panic("not using uaa")
		}

		fmt.Println("Old access token: ", uaa.AccessToken())

		uaa.Refresh() // For demo purposes only, tokens will be automatically refreshed by auth.OAuthStrategy

		fmt.Println("New access token:", uaa.AccessToken())
		// Sample Output:
		// CredHub server: https://example.com
		// Auth server: https://uaa.example.com
		// My password: random-password
		// Old access token: some-access-token
		// New access token: new-access-token
	}
}

func ExampleNew() {
	_ = func() {
		ch, _ := credhub.New(
			"https://example.com",
			credhub.SkipTLSValidation(true),
			credhub.Auth(auth.UaaClientCredentials("client-id", "client-secret")),
		)

		fmt.Println("Connected to ", ch.ApiURL)
	}
}

func ExampleCredHub_Request() {
	_ = func() {
		ch, _ := credhub.New("https://example.com")

		// Get encryption key usage
		response, err := ch.Request("GET", "/api/v1/key-usage", nil, nil, true)
		if err != nil {
			panic("couldn't get key usage")
		}

		var keyUsage map[string]int
		decoder := json.NewDecoder(response.Body)
		err = decoder.Decode(&keyUsage)
		if err != nil {
			panic("couldn't parse response")
		}

		fmt.Println("Active Key: ", keyUsage["active_key"])
		// Sample Output:
		// Active Key: 1231231
	}
}

func Example() {
	_ = func() {
		// CredHub server at https://example.com, using UAA Password grant
		ch, err := credhub.New("https://example.com",
			credhub.CaCerts(string("--- BEGIN ---\nroot-certificate\n--- END ---")),
			credhub.Auth(auth.UaaPassword("credhub_cli", "", "username", "password")),
		)

		// We'll be working with a certificate stored at "/my-certificates/the-cert"
		path := "/my-certificates/"
		name := "the-cert"

		// If the certificate already exists, delete it
		cert, err := ch.GetLatestCertificate(path + name)
		if err == nil {
			ch.Delete(cert.Name)
		}

		// Generate a new certificate
		gen := generate.Certificate{
			CommonName: "pivotal",
			KeyLength:  2048,
		}
		cert, err = ch.GenerateCertificate(path+name, gen, credhub.NoOverwrite)
		if err != nil {
			panic("couldn't generate certificate")
		}

		// Use the generated certificate's values to create a new certificate
		dupCert, err := ch.SetCertificate(path+"dup-cert", cert.Value)
		if err != nil {
			panic("couldn't create certificate")
		}

		if dupCert.Value.Certificate != cert.Value.Certificate {
			panic("certs don't match")
		}

		// List all credentials in "/my-certificates"
		creds, err := ch.FindByPath(path)
		if err != nil {
			panic("couldn't list certificates")
		}

		fmt.Println("Found the following credentials in " + path + ":")
		for _, cred := range creds.Credentials {
			fmt.Println(cred.Name)
		}
		// Sample Output:
		// Found the following credentials in /my-certificates:
		// /my-certificates/dup-cert
		// /my-certificates/the-cert
	}
}
