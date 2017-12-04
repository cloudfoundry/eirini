package config_test

import (
	"time"

	"code.cloudfoundry.org/locket"
	. "code.cloudfoundry.org/nsync/config"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {
	Context("Bulker config", func() {
		It("generates a config with the default values", func() {
			bulkerConfig, err := NewBulkerConfig("../fixtures/empty_config.json")
			Expect(err).ToNot(HaveOccurred())

			Expect(bulkerConfig.BBSCancelTaskPoolSize).To(Equal(50))
			Expect(bulkerConfig.BBSClientSessionCacheSize).To(Equal(0))
			Expect(bulkerConfig.BBSFailTaskPoolSize).To(Equal(50))
			Expect(bulkerConfig.BBSMaxIdleConnsPerHost).To(Equal(0))
			Expect(bulkerConfig.BBSUpdateLRPWorkers).To(Equal(50))
			Expect(bulkerConfig.CCBulkBatchSize).To(Equal(uint(500)))
			Expect(bulkerConfig.CCPollingInterval).To(Equal(Duration(30 * time.Second)))
			Expect(bulkerConfig.CommunicationTimeout).To(Equal(Duration(30 * time.Second)))
			Expect(bulkerConfig.DomainTTL).To(Equal(Duration(2 * time.Minute)))
			Expect(bulkerConfig.DropsondePort).To(Equal(3457))
			Expect(bulkerConfig.LagerConfig.LogLevel).To(Equal("info"))
			Expect(bulkerConfig.LockRetryInterval).To(Equal(Duration(locket.RetryInterval)))
			Expect(bulkerConfig.LockTTL).To(Equal(Duration(locket.DefaultSessionTTL)))
			Expect(bulkerConfig.PrivilegedContainers).To(Equal(false))
			Expect(bulkerConfig.SkipCertVerify).To(Equal(false))
		})

		It("reads from the config file and populates the config", func() {
			bulkerConfig, err := NewBulkerConfig("../fixtures/bulker_config.json")
			Expect(err).ToNot(HaveOccurred())

			Expect(bulkerConfig.BBSAddress).To(Equal("https://foobar.com"))
			Expect(bulkerConfig.BBSCancelTaskPoolSize).To(Equal(1234))
			Expect(bulkerConfig.CCBulkBatchSize).To(Equal(uint(117)))
			Expect(bulkerConfig.CCPollingInterval).To(Equal(Duration(120 * time.Second)))
			Expect(bulkerConfig.LagerConfig.LogLevel).To(Equal("debug"))
			Expect(bulkerConfig.Lifecycles).To(Equal([]string{
				"buildpack/cflinuxfs2:/path/to/bundle",
				"buildpack/cflinuxfs2:/path/to/another/bundle",
				"buildpack/somethingelse:/path/to/third/bundle",
			}))
			Expect(bulkerConfig.SkipCertVerify).To(BeTrue())
			Expect(bulkerConfig.DebugServerConfig.DebugAddress).To(Equal("https://debugger.com"))
		})
	})

	Context("Listener config", func() {
		It("generates a config with the default values", func() {
			listenerConfig, err := NewListenerConfig("../fixtures/empty_config.json")
			Expect(err).ToNot(HaveOccurred())

			Expect(listenerConfig.BBSClientSessionCacheSize).To(Equal(0))
			Expect(listenerConfig.BBSMaxIdleConnsPerHost).To(Equal(0))
			Expect(listenerConfig.CommunicationTimeout).To(Equal(Duration(30 * time.Second)))
			Expect(listenerConfig.DropsondePort).To(Equal(3457))
			Expect(listenerConfig.LagerConfig.LogLevel).To(Equal("info"))
			Expect(listenerConfig.PrivilegedContainers).To(Equal(false))
		})

		It("reads from the config file and populates the config", func() {
			listenerConfig, err := NewListenerConfig("../fixtures/listener_config.json")
			Expect(err).ToNot(HaveOccurred())

			Expect(listenerConfig.BBSAddress).To(Equal("https://foobar.com"))
			Expect(listenerConfig.BBSCACert).To(Equal("/path/to/cert"))
			Expect(listenerConfig.BBSClientCert).To(Equal("/path/to/another/cert"))
			Expect(listenerConfig.BBSClientKey).To(Equal("/path/to/key"))
			Expect(listenerConfig.BBSClientSessionCacheSize).To(Equal(1234))
			Expect(listenerConfig.BBSMaxIdleConnsPerHost).To(Equal(10))
			Expect(listenerConfig.CommunicationTimeout).To(Equal(Duration(256 * time.Second)))
			Expect(listenerConfig.ConsulCluster).To(Equal("https://consul.com"))
			Expect(listenerConfig.DebugServerConfig.DebugAddress).To(Equal("https://debugger.com"))
			Expect(listenerConfig.DropsondePort).To(Equal(666))
			Expect(listenerConfig.FileServerURL).To(Equal("https://fileserver.com"))
			Expect(listenerConfig.Lifecycles).To(Equal([]string{
				"buildpack/cflinuxfs2:/path/to/bundle",
				"buildpack/cflinuxfs2:/path/to/another/bundle",
				"buildpack/somethingelse:/path/to/third/bundle",
			}))
			Expect(listenerConfig.ListenAddress).To(Equal("https://nsync.com/listen"))
			Expect(listenerConfig.LagerConfig.LogLevel).To(Equal("debug"))
			Expect(listenerConfig.PrivilegedContainers).To(Equal(true))
		})
	})
})
