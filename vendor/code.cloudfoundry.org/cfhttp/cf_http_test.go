package cfhttp_test

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"time"

	"code.cloudfoundry.org/cfhttp"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CfHttp", func() {
	var timeout time.Duration

	BeforeEach(func() {
		timeout = 1 * time.Second
	})

	JustBeforeEach(func() {
		cfhttp.Initialize(timeout)
	})

	Describe("NewClient", func() {
		It("returns an http client", func() {
			client := cfhttp.NewClient()
			Expect(client.Timeout).To(Equal(timeout))
			transport := client.Transport.(*http.Transport)
			Expect(transport.DialContext).NotTo(BeNil())
			Expect(transport.DisableKeepAlives).To(BeFalse())
		})
	})

	Describe("NewUnixClient", func() {
		It("returns an http client", func() {
			client := cfhttp.NewUnixClient("socketPath")
			Expect(client.Timeout).To(Equal(timeout))
			transport := client.Transport.(*http.Transport)
			Expect(transport.Dial).NotTo(BeNil())
		})
	})

	Describe("NewCustomTimeoutClient", func() {
		It("returns an http client with specified timeout", func() {
			client := cfhttp.NewCustomTimeoutClient(5 * time.Second)
			Expect(client.Timeout).To(Equal(5 * time.Second))
			transport := client.Transport.(*http.Transport)
			Expect(transport.DialContext).NotTo(BeNil())
			Expect(transport.DisableKeepAlives).To(BeFalse())
		})
	})

	Describe("NewStreamingClient", func() {
		It("returns an http client", func() {
			client := cfhttp.NewStreamingClient()
			Expect(client.Timeout).To(BeZero())
			transport := client.Transport.(*http.Transport)
			Expect(transport.DialContext).NotTo(BeNil())
			Expect(transport.DisableKeepAlives).To(BeFalse())
		})
	})

	Describe("NewTLSConfig", func() {
		var certFixture, keyFixture, caCertFixture string

		BeforeEach(func() {
			certFixture = "fixtures/cert.crt"
			keyFixture = "fixtures/key.key"
			caCertFixture = "fixtures/cacert.crt"
		})

		It("requires TLS Version 1.2", func() {
			tlsConfig, err := cfhttp.NewTLSConfig(certFixture, keyFixture, caCertFixture)
			Expect(err).NotTo(HaveOccurred())
			Expect(tlsConfig.MinVersion).To(BeEquivalentTo(tls.VersionTLS12))
		})

		It("requires certain cipher suites", func() {
			tlsConfig, err := cfhttp.NewTLSConfig(certFixture, keyFixture, caCertFixture)
			Expect(err).NotTo(HaveOccurred())
			Expect(tlsConfig.CipherSuites).To(Equal(cfhttp.SUPPORTED_CIPHER_SUITES))
		})
	})

	Describe("NewTLSConfigWithCertPool", func() {
		var certFixture, keyFixture string
		var caCertPool *x509.CertPool

		BeforeEach(func() {
			certFixture = "fixtures/cert.crt"
			keyFixture = "fixtures/key.key"
			caCertPool = x509.NewCertPool()
		})

		It("sets the Root and Client CAs", func() {
			tlsConfig, err := cfhttp.NewTLSConfigWithCertPool(certFixture, keyFixture, caCertPool)
			Expect(err).NotTo(HaveOccurred())
			Expect(tlsConfig.RootCAs).To(Equal(caCertPool))
			Expect(tlsConfig.ClientCAs).To(Equal(caCertPool))
		})

		It("should error when the caCert is nil", func() {
			_, err := cfhttp.NewTLSConfigWithCertPool(certFixture, keyFixture, nil)
			Expect(err).To(HaveOccurred())
		})
	})
})
