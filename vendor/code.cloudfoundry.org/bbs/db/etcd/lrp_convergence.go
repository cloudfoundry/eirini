package etcd

import (
	"fmt"
	"path"
	"sync"
	"sync/atomic"
	"time"

	"code.cloudfoundry.org/auctioneer"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/workpool"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
)

const (
	convergeLRPRunsCounter = "ConvergenceLRPRuns"
	convergeLRPDuration    = "ConvergenceLRPDuration"

	malformedSchedulingInfosMetric = "ConvergenceLRPPreProcessingMalformedSchedulingInfos"
	malformedRunInfosMetric        = "ConvergenceLRPPreProcessingMalformedRunInfos"
	actualLRPsDeleted              = "ConvergenceLRPPreProcessingActualLRPsDeleted"
	orphanedRunInfosMetric         = "ConvergenceLRPPreProcessingOrphanedRunInfos"

	claimedLRPs         = "LRPsClaimed"
	desiredLRPs         = "LRPsDesired"
	extraLRPs           = "LRPsExtra"
	missingLRPs         = "LRPsMissing"
	runningLRPs         = "LRPsRunning"
	unclaimedLRPs       = "LRPsUnclaimed"
	crashedActualLRPs   = "CrashedActualLRPs"
	crashingDesiredLRPs = "CrashingDesiredLRPs"
)

func (db *ETCDDB) ConvergeLRPs(logger lager.Logger, cellSet models.CellSet) ([]*auctioneer.LRPStartRequest, []*models.ActualLRPKeyWithSchedulingInfo, []*models.ActualLRPKey, []models.Event) {
	convergeStart := db.clock.Now()
	db.metronClient.IncrementCounter(convergeLRPRunsCounter)
	logger = logger.Session("etcd")
	logger.Info("starting-convergence")
	defer logger.Info("finished-convergence")

	defer func() {
		err := db.metronClient.SendDuration(convergeLRPDuration, time.Since(convergeStart))
		if err != nil {
			logger.Error("failed-sending-converge-lrp-duration-metric", err)
		}
	}()

	logger.Debug("gathering-convergence-input")
	input, err := db.GatherAndPruneLRPs(logger, cellSet)
	if err != nil {
		logger.Error("failed-gathering-convergence-input", err)
		return nil, nil, nil, nil
	}
	logger.Debug("succeeded-gathering-convergence-input")

	changes := CalculateConvergence(logger, db, db.clock, models.NewDefaultRestartCalculator(), input)

	return db.ResolveConvergence(logger, input.DesiredLRPs, changes)
}

type LRPMetricCounter struct {
	unclaimedLRPs       int32
	claimedLRPs         int32
	runningLRPs         int32
	crashedActualLRPs   int32
	crashingDesiredLRPs int32
	desiredLRPs         int32
}

func (lmc LRPMetricCounter) Send(logger lager.Logger, db *ETCDDB) {
	err := db.metronClient.SendMetric(unclaimedLRPs, int(lmc.unclaimedLRPs))
	if err != nil {
		logger.Error("failed-sending-unclaimed-lrps-metric", err)
	}

	err = db.metronClient.SendMetric(claimedLRPs, int(lmc.claimedLRPs))
	if err != nil {
		logger.Error("failed-sending-claimed-lrps-metric", err)
	}

	err = db.metronClient.SendMetric(runningLRPs, int(lmc.runningLRPs))
	if err != nil {
		logger.Error("failed-sending-running-lrps-metric", err)
	}

	err = db.metronClient.SendMetric(crashedActualLRPs, int(lmc.crashedActualLRPs))
	if err != nil {
		logger.Error("failed-sending-crashed-actual-lrps-metric", err)
	}

	err = db.metronClient.SendMetric(crashingDesiredLRPs, int(lmc.crashingDesiredLRPs))
	if err != nil {
		logger.Error("failed-sending-crashing-desired-lrps-metric", err)
	}

	err = db.metronClient.SendMetric(desiredLRPs, int(lmc.desiredLRPs))
	if err != nil {
		logger.Error("failed-sending-desired-lrps-metric", err)
	}
}

