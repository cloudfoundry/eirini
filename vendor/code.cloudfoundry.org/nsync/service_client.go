package nsync

import (
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/consuladapter"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/locket"
	"github.com/tedsuo/ifrit"
)

const NysncBulkerLockSchemaKey = "nsync_bulker_lock"

func NysncBulkerLockSchemaPath() string {
	return locket.LockSchemaPath(NysncBulkerLockSchemaKey)
}

type ServiceClient interface {
	NewNsyncBulkerLockRunner(logger lager.Logger, bulkerID string, retryInterval, lockTTL time.Duration) ifrit.Runner
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

func (c serviceClient) NewNsyncBulkerLockRunner(logger lager.Logger, bulkerID string, retryInterval, lockTTL time.Duration) ifrit.Runner {
	return locket.NewLock(logger, c.consulClient, NysncBulkerLockSchemaPath(), []byte(bulkerID), c.clock, retryInterval, lockTTL)
}
