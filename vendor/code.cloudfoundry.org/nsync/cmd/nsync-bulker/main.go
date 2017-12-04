package main

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"time"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/cfhttp"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/consuladapter"
	"code.cloudfoundry.org/debugserver"
	"code.cloudfoundry.org/diego-ssh/keys"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerflags"
	"code.cloudfoundry.org/runtimeschema/cc_messages/flags"
	"github.com/cloudfoundry/dropsonde"
	"github.com/nu7hatch/gouuid"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/sigmon"

	"code.cloudfoundry.org/nsync"
	"code.cloudfoundry.org/nsync/bulk"
	"code.cloudfoundry.org/nsync/config"
	"code.cloudfoundry.org/nsync/recipebuilder"
)

var configPath = flag.String(
	"configPath",
	"",
	"path to config",
)

const (
	dropsondeOrigin = "nsync_bulker"
)

func main() {
	flag.Parse()

	bulkerConfig, err := config.NewBulkerConfig(*configPath)
	if err != nil {
		panic(err.Error())
	}
	lifecycles := flags.LifecycleMap{}
	for _, value := range bulkerConfig.Lifecycles {
		lifecycles.Set(value)
	}

	logger, reconfigurableSink := lagerflags.NewFromConfig("nsync-bulker", bulkerConfig.LagerConfig)

	initializeDropsonde(logger, bulkerConfig)
	cfhttp.Initialize(time.Duration(bulkerConfig.CommunicationTimeout))

	serviceClient := initializeServiceClient(logger, bulkerConfig)
	uuid, err := uuid.NewV4()
	if err != nil {
		logger.Fatal("Couldn't generate uuid", err)
	}
	lockMaintainer := serviceClient.NewNsyncBulkerLockRunner(logger, uuid.String(), time.Duration(bulkerConfig.LockRetryInterval), time.Duration(bulkerConfig.LockTTL))

	dockerRecipeBuilderConfig := recipebuilder.Config{
		Lifecycles:    lifecycles,
		FileServerURL: bulkerConfig.FileServerUrl,
		KeyFactory:    keys.RSAKeyPairFactory,
	}

	buildpackRecipeBuilderConfig := recipebuilder.Config{
		Lifecycles:           lifecycles,
		FileServerURL:        bulkerConfig.FileServerUrl,
		KeyFactory:           keys.RSAKeyPairFactory,
		PrivilegedContainers: bulkerConfig.PrivilegedContainers,
	}

	recipeBuilders := map[string]recipebuilder.RecipeBuilder{
		"buildpack": recipebuilder.NewBuildpackRecipeBuilder(logger, buildpackRecipeBuilderConfig),
		"docker":    recipebuilder.NewDockerRecipeBuilder(logger, dockerRecipeBuilderConfig),
	}

	lrpRunner := bulk.NewLRPProcessor(
		logger,
		initializeBBSClient(logger, bulkerConfig),
		time.Duration(bulkerConfig.CCPollingInterval),
		time.Duration(bulkerConfig.DomainTTL),
		bulkerConfig.CCBulkBatchSize,
		bulkerConfig.BBSUpdateLRPWorkers,
		bulkerConfig.SkipCertVerify,
		&bulk.CCFetcher{
			BaseURI:   bulkerConfig.CCBaseUrl,
			BatchSize: int(bulkerConfig.CCBulkBatchSize),
			Username:  bulkerConfig.CCUsername,
			Password:  bulkerConfig.CCPassword,
		},
		recipeBuilders,
		clock.NewClock(),
	)

	taskRunner := bulk.NewTaskProcessor(
		logger,
		initializeBBSClient(logger, bulkerConfig),
		&bulk.CCTaskClient{},
		time.Duration(bulkerConfig.CCPollingInterval),
		time.Duration(bulkerConfig.DomainTTL),
		bulkerConfig.BBSFailTaskPoolSize,
		bulkerConfig.BBSCancelTaskPoolSize,
		bulkerConfig.SkipCertVerify,
		&bulk.CCFetcher{
			BaseURI:   bulkerConfig.CCBaseUrl,
			BatchSize: int(bulkerConfig.CCBulkBatchSize),
			Username:  bulkerConfig.CCUsername,
			Password:  bulkerConfig.CCPassword,
		},
		clock.NewClock(),
	)

	members := grouper.Members{
		{"lock-maintainer", lockMaintainer},
		{"lrp-runner", lrpRunner},
		{"task-runner", taskRunner},
	}

	if dbgAddr := bulkerConfig.DebugServerConfig.DebugAddress; dbgAddr != "" {
		members = append(grouper.Members{
			{"debug-server", debugserver.Runner(dbgAddr, reconfigurableSink)},
		}, members...)
	}

	group := grouper.NewOrdered(os.Interrupt, members)

	logger.Info("waiting-for-lock")

	monitor := ifrit.Invoke(sigmon.New(group))

	logger.Info("started")

	err = <-monitor.Wait()
	if err != nil {
		logger.Error("exited-with-failure", err)
		os.Exit(1)
	}

	logger.Info("exited")
	os.Exit(0)
}

func initializeDropsonde(logger lager.Logger, bulkerConfig config.BulkerConfig) {
	dropsondeDestination := fmt.Sprint("localhost:", bulkerConfig.DropsondePort)
	err := dropsonde.Initialize(dropsondeDestination, dropsondeOrigin)
	if err != nil {
		logger.Error("failed to initialize dropsonde: %v", err)
	}
}

func initializeServiceClient(logger lager.Logger, bulkerConfig config.BulkerConfig) nsync.ServiceClient {
	consulClient, err := consuladapter.NewClientFromUrl(bulkerConfig.ConsulCluster)
	if err != nil {
		logger.Fatal("new-client-failed", err)
	}

	return nsync.NewServiceClient(consulClient, clock.NewClock())
}

func initializeBBSClient(logger lager.Logger, bulkerConfig config.BulkerConfig) bbs.Client {
	bbsURL, err := url.Parse(bulkerConfig.BBSAddress)
	if err != nil {
		logger.Fatal("Invalid BBS URL", err)
	}

	if bbsURL.Scheme != "https" {
		return bbs.NewClient(bulkerConfig.BBSAddress)
	}

	bbsClient, err := bbs.NewSecureClient(bulkerConfig.BBSAddress, bulkerConfig.BBSCACert, bulkerConfig.BBSClientCert, bulkerConfig.BBSClientKey, bulkerConfig.BBSClientSessionCacheSize, bulkerConfig.BBSMaxIdleConnsPerHost)
	if err != nil {
		logger.Fatal("Failed to configure secure BBS client", err)
	}
	return bbsClient
}
