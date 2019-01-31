package controllers

import (
	"code.cloudfoundry.org/auctioneer"
	"code.cloudfoundry.org/bbs/db"
	"code.cloudfoundry.org/bbs/events"
	"code.cloudfoundry.org/bbs/events/calculator"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/serviceclient"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/rep"
)

type ActualLRPLifecycleController struct {
	db                   db.ActualLRPDB
	suspectDB            db.SuspectDB
	evacuationDB         db.EvacuationDB
	desiredLRPDB         db.DesiredLRPDB
	auctioneerClient     auctioneer.Client
	serviceClient        serviceclient.ServiceClient
	repClientFactory     rep.ClientFactory
	actualHub            events.Hub
	actualLRPInstanceHub events.Hub
}

func NewActualLRPLifecycleController(
	db db.ActualLRPDB,
	suspectDB db.SuspectDB,
	evacuationDB db.EvacuationDB,
	desiredLRPDB db.DesiredLRPDB,
	auctioneerClient auctioneer.Client,
	serviceClient serviceclient.ServiceClient,
	repClientFactory rep.ClientFactory,
	actualHub events.Hub,
	actualLRPInstanceHub events.Hub,
) *ActualLRPLifecycleController {
	return &ActualLRPLifecycleController{
		db:                   db,
		suspectDB:            suspectDB,
		evacuationDB:         evacuationDB,
		desiredLRPDB:         desiredLRPDB,
		auctioneerClient:     auctioneerClient,
		serviceClient:        serviceClient,
		repClientFactory:     repClientFactory,
		actualHub:            actualHub,
		actualLRPInstanceHub: actualLRPInstanceHub,
	}
}

func findWithPresence(lrps []*models.ActualLRP, presence models.ActualLRP_Presence) *models.ActualLRP {
	for _, lrp := range lrps {
		if lrp.Presence == presence {
			return lrp
		}
	}
	return nil
}

func lookupLRPInSlice(lrps []*models.ActualLRP, key *models.ActualLRPInstanceKey) *models.ActualLRP {
	for _, lrp := range lrps {
		if lrp.ActualLRPInstanceKey == *key {
			return lrp
		}
	}
	return nil
}

func (h *ActualLRPLifecycleController) ClaimActualLRP(logger lager.Logger, processGUID string, index int32, actualLRPInstanceKey *models.ActualLRPInstanceKey) error {
	eventCalculator := calculator.ActualLRPEventCalculator{
		ActualLRPGroupHub:    h.actualHub,
		ActualLRPInstanceHub: h.actualLRPInstanceHub,
	}

	lrps, err := h.db.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: processGUID, Index: &index})
	if err != nil {
		return err
	}

	before, after, err := h.db.ClaimActualLRP(logger, processGUID, index, actualLRPInstanceKey)
	if err != nil {
		return err
	}

	newLRPs := eventCalculator.RecordChange(before, after, lrps)
	go eventCalculator.EmitEvents(lrps, newLRPs)

	return nil
}

