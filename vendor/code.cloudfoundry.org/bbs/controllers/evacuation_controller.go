package controllers

import (
	"code.cloudfoundry.org/auctioneer"
	"code.cloudfoundry.org/bbs/db"
	"code.cloudfoundry.org/bbs/events"
	"code.cloudfoundry.org/bbs/events/calculator"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"
)

type EvacuationController struct {
	db                   db.EvacuationDB
	actualLRPDB          db.ActualLRPDB
	suspectLRPDB         db.SuspectDB
	desiredLRPDB         db.DesiredLRPDB
	auctioneerClient     auctioneer.Client
	actualHub            events.Hub
	actualLRPInstanceHub events.Hub
}

func NewEvacuationController(
	db db.EvacuationDB,
	actualLRPDB db.ActualLRPDB,
	suspectLRPDB db.SuspectDB,
	desiredLRPDB db.DesiredLRPDB,
	auctioneerClient auctioneer.Client,
	actualHub events.Hub,
	actualLRPInstanceHub events.Hub,
) *EvacuationController {
	return &EvacuationController{
		db:                   db,
		actualLRPDB:          actualLRPDB,
		suspectLRPDB:         suspectLRPDB,
		desiredLRPDB:         desiredLRPDB,
		auctioneerClient:     auctioneerClient,
		actualHub:            actualHub,
		actualLRPInstanceHub: actualLRPInstanceHub,
	}
}

func (h *EvacuationController) RemoveEvacuatingActualLRP(logger lager.Logger, actualLRPKey *models.ActualLRPKey, actualLRPInstanceKey *models.ActualLRPInstanceKey) error {
	actualLRPs, err := h.actualLRPDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: actualLRPKey.ProcessGuid, Index: &actualLRPKey.Index})
	if err != nil {
		return err
	}

	eventCalculator := calculator.ActualLRPEventCalculator{
		ActualLRPGroupHub:    h.actualHub,
		ActualLRPInstanceHub: h.actualLRPInstanceHub,
	}

	newLRPs := make([]*models.ActualLRP, len(actualLRPs))
	copy(newLRPs, actualLRPs)
	defer func() {
		go eventCalculator.EmitEvents(actualLRPs, newLRPs)
	}()

	lrp := lookupLRPInSlice(actualLRPs, actualLRPInstanceKey)
	if lrp == nil {
		logger.Debug("actual-lrp-not-found", lager.Data{"guid": actualLRPKey.ProcessGuid, "index": actualLRPKey.Index})
		return models.ErrResourceNotFound
	}

	if lrp.Presence != models.ActualLRP_Evacuating {
		logger.Info("evacuating-lrp-is-empty")
		return models.ErrResourceNotFound
	}

	evacuatingLRPLogData := lager.Data{
		"process-guid": actualLRPKey.ProcessGuid,
		"index":        actualLRPKey.Index,
		"instance-key": actualLRPInstanceKey,
	}

	instance := findWithPresence(actualLRPs, models.ActualLRP_Ordinary)
	if instance != nil {
		evacuatingLRPLogData["replacement-lrp-instance-key"] = instance.ActualLRPInstanceKey
		evacuatingLRPLogData["replacement-state"] = instance.State
		evacuatingLRPLogData["replacement-lrp-placement-error"] = instance.PlacementError
	}

	logger.Info("removing-stranded-evacuating-actual-lrp", evacuatingLRPLogData)

	err = h.db.RemoveEvacuatingActualLRP(logger, actualLRPKey, actualLRPInstanceKey)
	if err != nil {
		return err
	}
	newLRPs = eventCalculator.RecordChange(lrp, nil, actualLRPs)

	return nil
}

