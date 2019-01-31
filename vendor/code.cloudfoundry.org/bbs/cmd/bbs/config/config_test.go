package config_test

import (
	"io/ioutil"
	"os"
	"time"

	"code.cloudfoundry.org/bbs/cmd/bbs/config"
	"code.cloudfoundry.org/bbs/encryption"
	"code.cloudfoundry.org/bbs/test_helpers"
	"code.cloudfoundry.org/debugserver"
	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/durationjson"
	"code.cloudfoundry.org/lager/lagerflags"
	"code.cloudfoundry.org/locket"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("BBSConfig", func() {
	var configFilePath, configData string

	BeforeEach(func() {
		configData = `{
			"access_log_path": "/var/vcap/sys/log/bbs/access.log",
			"active_key_label": "label",
			"advertise_url": "bbs.service.cf.internal",
			"auctioneer_address": "https://auctioneer.service.cf.internal:9016",
			"auctioneer_ca_cert": "/var/vcap/jobs/bbs/config/auctioneer.ca",
			"auctioneer_client_cert": "/var/vcap/jobs/bbs/config/auctioneer.crt",
			"auctioneer_client_key": "/var/vcap/jobs/bbs/config/auctioneer.key",
			"auctioneer_require_tls": true,
			"uuid": "bosh-boshy-bosh-bosh",
			"ca_file": "/var/vcap/jobs/bbs/config/ca.crt",
			"cell_registrations_locket_enabled": true,
			"cert_file": "/var/vcap/jobs/bbs/config/bbs.crt",
			"communication_timeout": "20s",
			"consul_cluster": "",
			"converge_repeat_interval": "30s",
			"convergence_workers": 20,
			"database_connection_string": "",
			"database_driver": "postgres",
			"debug_address": "127.0.0.1:17017",
			"desired_lrp_creation_timeout": "1m0s",
			"detect_consul_cell_registrations": true,
			"enable_consul_service_registration": false,
			"encryption_keys": {"label": "key"},
			"expire_completed_task_duration": "2m0s",
			"expire_pending_task_duration": "30m0s",
			"health_address": "127.0.0.1:8890",
			"key_file": "/var/vcap/jobs/bbs/config/bbs.key",
			"kick_task_duration": "30s",
			"listen_address": "0.0.0.0:8889",
			"lock_retry_interval": "5s",
			"lock_ttl": "15s",
			"locks_locket_enabled": true,
			"locket_address": "127.0.0.1:18018",
			"locket_ca_cert_file": "locket-ca-cert",
			"locket_client_cert_file": "locket-client-cert",
			"locket_client_key_file": "locket-client-key",
			"log_level": "debug",
      "loggregator": {
        "loggregator_use_v2_api": true,
        "loggregator_api_port": 1234,
        "loggregator_ca_path": "ca-path",
        "loggregator_cert_path": "cert-path",
        "loggregator_key_path": "key-path",
        "loggregator_job_deployment": "job-deployment",
        "loggregator_job_name": "job-name",
        "loggregator_job_index": "job-index",
        "loggregator_job_ip": "job-ip",
        "loggregator_job_origin": "job-origin"
      },
			"max_idle_database_connections": 50,
			"max_open_database_connections": 200,
			"rep_ca_cert": "/var/vcap/jobs/bbs/config/rep.ca",
			"rep_client_cert": "/var/vcap/jobs/bbs/config/rep.crt",
			"rep_client_key": "/var/vcap/jobs/bbs/config/rep.key",
			"rep_client_session_cache_size": 10,
			"rep_require_tls": true,
			"report_interval": "1m0s",
			"require_ssl": true,
			"session_name": "bbs-session",
			"skip_consul_lock": true,
			"sql_ca_cert_file": "/var/vcap/jobs/bbs/config/sql.ca",
			"sql_enable_identity_verification": true,
			"task_callback_workers": 1000,
			"update_workers": 1000,
			"max_task_retries": 3
		}`
	})

	JustBeforeEach(func() {
		configFile, err := ioutil.TempFile("", "config-file")
		Expect(err).NotTo(HaveOccurred())

		n, err := configFile.WriteString(configData)
		Expect(err).NotTo(HaveOccurred())
		Expect(n).To(Equal(len(configData)))

		configFilePath = configFile.Name()
	})

	AfterEach(func() {
		err := os.RemoveAll(configFilePath)
		Expect(err).NotTo(HaveOccurred())
	})

	It("correctly parses the config file", func() {
		bbsConfig, err := config.NewBBSConfig(configFilePath)
		Expect(err).NotTo(HaveOccurred())

		config := config.BBSConfig{
			AccessLogPath:                  "/var/vcap/sys/log/bbs/access.log",
			AdvertiseURL:                   "bbs.service.cf.internal",
			AuctioneerAddress:              "https://auctioneer.service.cf.internal:9016",
			AuctioneerCACert:               "/var/vcap/jobs/bbs/config/auctioneer.ca",
			AuctioneerClientCert:           "/var/vcap/jobs/bbs/config/auctioneer.crt",
			AuctioneerClientKey:            "/var/vcap/jobs/bbs/config/auctioneer.key",
			AuctioneerRequireTLS:           true,
			UUID:                           "bosh-boshy-bosh-bosh",
			CaFile:                         "/var/vcap/jobs/bbs/config/ca.crt",
			CellRegistrationsLocketEnabled: true,
			CertFile:                       "/var/vcap/jobs/bbs/config/bbs.crt",
			LocksLocketEnabled:             true,
			ClientLocketConfig: locket.ClientLocketConfig{
				LocketAddress:        "127.0.0.1:18018",
				LocketCACertFile:     "locket-ca-cert",
				LocketClientCertFile: "locket-client-cert",
				LocketClientKeyFile:  "locket-client-key",
			},
			CommunicationTimeout:   durationjson.Duration(20 * time.Second),
			ConvergeRepeatInterval: durationjson.Duration(30 * time.Second),
			ConvergenceWorkers:     20,
			DatabaseDriver:         "postgres",
			DebugServerConfig: debugserver.DebugServerConfig{
				DebugAddress: "127.0.0.1:17017",
			},
			DesiredLRPCreationTimeout:       durationjson.Duration(1 * time.Minute),
			DetectConsulCellRegistrations:   true,
			EnableConsulServiceRegistration: false,
			EncryptionConfig: encryption.EncryptionConfig{
				ActiveKeyLabel: "label",
				EncryptionKeys: map[string]string{
					"label": "key",
				},
			},
			ExpireCompletedTaskDuration: durationjson.Duration(2 * time.Minute),
			ExpirePendingTaskDuration:   durationjson.Duration(30 * time.Minute),
			HealthAddress:               "127.0.0.1:8890",
			KeyFile:                     "/var/vcap/jobs/bbs/config/bbs.key",
			KickTaskDuration:            durationjson.Duration(30 * time.Second),
			LagerConfig: lagerflags.LagerConfig{
				LogLevel: "debug",
			},
			LoggregatorConfig: loggingclient.Config{
				UseV2API:      true,
				APIPort:       1234,
				CACertPath:    "ca-path",
				CertPath:      "cert-path",
				KeyPath:       "key-path",
				JobDeployment: "job-deployment",
				JobName:       "job-name",
				JobIndex:      "job-index",
				JobIP:         "job-ip",
				JobOrigin:     "job-origin",
			},
			ListenAddress:                 "0.0.0.0:8889",
			LockRetryInterval:             durationjson.Duration(locket.RetryInterval),
			LockTTL:                       durationjson.Duration(locket.DefaultSessionTTL),
			MaxIdleDatabaseConnections:    50,
			MaxOpenDatabaseConnections:    200,
			RepCACert:                     "/var/vcap/jobs/bbs/config/rep.ca",
			RepClientCert:                 "/var/vcap/jobs/bbs/config/rep.crt",
			RepClientKey:                  "/var/vcap/jobs/bbs/config/rep.key",
			RepClientSessionCacheSize:     10,
			RepRequireTLS:                 true,
			ReportInterval:                durationjson.Duration(1 * time.Minute),
			RequireSSL:                    true,
			SQLCACertFile:                 "/var/vcap/jobs/bbs/config/sql.ca",
			SQLEnableIdentityVerification: true,
			SessionName:                   "bbs-session",
			TaskCallbackWorkers:           1000,
			UpdateWorkers:                 1000,
			SkipConsulLock:                true,
			MaxTaskRetries:                3,
		}

		Expect(bbsConfig).To(test_helpers.DeepEqual(config))
	})

	Context("when the file does not exist", func() {
		It("returns an error", func() {
			_, err := config.NewBBSConfig("foobar")
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when the file does not contain valid json", func() {
		BeforeEach(func() {
			configData = "{{"
		})

		It("returns an error", func() {
			_, err := config.NewBBSConfig(configFilePath)
			Expect(err).To(HaveOccurred())
		})

	})

	Context("when the file contains invalid durations", func() {
		BeforeEach(func() {
			configData = `{"expire_completed_task_duration": "4234342342"}`
		})

		It("returns an error", func() {
			_, err := config.NewBBSConfig(configFilePath)
			Expect(err).To(MatchError(ContainSubstring("missing unit")))
		})
	})
})