func (h *ActualLRPLifecycleController) StartActualLRP(logger lager.Logger, actualLRPKey *models.ActualLRPKey, actualLRPInstanceKey *models.ActualLRPInstanceKey, actualLRPNetInfo *models.ActualLRPNetInfo) error {
	eventCalculator := calculator.ActualLRPEventCalculator{
		ActualLRPGroupHub:    h.actualHub,
		ActualLRPInstanceHub: h.actualLRPInstanceHub,
	}

	lrps, err := h.db.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: actualLRPKey.ProcessGuid, Index: &actualLRPKey.Index})
	if err != nil && err != models.ErrResourceNotFound {
		return err
	}

	lrp := lookupLRPInSlice(lrps, actualLRPInstanceKey)
	if lrp != nil && lrp.Presence == models.ActualLRP_Suspect {
		logger.Info("ignored-start-request-from-suspect", lager.Data{
			"process_guid":  actualLRPKey.ProcessGuid,
			"index":         actualLRPKey.Index,
			"instance_guid": actualLRPInstanceKey,
			"state":         lrp.State,
		})
		return nil
	}

	// creates ordinary running actual LRP if it doesn't exist, otherwise updates
	// the existing ordinary actual LRP to running state
	before, after, err := h.db.StartActualLRP(logger, actualLRPKey, actualLRPInstanceKey, actualLRPNetInfo)
	if err != nil {
		return err
	}
	newLRPs := eventCalculator.RecordChange(before, after, lrps)

	defer func() {
		go eventCalculator.EmitEvents(lrps, newLRPs)
	}()

	evacuating := findWithPresence(lrps, models.ActualLRP_Evacuating)
	suspect := findWithPresence(lrps, models.ActualLRP_Suspect)

	var suspectLRP *models.ActualLRP
	if evacuating != nil {
		h.evacuationDB.RemoveEvacuatingActualLRP(logger, &evacuating.ActualLRPKey, &evacuating.ActualLRPInstanceKey)
		newLRPs = eventCalculator.RecordChange(evacuating, nil, newLRPs)
	}

	// prior to starting this ActualLRP there was a suspect LRP that we need to remove
	if suspect != nil {
		suspectLRP, err = h.suspectDB.RemoveSuspectActualLRP(logger, actualLRPKey)
		if err != nil {
			logger.Error("failed-to-remove-suspect-lrp", err)
		} else {
			newLRPs = eventCalculator.RecordChange(suspectLRP, nil, newLRPs)
		}
	}

	return nil
}

func (h *ActualLRPLifecycleController) CrashActualLRP(logger lager.Logger, actualLRPKey *models.ActualLRPKey, actualLRPInstanceKey *models.ActualLRPInstanceKey, errorMessage string) error {
	lrps, err := h.db.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: actualLRPKey.ProcessGuid, Index: &actualLRPKey.Index})
	if err != nil {
		return err
	}

	eventCalculator := calculator.ActualLRPEventCalculator{
		ActualLRPGroupHub:    h.actualHub,
		ActualLRPInstanceHub: h.actualLRPInstanceHub,
	}

	lrp := lookupLRPInSlice(lrps, actualLRPInstanceKey)
	if lrp != nil && lrp.Presence == models.ActualLRP_Suspect {
		suspectLRP, err := h.suspectDB.RemoveSuspectActualLRP(logger, actualLRPKey)
		if err != nil {
			return err
		}

		afterLRPs := eventCalculator.RecordChange(suspectLRP, nil, lrps)
		logger.Info("removing-suspect-lrp", lager.Data{"ig": suspectLRP.InstanceGuid})
		go eventCalculator.EmitEvents(lrps, afterLRPs)

		return nil
	}

	before, after, shouldRestart, err := h.db.CrashActualLRP(logger, actualLRPKey, actualLRPInstanceKey, errorMessage)
	if err != nil {
		return err
	}

	afterLRPs := eventCalculator.RecordChange(before, after, lrps)
	go eventCalculator.EmitEvents(lrps, afterLRPs)

	if !shouldRestart {
		return nil
	}

	desiredLRP, err := h.desiredLRPDB.DesiredLRPByProcessGuid(logger, actualLRPKey.ProcessGuid)
	if err != nil {
		logger.Error("failed-fetching-desired-lrp", err)
		return err
	}

	schedInfo := desiredLRP.DesiredLRPSchedulingInfo()
	startRequest := auctioneer.NewLRPStartRequestFromSchedulingInfo(&schedInfo, int(actualLRPKey.Index))
	logger.Info("start-lrp-auction-request", lager.Data{"app_guid": schedInfo.ProcessGuid, "index": int(actualLRPKey.Index)})
	err = h.auctioneerClient.RequestLRPAuctions(logger, []*auctioneer.LRPStartRequest{&startRequest})
	logger.Info("finished-lrp-auction-request", lager.Data{"app_guid": schedInfo.ProcessGuid, "index": int(actualLRPKey.Index)})
	if err != nil {
		logger.Error("failed-requesting-auction", err)
	}
	return nil
}