// removeEvacuatingOrSuspect removes an evacuating or suspect LRP if they
// exist.  Returns true if the LRP was found and removed, false otherwise.
// Also returns the new lrp set and any errors encountered.
//
// This is a helper function used by all evacuating controller endpoints
// (e.g. EvacuateClaimedActualLRP) that delete the LRP because transitioning
// the LRP state wouldn't make sense if the presence is Suspect or Evacuating.
func (h *EvacuationController) removeEvacuatingOrSuspect(
	logger lager.Logger,
	calculator calculator.ActualLRPEventCalculator,
	lrps []*models.ActualLRP,
	key *models.ActualLRPKey,
	instanceKey *models.ActualLRPInstanceKey,
) (bool, []*models.ActualLRP, error) {
	lrp := lookupLRPInSlice(lrps, instanceKey)
	if lrp == nil {
		logger.Debug("actual-lrp-not-found", lager.Data{"guid": key.ProcessGuid, "index": key.Index})
		return false, lrps, models.ErrResourceNotFound
	}

	switch lrp.Presence {
	case models.ActualLRP_Evacuating:
		err := h.db.RemoveEvacuatingActualLRP(logger, key, instanceKey)
		if err != nil {
			logger.Error("failed-removing-evacuating-actual-lrp", err)
			return false, lrps, err
		}
	case models.ActualLRP_Suspect:
		_, err := h.suspectLRPDB.RemoveSuspectActualLRP(logger, key)
		if err != nil {
			logger.Error("failed-removing-suspect-actual-lrp", err)
			return false, lrps, err
		}
	default:
		return false, lrps, nil
	}

	lrps = calculator.RecordChange(lrp, nil, lrps)
	return true, lrps, nil
}

func (h *EvacuationController) EvacuateClaimedActualLRP(logger lager.Logger, actualLRPKey *models.ActualLRPKey, actualLRPInstanceKey *models.ActualLRPInstanceKey) (bool, error) {
	eventCalculator := calculator.ActualLRPEventCalculator{
		ActualLRPGroupHub:    h.actualHub,
		ActualLRPInstanceHub: h.actualLRPInstanceHub,
	}

	guid := actualLRPKey.ProcessGuid
	index := actualLRPKey.Index
	actualLRPs, err := h.actualLRPDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: guid, Index: &index})
	if err != nil {
		logger.Error("failed-fetching-actual-lrps", err, lager.Data{"guid": guid, "index": index})
		return false, err
	}

	newLRPs := make([]*models.ActualLRP, len(actualLRPs))
	copy(newLRPs, actualLRPs)

	defer func() {
		go eventCalculator.EmitEvents(actualLRPs, newLRPs)
	}()

	removed, newLRPs, err := h.removeEvacuatingOrSuspect(logger, eventCalculator, newLRPs, actualLRPKey, actualLRPInstanceKey)
	if err != nil {
		return false, err
	}

	if removed {
		return false, nil
	}

	// this is an ordinary LRP
	before, after, err := h.actualLRPDB.UnclaimActualLRP(logger, actualLRPKey)
	bbsErr := models.ConvertError(err)
	if bbsErr != nil {
		if bbsErr.Type == models.Error_ResourceNotFound {
			return false, nil
		}
		return true, bbsErr
	}

	newLRPs = eventCalculator.RecordChange(before, after, newLRPs)

	h.requestAuction(logger, actualLRPKey)

	return false, nil
}

func (h *EvacuationController) EvacuateCrashedActualLRP(logger lager.Logger, actualLRPKey *models.ActualLRPKey, actualLRPInstanceKey *models.ActualLRPInstanceKey, errorMessage string) error {
	eventCalculator := calculator.ActualLRPEventCalculator{
		ActualLRPGroupHub:    h.actualHub,
		ActualLRPInstanceHub: h.actualLRPInstanceHub,
	}

	guid := actualLRPKey.ProcessGuid
	index := actualLRPKey.Index

	actualLRPs, err := h.actualLRPDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: guid, Index: &index})
	if err != nil {
		logger.Error("failed-fetching-actual-lrps", err)
		return err
	}

	newLRPs := make([]*models.ActualLRP, len(actualLRPs))
	copy(newLRPs, actualLRPs)

	defer func() {
		go eventCalculator.EmitEvents(actualLRPs, newLRPs)
	}()

	removed, newLRPs, err := h.removeEvacuatingOrSuspect(logger, eventCalculator, newLRPs, actualLRPKey, actualLRPInstanceKey)
	if err != nil {
		return err
	}

	if removed {
		return nil
	}

	before, after, _, err := h.actualLRPDB.CrashActualLRP(logger, actualLRPKey, actualLRPInstanceKey, errorMessage)
	if err != nil {
		logger.Error("failed-to-crash-actual-lrp", err)
		return err
	}

	newLRPs = eventCalculator.RecordChange(before, after, newLRPs)

	return nil
}