func (db *ETCDDB) GatherAndPruneLRPs(logger lager.Logger, cellSet models.CellSet) (*models.ConvergenceInput, error) {
	guids := map[string]struct{}{}
	// always fetch actualLRPs before desiredLRPs to ensure correctness
	logger.Debug("gathering-and-pruning-actual-lrps")
	lrpMetricCounter := &LRPMetricCounter{}

	actuals, err := db.gatherAndPruneActualLRPs(logger, guids, lrpMetricCounter) // modifies guids
	if err != nil {
		logger.Error("failed-gathering-and-pruning-actual-lrps", err)

		lrpMetricCounter.unclaimedLRPs = -1
		lrpMetricCounter.claimedLRPs = -1
		lrpMetricCounter.runningLRPs = -1
		lrpMetricCounter.crashedActualLRPs = -1
		lrpMetricCounter.crashingDesiredLRPs = -1
		lrpMetricCounter.desiredLRPs = -1

		lrpMetricCounter.Send(logger, db)

		return &models.ConvergenceInput{}, err
	}
	logger.Debug("succeeded-gathering-and-pruning-actual-lrps")

	// always fetch desiredLRPs after actualLRPs to ensure correctness
	logger.Debug("gathering-and-pruning-desired-lrps")
	desireds, err := db.GatherAndPruneDesiredLRPs(logger, guids, lrpMetricCounter) // modifies guids
	if err != nil {
		logger.Error("failed-gathering-and-pruning-desired-lrps", err)

		lrpMetricCounter.desiredLRPs = -1
		lrpMetricCounter.Send(logger, db)

		return &models.ConvergenceInput{}, err
	}
	logger.Debug("succeeded-gathering-and-pruning-desired-lrps")

	lrpMetricCounter.Send(logger, db)

	logger.Debug("listing-domains")
	domains, err := db.Domains(logger)
	if err != nil {
		return &models.ConvergenceInput{}, err
	}
	logger.Debug("succeeded-listing-domains")
	for _, domain := range domains {
		err = db.metronClient.SendMetric(fmt.Sprintf("Domain.%s", domain), 1)
		if err != nil {
			logger.Error("failed-sending-domain-metric", err)
		}
	}

	return &models.ConvergenceInput{
		AllProcessGuids: guids,
		DesiredLRPs:     desireds,
		ActualLRPs:      actuals,
		Domains:         models.NewDomainSet(domains),
		Cells:           cellSet,
	}, nil
}

func (db *ETCDDB) gatherAndPruneActualLRPs(logger lager.Logger, guids map[string]struct{}, lmc *LRPMetricCounter) (map[string]map[int32]*models.ActualLRP, error) {
	return db.gatherAndOptionallyPruneActualLRPs(logger, guids, true, lmc)
}

func (db *ETCDDB) GatherActualLRPs(logger lager.Logger, guids map[string]struct{}, lmc *LRPMetricCounter) (map[string]map[int32]*models.ActualLRP, error) {
	return db.gatherAndOptionallyPruneActualLRPs(logger, guids, false, lmc)
}

