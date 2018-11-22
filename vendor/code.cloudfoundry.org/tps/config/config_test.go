package config_test

import (
	"time"

	. "code.cloudfoundry.org/tps/config"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {
	Context("Watcher config", func() {
		It("generates a config with the default values", func() {
			watcherConfig, err := NewWatcherConfig("../fixtures/empty_config.json")
			Expect(err).ToNot(HaveOccurred())

			Expect(watcherConfig.BBSClientSessionCacheSize).To(Equal(0))
			Expect(watcherConfig.BBSMaxIdleConnsPerHost).To(Equal(0))
			Expect(watcherConfig.DropsondePort).To(Equal(3457))
			Expect(watcherConfig.LagerConfig.LogLevel).To(Equal("info"))
			Expect(watcherConfig.MaxEventHandlingWorkers).To(Equal(500))
			Expect(watcherConfig.SkipConsulLock).To(Equal(false))
		})

		It("reads from the config file and populates the config", func() {
			watcherConfig, err := NewWatcherConfig("../fixtures/watcher_config.json")
			Expect(err).ToNot(HaveOccurred())

			Expect(watcherConfig.BBSAddress).To(Equal("https://foobar.com"))
			Expect(watcherConfig.BBSCACert).To(Equal("/path/to/cert"))
			Expect(watcherConfig.BBSClientCert).To(Equal("/path/to/another/cert"))
			Expect(watcherConfig.BBSClientKey).To(Equal("/path/to/key"))
			Expect(watcherConfig.BBSClientSessionCacheSize).To(Equal(1234))
			Expect(watcherConfig.BBSMaxIdleConnsPerHost).To(Equal(10))
			Expect(watcherConfig.ConsulCluster).To(Equal("https://consul.com"))
			Expect(watcherConfig.CCBaseUrl).To(Equal("https://cloudcontroller.com"))
			Expect(watcherConfig.ConsulCluster).To(Equal("https://consul.com"))
			Expect(watcherConfig.DebugServerConfig.DebugAddress).To(Equal("https://debugger.com"))
			Expect(watcherConfig.DropsondePort).To(Equal(666))
			Expect(watcherConfig.LagerConfig.LogLevel).To(Equal("debug"))
			Expect(watcherConfig.LockRetryInterval).To(Equal(Duration(100 * time.Second)))
			Expect(watcherConfig.LockTTL).To(Equal(Duration(200 * time.Second)))
			Expect(watcherConfig.MaxEventHandlingWorkers).To(Equal(33))
			Expect(watcherConfig.CCClientCert).To(Equal("/path/to/server.cert"))
			Expect(watcherConfig.CCClientKey).To(Equal("/path/to/server.key"))
			Expect(watcherConfig.CCCACert).To(Equal("/path/to/server-ca.cert"))
			Expect(watcherConfig.SkipConsulLock).To(Equal(true))
			Expect(watcherConfig.LocketAddress).To(Equal("https://locket.com"))
			Expect(watcherConfig.LocketCACertFile).To(Equal("/path/to/locket/ca-cert"))
			Expect(watcherConfig.LocketClientCertFile).To(Equal("/path/to/locket/cert"))
			Expect(watcherConfig.LocketClientKeyFile).To(Equal("/path/to/locket/key"))
			Expect(watcherConfig.InstanceID).To(Equal("long-bosh-guid"))
		})
	})
})