func (h *ActualLRPLifecycleController) FailActualLRP(logger lager.Logger, key *models.ActualLRPKey, errorMessage string) error {
	lrps, err := h.db.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: key.ProcessGuid, Index: &key.Index})
	if err != nil {
		return err
	}

	before, after, err := h.db.FailActualLRP(logger, key, errorMessage)
	if err != nil && err != models.ErrResourceNotFound {
		return err
	}

	eventCalculator := calculator.ActualLRPEventCalculator{
		ActualLRPGroupHub:    h.actualHub,
		ActualLRPInstanceHub: h.actualLRPInstanceHub,
	}

	newLRPs := eventCalculator.RecordChange(before, after, lrps)
	go eventCalculator.EmitEvents(lrps, newLRPs)

	return nil
}

func (h *ActualLRPLifecycleController) RemoveActualLRP(logger lager.Logger, processGUID string, index int32, instanceKey *models.ActualLRPInstanceKey) error {
	beforeLRPs, err := h.db.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: processGUID, Index: &index})
	if err != nil {
		return err
	}

	lrp := findWithPresence(beforeLRPs, models.ActualLRP_Ordinary)
	if lrp == nil {
		return models.ErrResourceNotFound
	}

	err = h.db.RemoveActualLRP(logger, processGUID, index, instanceKey)
	if err != nil {
		return err
	}

	eventCalculator := calculator.ActualLRPEventCalculator{
		ActualLRPGroupHub:    h.actualHub,
		ActualLRPInstanceHub: h.actualLRPInstanceHub,
	}

	newLRPs := eventCalculator.RecordChange(lrp, nil, beforeLRPs)
	go eventCalculator.EmitEvents(beforeLRPs, newLRPs)

	return nil
}

func (h *ActualLRPLifecycleController) RetireActualLRP(logger lager.Logger, key *models.ActualLRPKey) error {
	var err error
	var cell *models.CellPresence

	logger = logger.Session("retire-actual-lrp", lager.Data{"process_guid": key.ProcessGuid, "index": key.Index})

	lrps, err := h.db.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: key.ProcessGuid, Index: &key.Index})
	if err != nil {
		return err
	}

	eventCalculator := calculator.ActualLRPEventCalculator{
		ActualLRPGroupHub:    h.actualHub,
		ActualLRPInstanceHub: h.actualLRPInstanceHub,
	}

	lrp := findWithPresence(lrps, models.ActualLRP_Ordinary)
	if lrp == nil {
		return models.ErrResourceNotFound
	}

	newLRPs := make([]*models.ActualLRP, len(lrps))
	copy(newLRPs, lrps)

	defer func() {
		go eventCalculator.EmitEvents(lrps, newLRPs)
	}()

	removeLRP := func() error {
		err = h.db.RemoveActualLRP(logger, lrp.ProcessGuid, lrp.Index, &lrp.ActualLRPInstanceKey)
		if err == nil {
			newLRPs = eventCalculator.RecordChange(lrp, nil, lrps)
		}
		return err
	}

	for retryCount := 0; retryCount < models.RetireActualLRPRetryAttempts; retryCount++ {
		switch lrp.State {
		case models.ActualLRPStateUnclaimed, models.ActualLRPStateCrashed:
			err = removeLRP()
		case models.ActualLRPStateClaimed, models.ActualLRPStateRunning:
			cell, err = h.serviceClient.CellById(logger, lrp.CellId)
			if err != nil {
				bbsErr := models.ConvertError(err)
				if bbsErr.Type == models.Error_ResourceNotFound {
					return removeLRP()
				}
				return err
			}

			var client rep.Client
			client, err = h.repClientFactory.CreateClient(cell.RepAddress, cell.RepUrl)
			if err != nil {
				return err
			}
			err = client.StopLRPInstance(logger, lrp.ActualLRPKey, lrp.ActualLRPInstanceKey)
		}

		if err == nil {
			return nil
		}

		logger.Error("retrying-failed-retire-of-actual-lrp", err, lager.Data{"attempt": retryCount + 1})
	}

	return err
}