// EvacuateRunningActualLRP evacuates the LRP with the given lrp keys.  This
// function has to handle the following cases:
//
// 1. Create a Evacuating LRP if one doesn't already exist and this isn't the
// Ordinary LRP.
//
// 2. Do the evacuation dance if this is the Ordinary LRP
//
// 3. Remove the evacuating LRP if it is no longer needed (an Ordinary is
// running or the desired LRP was removed)
//
// Refer to
// https://github.com/cloudfoundry/diego-notes/tree/2cbd7451#harmonizing-during-evacuation
// for more details.
func (h *EvacuationController) EvacuateRunningActualLRP(logger lager.Logger, actualLRPKey *models.ActualLRPKey, actualLRPInstanceKey *models.ActualLRPInstanceKey, netInfo *models.ActualLRPNetInfo) (bool, error) {
	eventCalculator := calculator.ActualLRPEventCalculator{
		ActualLRPGroupHub:    h.actualHub,
		ActualLRPInstanceHub: h.actualLRPInstanceHub,
	}
	guid := actualLRPKey.ProcessGuid
	index := actualLRPKey.Index
	actualLRPs, err := h.actualLRPDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: guid, Index: &index})
	if err != nil {
		logger.Error("failed-fetching-actual-lrps", err)
		return true, err
	}

	if len(actualLRPs) == 0 {
		return false, nil
	}

	newLRPs := make([]*models.ActualLRP, len(actualLRPs))
	copy(newLRPs, actualLRPs)

	defer func() {
		go eventCalculator.EmitEvents(actualLRPs, newLRPs)
	}()

	// the ActualLRP whose InstanceGuid, and CellId match the method
	// parameters.
	targetActualLRP := lookupLRPInSlice(actualLRPs, actualLRPInstanceKey)

	instance := findWithPresence(actualLRPs, models.ActualLRP_Ordinary)

	// `instance == nil' means the DesiredLRP has been removed and
	// stopInstancesFrom deleted the Ordinary instance.
	desiredLRPIsRemoved := instance == nil

	// the replacement is already running or crashed.  Wrapped in a function so
	// we can short circuit its evaluation if instance is nil.
	replacementLRPIsRunning := func() bool {
		return !instance.Equal(targetActualLRP) &&
			(instance.State == models.ActualLRPStateRunning ||
				instance.State == models.ActualLRPStateCrashed)
	}

	if desiredLRPIsRemoved || replacementLRPIsRunning() {
		removedEvacuating, err := h.removeEvacuating(logger, targetActualLRP)
		newLRPs = eventCalculator.RecordChange(removedEvacuating, nil, newLRPs)
		keepContainer := err != nil
		return keepContainer, err
	}

	if targetActualLRP == nil || targetActualLRP.Presence == models.ActualLRP_Evacuating {
		// Create a new Evacuating LRP or update an existing one
		evacuating := findWithPresence(actualLRPs, models.ActualLRP_Evacuating)

		if evacuating != nil && !evacuating.Equal(targetActualLRP) {
			// There is already another evacuating instance.  Let the Rep know
			// that we don't need this instance anymore.  We can't have more
			// than one evacuating instance.
			logger.Info("already-evacuated-by-different-cell")
			return false, nil
		}

		// FIXME: there might be a bug when the LRP is originally in the CLAIMED
		// state.  db.EvacuateActualLRP always create an evacuating LRP in the
		// running state regardless.
		newLRP, err := h.db.EvacuateActualLRP(logger, actualLRPKey, actualLRPInstanceKey, netInfo)

		if err != nil {
			logger.Error("failed-evacuating-actual-lrp", err)
		}

		if err == models.ErrResourceExists {
			// nothing to do, the evacuating LRP already exists in the DB
			return true, nil
		}

		newLRPs = eventCalculator.RecordChange(nil, newLRP, newLRPs)
		return true, err
	}

	if (targetActualLRP.State == models.ActualLRPStateRunning) ||
		(targetActualLRP.State == models.ActualLRPStateClaimed) {
		// do the evacuation dance.  Change the instance from Running/Ordinary
		// -> Running/Evacuating and create a new Unclaimed/Ordinary LRP.
		err = h.evacuateInstance(logger, actualLRPs, targetActualLRP)
		return true, err
	}

	// for all other states, just delete the container.
	return false, nil
}

