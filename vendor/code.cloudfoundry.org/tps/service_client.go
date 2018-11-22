package tps

import (
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/consuladapter"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/locket"
	"github.com/tedsuo/ifrit"
)

const TPSWatcherLockSchemaKey = "tps_watcher_lock"

func TPSWatcherLockSchemaPath() string {
	return locket.LockSchemaPath(TPSWatcherLockSchemaKey)
}

type ServiceClient interface {
	NewTPSWatcherLockRunner(logger lager.Logger, bulkerID string, retryInterval, lockTTL time.Duration) ifrit.Runner
}

type serviceClient struct {
	consulClient consuladapter.Client
	clock        clock.Clock
}

func NewServiceClient(consulClient consuladapter.Client, clock clock.Clock) ServiceClient {
	return serviceClient{
		consulClient: consulClient,
		clock:        clock,
	}
}

func (c serviceClient) NewTPSWatcherLockRunner(logger lager.Logger, emitterID string, retryInterval, lockTTL time.Duration) ifrit.Runner {
	return locket.NewLock(logger, c.consulClient, TPSWatcherLockSchemaPath(), []byte(emitterID), c.clock, retryInterval, lockTTL)
}
