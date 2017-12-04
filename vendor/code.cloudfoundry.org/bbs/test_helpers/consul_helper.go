package test_helpers

import (
	"encoding/json"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/consuladapter"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/locket"
	"code.cloudfoundry.org/rep/maintain"
	"github.com/tedsuo/ifrit"

	. "github.com/onsi/gomega"
)

type ConsulHelper struct {
	consulClient consuladapter.Client
	logger       lager.Logger
}

func NewConsulHelper(logger lager.Logger, consulClient consuladapter.Client) *ConsulHelper {
	return &ConsulHelper{
		logger:       logger,
		consulClient: consulClient,
	}
}

func (t *ConsulHelper) RegisterCell(cell *models.CellPresence) {
	var err error
	jsonBytes, err := json.Marshal(cell)
	Expect(err).NotTo(HaveOccurred())

	// Use NewLock instead of NewPresence in order to block on the cell being registered
	runner := locket.NewLock(t.logger, t.consulClient, maintain.CellSchemaPath(cell.CellId), jsonBytes, clock.NewClock(), locket.RetryInterval, locket.DefaultSessionTTL)
	ifrit.Invoke(runner)

	Expect(err).NotTo(HaveOccurred())
}
