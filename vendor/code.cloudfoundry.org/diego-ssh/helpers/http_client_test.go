package helpers_test

import (
	"crypto/x509"
	"io/ioutil"
	"net/http"
	"time"

	"code.cloudfoundry.org/diego-ssh/helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("NewHTTPSClient", func() {
	var (
		uaaCACert          string
		insecureSkipVerify bool
		timeout            time.Duration
	)

	BeforeEach(func() {
		uaaCACert = ""
	})

	It("sets InsecureSkipVerify on the TLS config", func() {
		client, err := helpers.NewHTTPSClient(true, uaaCACert, timeout)
		Expect(err).NotTo(HaveOccurred())
		httpTrans, ok := client.Transport.(*http.Transport)
		Expect(ok).To(BeTrue())
		Expect(httpTrans.TLSClientConfig.InsecureSkipVerify).To(BeTrue())
	})

	It("sets the client timeout", func() {
		client, err := helpers.NewHTTPSClient(insecureSkipVerify, uaaCACert, 5*time.Second)
		Expect(err).NotTo(HaveOccurred())
		Expect(client.Timeout).To(Equal(5 * time.Second))
	})

	Context("when a ca Cert file is provided", func() {
		BeforeEach(func() {
			uaaCACert = "fixtures/ca_cert.crt"
		})

		It("sets the RootCAs with a pool consisting of that CA", func() {
			certBytes, err := ioutil.ReadFile(uaaCACert)
			Expect(err).NotTo(HaveOccurred())

			expectedPool := x509.NewCertPool()
			Expect(expectedPool.AppendCertsFromPEM(certBytes)).To(BeTrue())

			client, err := helpers.NewHTTPSClient(insecureSkipVerify, uaaCACert, timeout)
			Expect(err).NotTo(HaveOccurred())
			httpTrans, ok := client.Transport.(*http.Transport)
			Expect(ok).To(BeTrue())

			caPool := httpTrans.TLSClientConfig.RootCAs

			Expect(expectedPool).To(Equal(caPool))
		})

		Context("when the UAA tls cert is invalid", func() {
			BeforeEach(func() {
				uaaCACert = "fixtures/invalid.crt"
			})

			It("returns an error", func() {
				_, err := helpers.NewHTTPSClient(insecureSkipVerify, uaaCACert, timeout)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Unable to load caCert"))
			})
		})

		Context("when the UAA tls cert does not exist", func() {
			BeforeEach(func() {
				uaaCACert = "nonexistentpath"
			})

			It("returns an error", func() {
				_, err := helpers.NewHTTPSClient(insecureSkipVerify, uaaCACert, timeout)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to read ca cert file"))
			})
		})
	})
})