func (db *ETCDDB) gatherAndOptionallyPruneActualLRPs(logger lager.Logger, guids map[string]struct{}, doPrune bool, lmc *LRPMetricCounter) (map[string]map[int32]*models.ActualLRP, error) {
	response, modelErr := db.fetchRecursiveRaw(logger, ActualLRPSchemaRoot)

	if modelErr == models.ErrResourceNotFound {
		logger.Info("actual-lrp-schema-root-not-found")
		return map[string]map[int32]*models.ActualLRP{}, nil
	}

	if modelErr != nil {
		return nil, modelErr
	}

	actuals := map[string]map[int32]*models.ActualLRP{}
	var guidKeysToDelete, indexKeysToDelete []string
	var actualsToDelete []string
	var guidsLock, actualsLock, guidKeysToDeleteLock, indexKeysToDeleteLock,
		crashingDesiredsLock, actualsToDeleteLock sync.Mutex

	logger.Debug("walking-actual-lrp-tree")
	works := []func(){}
	crashingDesireds := map[string]struct{}{}

	for _, guidGroup := range response.Nodes {
		guidGroup := guidGroup
		works = append(works, func() {
			guidGroupWillBeEmpty := true

			for _, indexGroup := range guidGroup.Nodes {
				indexGroupWillBeEmpty := true

				for _, actualNode := range indexGroup.Nodes {
					actual := new(models.ActualLRP)
					err := db.deserializeModel(logger, actualNode, actual)
					if err != nil {
						actualsToDeleteLock.Lock()
						actualsToDelete = append(actualsToDelete, actualNode.Key)
						actualsToDeleteLock.Unlock()

						continue
					}

					err = actual.Validate()
					if err != nil {
						actualsToDeleteLock.Lock()
						actualsToDelete = append(actualsToDelete, actualNode.Key)
						actualsToDeleteLock.Unlock()

						continue
					}

					indexGroupWillBeEmpty = false
					guidGroupWillBeEmpty = false

					switch actual.State {
					case models.ActualLRPStateUnclaimed:
						atomic.AddInt32(&lmc.unclaimedLRPs, 1)
					case models.ActualLRPStateClaimed:
						atomic.AddInt32(&lmc.claimedLRPs, 1)
					case models.ActualLRPStateRunning:
						atomic.AddInt32(&lmc.runningLRPs, 1)
					case models.ActualLRPStateCrashed:
						crashingDesiredsLock.Lock()
						crashingDesireds[actual.ProcessGuid] = struct{}{}
						crashingDesiredsLock.Unlock()
						atomic.AddInt32(&lmc.crashedActualLRPs, 1)
					}

					guidsLock.Lock()
					guids[actual.ProcessGuid] = struct{}{}
					guidsLock.Unlock()

					if path.Base(actualNode.Key) == ActualLRPInstanceKey {
						actualsLock.Lock()
						if actuals[actual.ProcessGuid] == nil {
							actuals[actual.ProcessGuid] = map[int32]*models.ActualLRP{}
						}
						actuals[actual.ProcessGuid][actual.Index] = actual
						actualsLock.Unlock()
					}
				}

				if indexGroupWillBeEmpty {
					indexKeysToDeleteLock.Lock()
					indexKeysToDelete = append(indexKeysToDelete, indexGroup.Key)
					indexKeysToDeleteLock.Unlock()
				}
			}

			if guidGroupWillBeEmpty {
				guidKeysToDeleteLock.Lock()
				guidKeysToDelete = append(guidKeysToDelete, guidGroup.Key)
				guidKeysToDeleteLock.Unlock()
			}
		})
	}
	logger.Debug("done-walking-actual-lrp-tree")

	throttler, err := workpool.NewThrottler(db.convergenceWorkersSize, works)
	if err != nil {
		logger.Error("failed-to-create-throttler", err)
	}

	throttler.Work()

	if doPrune {
		logger.Info("deleting-invalid-actual-lrps", lager.Data{"num_lrps": len(actualsToDelete)})
		db.batchDeleteNodes(actualsToDelete, logger)
		db.metronClient.IncrementCounterWithDelta(actualLRPsDeleted, uint64(len(actualsToDelete)))

		logger.Info("deleting-empty-actual-indices", lager.Data{"num_indices": len(indexKeysToDelete)})
		err = db.deleteLeaves(logger, indexKeysToDelete)
		if err != nil {
			logger.Error("failed-deleting-empty-actual-indices", err, lager.Data{"num_indices": len(indexKeysToDelete)})
		} else {
			logger.Info("succeeded-deleting-empty-actual-indices", lager.Data{"num_indices": len(indexKeysToDelete)})
		}

		logger.Info("deleting-empty-actual-guids", lager.Data{"num_guids": len(guidKeysToDelete)})
		err = db.deleteLeaves(logger, guidKeysToDelete)
		if err != nil {
			logger.Error("failed-deleting-empty-actual-guids", err, lager.Data{"num_guids": len(guidKeysToDelete)})
		} else {
			logger.Info("succeeded-deleting-empty-actual-guids", lager.Data{"num_guids": len(guidKeysToDelete)})
		}
	}

	lmc.crashingDesiredLRPs = int32(len(crashingDesireds))

	return actuals, nil
}

