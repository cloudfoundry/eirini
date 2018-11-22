package main

import (
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"time"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/consuladapter"
	"code.cloudfoundry.org/debugserver"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerflags"
	"code.cloudfoundry.org/locket"
	"code.cloudfoundry.org/locket/lock"
	locketmodels "code.cloudfoundry.org/locket/models"
	"code.cloudfoundry.org/tps"
	"code.cloudfoundry.org/tps/cc_client"
	"code.cloudfoundry.org/tps/config"
	"code.cloudfoundry.org/tps/watcher"
	"github.com/cloudfoundry/dropsonde"
	"github.com/nu7hatch/gouuid"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/sigmon"
)

var configPath = flag.String(
	"configPath",
	"",
	"path to config",
)

const (
	dropsondeOrigin = "tps_watcher"
)

func main() {
	flag.Parse()

	watcherConfig, err := config.NewWatcherConfig(*configPath)
	if err != nil {
		panic(err.Error())
	}

	logger, reconfigurableSink := lagerflags.NewFromConfig("tps-watcher", watcherConfig.LagerConfig)

	initializeDropsonde(logger, watcherConfig.DropsondePort)

	locks := []grouper.Member{}
	if !watcherConfig.SkipConsulLock {
		locks = append(locks, grouper.Member{"consul-lock", initializeConsulLockMaintainer(logger, watcherConfig)})
	}

	if watcherConfig.LocketAddress != "" {
		locks = append(locks, grouper.Member{"sql-lock", initializeLocketLockMaintainer(logger, watcherConfig)})
	}

	if len(locks) < 1 {
		logger.Fatal("no-locks-configured", errors.New("Lock configuration must be provided"))
	}

	tlsConfig, err := cc_client.NewTLSConfig(
		watcherConfig.CCClientCert,
		watcherConfig.CCClientKey,
		watcherConfig.CCCACert,
	)
	if err != nil {
		panic(err.Error())
	}
	ccClient := cc_client.NewCcClient(watcherConfig.CCBaseUrl, tlsConfig)

	watcher := ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
		w, err := watcher.NewWatcher(logger,
			watcherConfig.MaxEventHandlingWorkers,
			watcher.DefaultRetryPauseInterval,
			initializeBBSClient(logger, watcherConfig), ccClient)

		if err != nil {
			return err
		}

		return w.Run(signals, ready)
	})

	members := append(locks, grouper.Member{"watcher", watcher})

	if dbgAddr := watcherConfig.DebugServerConfig.DebugAddress; dbgAddr != "" {
		members = append(grouper.Members{
			{"debug-server", debugserver.Runner(dbgAddr, reconfigurableSink)},
		}, members...)
	}

	group := grouper.NewOrdered(os.Interrupt, members)

	monitor := ifrit.Invoke(sigmon.New(group))

	logger.Info("started")

	err = <-monitor.Wait()
	if err != nil {
		logger.Error("exited-with-failure", err)
		os.Exit(1)
	}

	logger.Info("exited")
}

func initializeDropsonde(logger lager.Logger, dropsondePort int) {
	dropsondeDestination := fmt.Sprint("localhost:", dropsondePort)
	err := dropsonde.Initialize(dropsondeDestination, dropsondeOrigin)
	if err != nil {
		logger.Error("failed to initialize dropsonde: %v", err)
	}
}

func initializeServiceClient(logger lager.Logger, consulCluster string) tps.ServiceClient {
	consulClient, err := consuladapter.NewClientFromUrl(consulCluster)
	if err != nil {
		logger.Fatal("new-client-failed", err)
	}

	return tps.NewServiceClient(consulClient, clock.NewClock())
}

func initializeConsulLockMaintainer(logger lager.Logger, watcherConfig config.WatcherConfig) ifrit.Runner {
	serviceClient := initializeServiceClient(logger, watcherConfig.ConsulCluster)

	uuid, err := uuid.NewV4()
	if err != nil {
		logger.Fatal("Couldn't generate uuid", err)
	}

	return serviceClient.NewTPSWatcherLockRunner(logger, uuid.String(), time.Duration(watcherConfig.LockRetryInterval), time.Duration(watcherConfig.LockTTL))
}

func initializeLocketLockMaintainer(logger lager.Logger, watcherConfig config.WatcherConfig) ifrit.Runner {
	locketClient, err := locket.NewClient(logger, watcherConfig.ClientLocketConfig)
	if err != nil {
		logger.Fatal("Failed to initialize locket client", err)
	}

	owner := fmt.Sprintf("tps-watcher-%s", watcherConfig.InstanceID)

	lockIdentifier := &locketmodels.Resource{
		Key:      "tps_watcher",
		Owner:    owner,
		TypeCode: locketmodels.LOCK,
		Type:     locketmodels.LockType,
	}

	return lock.NewLockRunner(
		logger,
		locketClient,
		lockIdentifier,
		locket.DefaultSessionTTLInSeconds,
		clock.NewClock(),
		locket.SQLRetryInterval,
	)
}

func initializeBBSClient(logger lager.Logger, watcherConfig config.WatcherConfig) bbs.Client {
	bbsURL, err := url.Parse(watcherConfig.BBSAddress)
	if err != nil {
		logger.Fatal("Invalid BBS URL", err)
	}
	var bbsClient bbs.InternalClient

	if bbsURL.Scheme != "https" {
		var cfg bbs.ClientConfig
		cfg.URL = watcherConfig.BBSAddress
		cfg.IsTLS = false

		bbsClient, err = bbs.NewClientWithConfig(cfg)
		if err != nil {
			logger.Fatal("Failed to configure insecure BBS client", err)
		}

		return bbsClient
	}

	bbsClient, err = bbs.NewClient(
		watcherConfig.BBSAddress,
		watcherConfig.BBSCACert,
		watcherConfig.BBSClientCert,
		watcherConfig.BBSClientKey,
		watcherConfig.BBSClientSessionCacheSize,
		watcherConfig.BBSMaxIdleConnsPerHost,
	)
	if err != nil {
		logger.Fatal("Failed to configure secure BBS client", err)
	}
	return bbsClient
}
