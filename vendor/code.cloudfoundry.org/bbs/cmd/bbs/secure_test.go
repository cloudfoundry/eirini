package main_test

import (
	"net/http"
	"os"
	"path"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/cmd/bbs/testrunner"
	"code.cloudfoundry.org/bbs/models/test/model_helpers"
	"code.cloudfoundry.org/cfhttp"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Secure", func() {
	var (
		client bbs.InternalClient
		err    error

		basePath string
	)

	BeforeEach(func() {
		basePath = path.Join(os.Getenv("GOPATH"), "src/code.cloudfoundry.org/bbs/cmd/bbs/fixtures")
		bbsURL.Scheme = "https"
	})

	Context("when configuring the BBS server for mutual SSL", func() {
		JustBeforeEach(func() {
			client = bbs.NewClient(bbsURL.String())
			bbsRunner = testrunner.New(bbsBinPath, bbsConfig)
			bbsProcess = ginkgomon.Invoke(bbsRunner)
		})

		BeforeEach(func() {
			bbsConfig.RequireSSL = true
			bbsConfig.CaFile = path.Join(basePath, "green-certs", "server-ca.crt")
			bbsConfig.CertFile = path.Join(basePath, "green-certs", "server.crt")
			bbsConfig.KeyFile = path.Join(basePath, "green-certs", "server.key")
		})

		It("succeeds for a client configured with the right certificate", func() {
			caFile := path.Join(basePath, "green-certs", "server-ca.crt")
			certFile := path.Join(basePath, "green-certs", "client.crt")
			keyFile := path.Join(basePath, "green-certs", "client.key")
			client, err = bbs.NewSecureClient(bbsURL.String(), caFile, certFile, keyFile, 0, 0)
			Expect(err).NotTo(HaveOccurred())
			Expect(client.Ping(logger)).To(BeTrue())
		})

		It("fails for a client with no SSL", func() {
			client = bbs.NewClient(bbsURL.String())
			Expect(client.Ping(logger)).To(BeFalse())
		})

		It("fails for a client configured with the wrong certificates", func() {
			caFile := path.Join(basePath, "green-certs", "server-ca.crt")
			certFile := path.Join(basePath, "blue-certs", "client.crt")
			keyFile := path.Join(basePath, "blue-certs", "client.key")
			client, err = bbs.NewSecureClient(bbsURL.String(), caFile, certFile, keyFile, 0, 0)
			Expect(err).NotTo(HaveOccurred())
			Expect(client.Ping(logger)).To(BeFalse())
		})

		It("fails for a client configured with a different ca certificate", func() {
			caFile := path.Join(basePath, "blue-certs", "server-ca.crt")
			certFile := path.Join(basePath, "green-certs", "client.crt")
			keyFile := path.Join(basePath, "green-certs", "client.key")
			client, err = bbs.NewSecureClient(bbsURL.String(), caFile, certFile, keyFile, 0, 0)
			Expect(err).NotTo(HaveOccurred())
			Expect(client.Ping(logger)).To(BeFalse())
		})

		It("fails to create the client if certs are not valid", func() {
			client, err = bbs.NewSecureClient(bbsURL.String(), "", "", "", 0, 0)
			Expect(err).To(HaveOccurred())
		})

		Context("task callbacks", func() {
			var (
				caFile, certFile, keyFile string
				tlsServer, insecureServer *ghttp.Server
				doneChan                  chan struct{}
			)

			BeforeEach(func() {
				doneChan = make(chan struct{})
				caFile = path.Join(basePath, "green-certs", "server-ca.crt")
				certFile = path.Join(basePath, "green-certs", "client.crt")
				keyFile = path.Join(basePath, "green-certs", "client.key")

				tlsServer = ghttp.NewUnstartedServer()
				insecureServer = ghttp.NewUnstartedServer()

				tlsConfig, err := cfhttp.NewTLSConfig(certFile, keyFile, caFile)
				Expect(err).NotTo(HaveOccurred())

				tlsServer.HTTPTestServer.TLS = tlsConfig

				handlers := []http.HandlerFunc{
					ghttp.VerifyRequest("POST", "/test"),
					ghttp.RespondWith(200, nil),
					func(w http.ResponseWriter, request *http.Request) {
						close(doneChan)
					},
				}

				tlsServer.RouteToHandler("POST", "/test", ghttp.CombineHandlers(handlers...))
				insecureServer.RouteToHandler("POST", "/test", ghttp.CombineHandlers(handlers...))

				insecureServer.Start()
				tlsServer.HTTPTestServer.StartTLS()
			})

			It("uses the tls configuration for task callbacks with https", func() {
				client, err = bbs.NewSecureClient(bbsURL.String(), caFile, certFile, keyFile, 0, 0)
				Expect(err).NotTo(HaveOccurred())

				taskDef := model_helpers.NewValidTaskDefinition()
				taskDef.CompletionCallbackUrl = tlsServer.URL() + "/test"

				err := client.DesireTask(logger, "task-guid", "domain", taskDef)
				Expect(err).NotTo(HaveOccurred())

				err = client.CancelTask(logger, "task-guid")
				Expect(err).NotTo(HaveOccurred())

				Eventually(doneChan).Should(BeClosed())
			})

			It("also works with http endpoints", func() {
				client, err = bbs.NewSecureClient(bbsURL.String(), caFile, certFile, keyFile, 0, 0)
				Expect(err).NotTo(HaveOccurred())

				taskDef := model_helpers.NewValidTaskDefinition()
				taskDef.CompletionCallbackUrl = insecureServer.URL() + "/test"

				err := client.DesireTask(logger, "task-guid", "domain", taskDef)
				Expect(err).NotTo(HaveOccurred())

				err = client.CancelTask(logger, "task-guid")
				Expect(err).NotTo(HaveOccurred())

				Eventually(doneChan).Should(BeClosed())
			})
		})
	})

	Context("when configuring a client without mutual SSL (skipping verification)", func() {
		JustBeforeEach(func() {
			client = bbs.NewClient(bbsURL.String())
			bbsRunner = testrunner.New(bbsBinPath, bbsConfig)
			bbsProcess = ginkgomon.Invoke(bbsRunner)
		})

		BeforeEach(func() {
			bbsConfig.RequireSSL = true
			bbsConfig.CertFile = path.Join(basePath, "green-certs", "server.crt")
			bbsConfig.KeyFile = path.Join(basePath, "green-certs", "server.key")
		})

		It("succeeds for a client configured with the right certificate", func() {
			certFile := path.Join(basePath, "green-certs", "client.crt")
			keyFile := path.Join(basePath, "green-certs", "client.key")
			client, err = bbs.NewSecureSkipVerifyClient(bbsURL.String(), certFile, keyFile, 0, 0)
			Expect(err).NotTo(HaveOccurred())
		})

		It("fails for a client configured with the wrong certificates", func() {
			certFile := path.Join(basePath, "blue-certs", "client.crt")
			keyFile := path.Join(basePath, "blue-certs", "client.key")
			client, err = bbs.NewSecureSkipVerifyClient(bbsURL.String(), certFile, keyFile, 0, 0)
			Expect(err).NotTo(HaveOccurred())
			Expect(client.Ping(logger)).To(BeFalse())
		})
	})

	Context("when configuring the auctioneer client with mutual SSL", func() {
		BeforeEach(func() {
			bbsConfig.AuctioneerCACert = path.Join(basePath, "green-certs", "server-ca.crt")
			bbsConfig.AuctioneerClientCert = path.Join(basePath, "green-certs", "client.crt")
			bbsConfig.AuctioneerClientKey = path.Join(basePath, "green-certs", "client.key")
		})

		JustBeforeEach(func() {
			bbsRunner = testrunner.New(bbsBinPath, bbsConfig)
			bbsProcess = ifrit.Background(bbsRunner)
		})

		It("works", func() {
			Eventually(bbsProcess.Ready()).Should(BeClosed())
			Consistently(bbsProcess.Wait()).ShouldNot(Receive())
		})

		Context("when the auctioneer client is configured incorrectly", func() {
			BeforeEach(func() {
				bbsConfig.AuctioneerClientCert = ""
			})

			It("exits with an error", func() {
				Eventually(bbsProcess.Wait()).Should(Receive())
				Consistently(bbsProcess.Ready()).ShouldNot(BeClosed())
			})
		})
	})
})