func (db *ETCDDB) deleteLeaves(logger lager.Logger, keys []string) error {
	works := []func(){}

	for _, key := range keys {
		key := key
		works = append(works, func() {
			_, err := db.client.DeleteDir(key)
			if err != nil {
				logger.Error("failed-deleting-leaf-node", err, lager.Data{"key": key})
			}
		})
	}

	throttler, err := workpool.NewThrottler(db.convergenceWorkersSize, works)
	if err != nil {
		return err
	}

	throttler.Work()

	return nil
}

func (db *ETCDDB) GatherAndPruneDesiredLRPs(logger lager.Logger, guids map[string]struct{}, lmc *LRPMetricCounter) (map[string]*models.DesiredLRP, error) {
	desiredLRPsRoot, modelErr := db.fetchRecursiveRaw(logger, DesiredLRPComponentsSchemaRoot)

	if modelErr == models.ErrResourceNotFound {
		logger.Info("actual-lrp-schema-root-not-found")
		return map[string]*models.DesiredLRP{}, nil
	}

	if modelErr != nil {
		return nil, modelErr
	}

	schedulingInfos := map[string]*models.DesiredLRPSchedulingInfo{}
	runInfos := map[string]*models.DesiredLRPRunInfo{}

	var malformedSchedulingInfos, malformedRunInfos []string

	var guidsLock, schedulingInfosLock, runInfosLock sync.Mutex

	works := []func(){}
	logger.Debug("walking-desired-lrp-components-tree")

	for _, componentRoot := range desiredLRPsRoot.Nodes {
		switch componentRoot.Key {
		case DesiredLRPSchedulingInfoSchemaRoot:
			for _, node := range componentRoot.Nodes {
				node := node
				works = append(works, func() {
					var schedulingInfo models.DesiredLRPSchedulingInfo
					err := db.deserializeModel(logger, node, &schedulingInfo)
					if err != nil || schedulingInfo.Validate() != nil {
						logger.Error("failed-to-deserialize-scheduling-info", err)
						schedulingInfosLock.Lock()
						malformedSchedulingInfos = append(malformedSchedulingInfos, node.Key)
						schedulingInfosLock.Unlock()
					} else {
						schedulingInfosLock.Lock()
						schedulingInfos[schedulingInfo.ProcessGuid] = &schedulingInfo
						schedulingInfosLock.Unlock()
						atomic.AddInt32(&lmc.desiredLRPs, schedulingInfo.Instances)

						guidsLock.Lock()
						guids[schedulingInfo.ProcessGuid] = struct{}{}
						guidsLock.Unlock()
					}
				})
			}
		case DesiredLRPRunInfoSchemaRoot:
			for _, node := range componentRoot.Nodes {
				node := node
				works = append(works, func() {
					var runInfo models.DesiredLRPRunInfo
					err := db.deserializeModel(logger, node, &runInfo)
					if err != nil || runInfo.Validate() != nil {
						runInfosLock.Lock()
						malformedRunInfos = append(malformedRunInfos, node.Key)
						runInfosLock.Unlock()
					} else {
						runInfosLock.Lock()
						runInfos[runInfo.ProcessGuid] = &runInfo
						runInfosLock.Unlock()
					}
				})
			}
		default:
			err := fmt.Errorf("unrecognized node under desired LRPs root node: %s", componentRoot.Key)
			logger.Error("unrecognized-node", err)
			return nil, err
		}
	}

	throttler, err := workpool.NewThrottler(db.convergenceWorkersSize, works)
	if err != nil {
		logger.Error("failed-to-create-throttler", err)
	}

	throttler.Work()

	db.batchDeleteNodes(malformedSchedulingInfos, logger)
	db.batchDeleteNodes(malformedRunInfos, logger)

	db.metronClient.IncrementCounterWithDelta(malformedSchedulingInfosMetric, uint64(len(malformedSchedulingInfos)))
	db.metronClient.IncrementCounterWithDelta(malformedRunInfosMetric, uint64(len(malformedRunInfos)))

	logger.Debug("done-walking-desired-lrp-tree")

	desireds := make(map[string]*models.DesiredLRP)
	var schedInfosToDelete []string
	for guid, schedulingInfo := range schedulingInfos {
		runInfo, ok := runInfos[guid]
		if !ok {
			err := fmt.Errorf("Missing runInfo for GUID %s", guid)
			logger.Error("runInfo-not-found-error", err)
			schedInfosToDelete = append(schedInfosToDelete, DesiredLRPSchedulingInfoSchemaPath(guid))
		} else {
			desiredLRP := models.NewDesiredLRP(*schedulingInfo, *runInfo)
			desireds[guid] = &desiredLRP
		}
	}
	db.batchDeleteNodes(schedInfosToDelete, logger)

	// Check to see if we have orphaned RunInfos
	if len(runInfos) != len(schedulingInfos) {
		var runInfosToDelete []string
		for guid, runInfo := range runInfos {
			// If there is no corresponding SchedulingInfo and the RunInfo has
			// existed for longer than desiredLRPCreationTimeout, consider it orphaned
			// and delete it.
			_, ok := schedulingInfos[guid]
			if !ok && db.clock.Since(time.Unix(0, runInfo.CreatedAt)) > db.desiredLRPCreationTimeout {
				db.metronClient.IncrementCounter(orphanedRunInfosMetric)
				runInfosToDelete = append(runInfosToDelete, DesiredLRPRunInfoSchemaPath(guid))
			}
		}

		db.batchDeleteNodes(runInfosToDelete, logger)
	}

	return desireds, nil
}

