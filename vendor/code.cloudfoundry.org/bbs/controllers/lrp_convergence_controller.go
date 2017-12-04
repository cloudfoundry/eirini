package controllers

import (
	"sync"

	"code.cloudfoundry.org/auctioneer"
	"code.cloudfoundry.org/bbs/db"
	"code.cloudfoundry.org/bbs/events"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/serviceclient"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/workpool"
)

//go:generate counterfeiter -o fakes/fake_retirer.go . Retirer
type Retirer interface {
	RetireActualLRP(logger lager.Logger, key *models.ActualLRPKey) error
}

type LRPConvergenceController struct {
	logger                 lager.Logger
	db                     db.LRPDB
	actualHub              events.Hub
	auctioneerClient       auctioneer.Client
	serviceClient          serviceclient.ServiceClient
	retirer                Retirer
	convergenceWorkersSize int
}

func NewLRPConvergenceController(
	logger lager.Logger,
	db db.LRPDB,
	actualHub events.Hub,
	auctioneerClient auctioneer.Client,
	serviceClient serviceclient.ServiceClient,
	retirer Retirer,
	convergenceWorkersSize int,
) *LRPConvergenceController {
	return &LRPConvergenceController{
		logger:                 logger,
		db:                     db,
		actualHub:              actualHub,
		auctioneerClient:       auctioneerClient,
		serviceClient:          serviceClient,
		retirer:                retirer,
		convergenceWorkersSize: convergenceWorkersSize,
	}
}

func (h *LRPConvergenceController) ConvergeLRPs(logger lager.Logger) error {
	logger = h.logger.Session("converge-lrps")
	var err error

	logger.Debug("listing-cells")
	var cellSet models.CellSet
	cellSet, err = h.serviceClient.Cells(logger)
	if err == models.ErrResourceNotFound {
		logger.Info("no-cells-found")
		cellSet = models.CellSet{}
	} else if err != nil {
		logger.Error("failed-listing-cells", err)
		// convergence should run again later
		return nil
	}
	logger.Debug("succeeded-listing-cells")

	startRequests, keysWithMissingCells, keysToRetire, events := h.db.ConvergeLRPs(logger, cellSet)

	for _, e := range events {
		go h.actualHub.Emit(e)
	}
	retireLogger := logger.WithData(lager.Data{"retiring_lrp_count": len(keysToRetire)})
	works := []func(){}
	for _, key := range keysToRetire {
		key := key
		works = append(works, func() {
			err := h.retirer.RetireActualLRP(retireLogger, key)
			if err != nil {
				logger.Error("retiring-lrp-failed", err)
			}
		})
	}

	errChan := make(chan *models.Error, 1)

	startRequestLock := &sync.Mutex{}
	for _, key := range keysWithMissingCells {
		key := key
		works = append(works, func() {
			before, after, err := h.db.UnclaimActualLRP(logger, key.Key)
			if err == nil {
				h.actualHub.Emit(models.NewActualLRPChangedEvent(before, after))
				startRequest := auctioneer.NewLRPStartRequestFromSchedulingInfo(key.SchedulingInfo, int(key.Key.Index))
				startRequestLock.Lock()
				startRequests = append(startRequests, &startRequest)
				startRequestLock.Unlock()
				logger.Info("creating-start-request",
					lager.Data{"reason": "missing-cell", "process_guid": key.Key.ProcessGuid, "index": key.Key.Index})
			} else {
				bbsErr := models.ConvertError(err)
				if bbsErr.GetType() != models.Error_Unrecoverable {
					return
				}

				logger.Error("unrecoverable-error", bbsErr)
				select {
				case errChan <- bbsErr:
				default:
				}
			}
		})
	}

	var throttler *workpool.Throttler
	throttler, err = workpool.NewThrottler(h.convergenceWorkersSize, works)
	if err != nil {
		logger.Error("failed-constructing-throttler", err, lager.Data{"max_workers": h.convergenceWorkersSize, "num_works": len(works)})
		return nil
	}

	retireLogger.Debug("retiring-actual-lrps")
	throttler.Work()
	retireLogger.Debug("done-retiring-actual-lrps")

	select {
	case err := <-errChan:
		return err
	default:
	}

	startLogger := logger.WithData(lager.Data{"start_requests_count": len(startRequests)})
	if len(startRequests) > 0 {
		startLogger.Debug("requesting-start-auctions")
		err = h.auctioneerClient.RequestLRPAuctions(logger, startRequests)
		if err != nil {
			startLogger.Error("failed-to-request-starts", err, lager.Data{"lrp_start_auctions": startRequests})
		}
		startLogger.Debug("done-requesting-start-auctions")
	}

	return nil
}