func (h *EvacuationController) EvacuateStoppedActualLRP(logger lager.Logger, actualLRPKey *models.ActualLRPKey, actualLRPInstanceKey *models.ActualLRPInstanceKey) error {
	eventCalculator := calculator.ActualLRPEventCalculator{
		ActualLRPGroupHub:    h.actualHub,
		ActualLRPInstanceHub: h.actualLRPInstanceHub,
	}

	guid := actualLRPKey.ProcessGuid
	index := actualLRPKey.Index

	actualLRPs, err := h.actualLRPDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: guid, Index: &index})
	if err != nil {
		logger.Error("failed-fetching-actual-lrps", err)
		return err
	}

	newLRPs := make([]*models.ActualLRP, len(actualLRPs))
	copy(newLRPs, actualLRPs)

	defer func() {
		go eventCalculator.EmitEvents(actualLRPs, newLRPs)
	}()

	removed, newLRPs, err := h.removeEvacuatingOrSuspect(logger, eventCalculator, newLRPs, actualLRPKey, actualLRPInstanceKey)
	if err != nil {
		return err
	}

	if removed {
		return nil
	}

	err = h.actualLRPDB.RemoveActualLRP(logger, guid, index, actualLRPInstanceKey)
	if err != nil {
		logger.Error("failed-to-remove-actual-lrp", err)
		return err
	}

	lrp := lookupLRPInSlice(actualLRPs, actualLRPInstanceKey)
	newLRPs = eventCalculator.RecordChange(lrp, nil, newLRPs)

	return nil
}

func (h *EvacuationController) requestAuction(logger lager.Logger, lrpKey *models.ActualLRPKey) {
	desiredLRP, err := h.desiredLRPDB.DesiredLRPByProcessGuid(logger, lrpKey.ProcessGuid)
	if err != nil {
		logger.Error("failed-fetching-desired-lrp", err)
		return
	}

	schedInfo := desiredLRP.DesiredLRPSchedulingInfo()
	startRequest := auctioneer.NewLRPStartRequestFromSchedulingInfo(&schedInfo, int(lrpKey.Index))
	err = h.auctioneerClient.RequestLRPAuctions(logger, []*auctioneer.LRPStartRequest{&startRequest})
	if err != nil {
		logger.Error("failed-requesting-auction", err)
	}
}

func (h *EvacuationController) evacuateInstance(logger lager.Logger, allLRPs []*models.ActualLRP, actualLRP *models.ActualLRP) error {

	eventCalculator := calculator.ActualLRPEventCalculator{
		ActualLRPGroupHub:    h.actualHub,
		ActualLRPInstanceHub: h.actualLRPInstanceHub,
	}

	evacuating, err := h.db.EvacuateActualLRP(logger, &actualLRP.ActualLRPKey, &actualLRP.ActualLRPInstanceKey, &actualLRP.ActualLRPNetInfo)
	if err != nil {
		return err
	}

	// although EvacuateActualLRP above creates a new database record.  We
	// would like to record that as a change event instead, since the instance
	// guid hasn't changed.  This will produce a simpler instance event stream
	// with a single changed event and keep the group events backward
	// compatible.
	newLRPs := eventCalculator.RecordChange(actualLRP, evacuating, allLRPs)

	defer func() {
		go eventCalculator.EmitEvents(allLRPs, newLRPs)
	}()

	if actualLRP.Presence == models.ActualLRP_Suspect {
		_, err := h.suspectLRPDB.RemoveSuspectActualLRP(logger, &actualLRP.ActualLRPKey)
		if err != nil {
			logger.Error("failed-removing-suspect-actual-lrp", err)
			return err
		}

		return nil
	}

	_, after, err := h.actualLRPDB.UnclaimActualLRP(logger, &actualLRP.ActualLRPKey)
	if err != nil {
		return err
	}

	// although UnclaimActualLRP above updates a database record.  We would
	// like to record that as a create event instead.  This will produce a
	// simpler instance event stream and keep the group events backward
	// compatible.
	newLRPs = eventCalculator.RecordChange(nil, after, newLRPs)

	h.requestAuction(logger, &actualLRP.ActualLRPKey)
	return nil
}

func (h *EvacuationController) removeEvacuating(logger lager.Logger, evacuating *models.ActualLRP) (*models.ActualLRP, error) {
	if evacuating == nil {
		return nil, nil
	}

	err := h.db.RemoveEvacuatingActualLRP(logger, &evacuating.ActualLRPKey, &evacuating.ActualLRPInstanceKey)

	if err == nil {
		return evacuating, nil
	}

	if err == models.ErrActualLRPCannotBeRemoved {
		return nil, nil
	}

	return nil, err
}