func (db *ETCDDB) batchDeleteNodes(keys []string, logger lager.Logger) {
	if len(keys) == 0 {
		return
	}

	works := []func(){}

	for _, key := range keys {
		key := key
		works = append(works, func() {
			logger.Info("deleting", lager.Data{"key": key})
			_, err := db.client.Delete(key, true)
			if err != nil {
				logger.Error("failed-to-delete", err, lager.Data{
					"key": key,
				})
			}
		})
	}

	throttler, err := workpool.NewThrottler(db.convergenceWorkersSize, works)
	if err != nil {
		logger.Error("failed-to-create-throttler", err)
	}

	throttler.Work()
	return
}

func CalculateConvergence(
	logger lager.Logger,
	db *ETCDDB,
	clock clock.Clock,
	restartCalculator models.RestartCalculator,
	input *models.ConvergenceInput,
) *models.ConvergenceChanges {
	sess := logger.Session("calculate-convergence")

	var extraLRPCount, missingLRPCount int

	sess.Info("start")
	defer sess.Info("done")

	changes := &models.ConvergenceChanges{}

	now := clock.Now()

	for processGuid, _ := range input.AllProcessGuids {
		pLog := sess.WithData(lager.Data{
			"process_guid": processGuid,
		})

		desired, hasDesired := input.DesiredLRPs[processGuid]

		actualsByIndex := input.ActualLRPs[processGuid]

		if hasDesired {
			for i := int32(0); i < desired.Instances; i++ {
				if _, hasIndex := actualsByIndex[i]; !hasIndex {
					pLog.Info("missing", lager.Data{"index": i})
					missingLRPCount++
					lrpKey := models.NewActualLRPKey(desired.ProcessGuid, i, desired.Domain)
					changes.ActualLRPKeysForMissingIndices = append(
						changes.ActualLRPKeysForMissingIndices,
						&lrpKey,
					)
				}
			}

			for i, actual := range actualsByIndex {
				if actual.CellIsMissing(input.Cells) {
					pLog.Info("missing-cell", lager.Data{"index": i, "cell_id": actual.CellId})
					changes.ActualLRPsWithMissingCells = append(changes.ActualLRPsWithMissingCells, actual)
					continue
				}

				if actual.Index >= desired.Instances && input.Domains.Contains(desired.Domain) {
					pLog.Info("extra", lager.Data{"index": i})
					extraLRPCount++
					changes.ActualLRPsForExtraIndices = append(changes.ActualLRPsForExtraIndices, actual)
					continue
				}

				if actual.ShouldRestartCrash(now, restartCalculator) {
					pLog.Info("restart-crash", lager.Data{"index": i})
					changes.RestartableCrashedActualLRPs = append(changes.RestartableCrashedActualLRPs, actual)
					continue
				}

				if actual.ShouldStartUnclaimed(now) {
					pLog.Info("stale-unclaimed", lager.Data{"index": i})
					changes.StaleUnclaimedActualLRPs = append(changes.StaleUnclaimedActualLRPs, actual)
					continue
				}
			}
		} else {
			for i, actual := range actualsByIndex {
				if !input.Domains.Contains(actual.Domain) {
					pLog.Info("skipping-unfresh-domain")
					continue
				}

				pLog.Info("no-longer-desired", lager.Data{"index": i})
				extraLRPCount++
				changes.ActualLRPsForExtraIndices = append(changes.ActualLRPsForExtraIndices, actual)
			}
		}
	}

	db.metronClient.SendMetric(missingLRPs, missingLRPCount)
	db.metronClient.SendMetric(extraLRPs, extraLRPCount)
	return changes
}

func (db *ETCDDB) ResolveConvergence(logger lager.Logger, desiredLRPs map[string]*models.DesiredLRP, changes *models.ConvergenceChanges) ([]*auctioneer.LRPStartRequest, []*models.ActualLRPKeyWithSchedulingInfo, []*models.ActualLRPKey, []models.Event) {
	startRequests := newStartRequests(desiredLRPs)
	for _, actual := range changes.StaleUnclaimedActualLRPs {
		startRequests.Add(logger, &actual.ActualLRPKey)
	}

	works := []func(){}

	keysToRetire := make([]*models.ActualLRPKey, len(changes.ActualLRPsForExtraIndices))
	for i, actual := range changes.ActualLRPsForExtraIndices {
		keysToRetire[i] = &actual.ActualLRPKey
	}

	keysWithMissingCells := []*models.ActualLRPKeyWithSchedulingInfo{}
	for _, actual := range changes.ActualLRPsWithMissingCells {
		desiredLRP, ok := desiredLRPs[actual.ProcessGuid]
		if !ok {
			logger.Debug("actual-with-missing-cell-no-desired")
			continue
		}

		schedInfo := desiredLRP.DesiredLRPSchedulingInfo()

		key := &models.ActualLRPKeyWithSchedulingInfo{
			Key:            &actual.ActualLRPKey,
			SchedulingInfo: &schedInfo,
		}

		keysWithMissingCells = append(keysWithMissingCells, key)
	}

	for _, actualKey := range changes.ActualLRPKeysForMissingIndices {
		works = append(works, db.resolveActualsWithMissingIndices(logger, desiredLRPs[actualKey.ProcessGuid], actualKey, startRequests))
	}

	for _, actual := range changes.RestartableCrashedActualLRPs {
		works = append(works, db.resolveRestartableCrashedActualLRPS(logger, actual, startRequests))
	}

	throttler, err := workpool.NewThrottler(db.convergenceWorkersSize, works)
	if err != nil {
		logger.Error("failed-constructing-throttler", err, lager.Data{"max_workers": db.convergenceWorkersSize, "num_works": len(works)})
		return nil, nil, nil, nil
	}

	logger.Debug("waiting-for-lrp-convergence-work")
	throttler.Work()
	logger.Debug("done-waiting-for-lrp-convergence-work")

	return startRequests.Slice(), keysWithMissingCells, keysToRetire, nil
}

func (db *ETCDDB) resolveActualsWithMissingIndices(logger lager.Logger, desired *models.DesiredLRP, actualKey *models.ActualLRPKey, starts *startRequests) func() {
	return func() {
		logger = logger.Session("start-missing-actual", lager.Data{
			"process_guid": actualKey.ProcessGuid,
			"index":        actualKey.Index,
		})

		logger.Debug("creating-actual-lrp")
		err := db.createActualLRP(logger, desired, actualKey.Index)
		if err != nil {
			logger.Error("failed-creating-actual-lrp", err)
			return
		}
		logger.Debug("succeeded-creating-actual-lrp")

		starts.Add(logger, actualKey)
	}
}

func (db *ETCDDB) resolveRestartableCrashedActualLRPS(logger lager.Logger, actualLRP *models.ActualLRP, starts *startRequests) func() {
	return func() {
		actualKey := actualLRP.ActualLRPKey

		logger = logger.Session("restart-crash", lager.Data{
			"process_guid": actualKey.ProcessGuid,
			"index":        actualKey.Index,
		})

		if actualLRP.State != models.ActualLRPStateCrashed {
			logger.Error("failed-actual-lrp-state-is-not-crashed", nil)
			return
		}

		logger.Debug("unclaiming-actual-lrp", lager.Data{"process_guid": actualLRP.ActualLRPKey.ProcessGuid, "index": actualLRP.ActualLRPKey.Index})
		_, err := db.unclaimActualLRP(logger, &actualLRP.ActualLRPKey, &actualLRP.ActualLRPInstanceKey)
		if err != nil {
			logger.Error("failed-unclaiming-crash", err)
			return
		}
		logger.Debug("succeeded-unclaiming-actual-lrp")

		starts.Add(logger, &actualKey)
	}
}

type startRequests struct {
	desiredMap    map[string]*models.DesiredLRP
	startMap      map[string]*auctioneer.LRPStartRequest
	instanceCount uint64
	*sync.Mutex
}

func newStartRequests(desiredMap map[string]*models.DesiredLRP) *startRequests {
	return &startRequests{
		desiredMap: desiredMap,
		startMap:   make(map[string]*auctioneer.LRPStartRequest),
		Mutex:      new(sync.Mutex),
	}
}

func (s *startRequests) Add(logger lager.Logger, actual *models.ActualLRPKey) {
	s.Lock()
	defer s.Unlock()

	desiredLRP, found := s.desiredMap[actual.ProcessGuid]
	if !found {
		logger.Info("failed-to-find-desired-lrp-for-stale-unclaimed-actual-lrp", lager.Data{"actual_lrp": actual})
		return
	}

	start, found := s.startMap[desiredLRP.ProcessGuid]
	if !found {
		startRequest := auctioneer.NewLRPStartRequestFromModel(desiredLRP, int(actual.Index))
		start = &startRequest
	} else {
		start.Indices = append(start.Indices, int(actual.Index))
	}

	logger.Info("adding-start-auction", lager.Data{"process_guid": desiredLRP.ProcessGuid, "index": actual.Index})
	s.startMap[desiredLRP.ProcessGuid] = start
	s.instanceCount++
}

func (s *startRequests) Slice() []*auctioneer.LRPStartRequest {
	s.Lock()
	defer s.Unlock()

	starts := make([]*auctioneer.LRPStartRequest, 0, len(s.startMap))
	for _, start := range s.startMap {
		starts = append(starts, start)
	}
	return starts
}

func (s *startRequests) InstanceCount() uint64 {
	s.Lock()
	defer s.Unlock()

	return s.instanceCount
}
