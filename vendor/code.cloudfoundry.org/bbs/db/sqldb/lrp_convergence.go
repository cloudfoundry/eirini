package sqldb

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"code.cloudfoundry.org/auctioneer"
	"code.cloudfoundry.org/bbs/db/sqldb/helpers"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/workpool"
)

const (
	convergeLRPRunsCounter = "ConvergenceLRPRuns"
	convergeLRPDuration    = "ConvergenceLRPDuration"

	domainMetricPrefix = "Domain."

	instanceLRPs  = "LRPsDesired" // this is the number of desired instances
	claimedLRPs   = "LRPsClaimed"
	unclaimedLRPs = "LRPsUnclaimed"
	runningLRPs   = "LRPsRunning"

	missingLRPs = "LRPsMissing"
	extraLRPs   = "LRPsExtra"

	crashedActualLRPs   = "CrashedActualLRPs"
	crashingDesiredLRPs = "CrashingDesiredLRPs"
)

func (db *SQLDB) ConvergeLRPs(logger lager.Logger, cellSet models.CellSet) ([]*auctioneer.LRPStartRequest, []*models.ActualLRPKeyWithSchedulingInfo, []*models.ActualLRPKey, []models.Event) {
	convergeStart := db.clock.Now()
	db.metronClient.IncrementCounter(convergeLRPRunsCounter)
	logger.Info("starting")
	defer logger.Info("completed")

	defer func() {
		err := db.metronClient.SendDuration(convergeLRPDuration, time.Since(convergeStart))
		if err != nil {
			logger.Error("failed-sending-converge-lrp-duration-metric", err)
		}
	}()

	now := db.clock.Now()

	db.pruneDomains(logger, now)
	events := db.pruneEvacuatingActualLRPs(logger, cellSet)
	domainSet, err := db.domainSet(logger)
	if err != nil {
		return nil, nil, nil, nil
	}

	db.emitDomainMetrics(logger, domainSet)

	converge := newConvergence(db)
	converge.staleUnclaimedActualLRPs(logger, now)
	converge.actualLRPsWithMissingCells(logger, cellSet)
	converge.lrpInstanceCounts(logger, domainSet)
	converge.orphanedActualLRPs(logger)
	converge.crashedActualLRPs(logger, now)

	return converge.result(logger), converge.keysWithMissingCells, converge.keysToRetire, events
}

type convergence struct {
	*SQLDB

	guidsToStartRequests map[string]*auctioneer.LRPStartRequest
	startRequestsMutex   sync.Mutex

	keysWithMissingCells []*models.ActualLRPKeyWithSchedulingInfo

	keysToRetire []*models.ActualLRPKey
	keysMutex    sync.Mutex

	pool   *workpool.WorkPool
	poolWg sync.WaitGroup
}

func newConvergence(db *SQLDB) *convergence {
	pool, err := workpool.NewWorkPool(db.convergenceWorkersSize)
	if err != nil {
		panic(fmt.Sprintf("failing to create workpool is irrecoverable %v", err))
	}

	return &convergence{
		SQLDB:                db,
		guidsToStartRequests: map[string]*auctioneer.LRPStartRequest{},
		keysToRetire:         []*models.ActualLRPKey{},
		pool:                 pool,
	}
}

// Adds stale UNCLAIMED Actual LRPs to the list of start requests.
func (c *convergence) staleUnclaimedActualLRPs(logger lager.Logger, now time.Time) {
	logger = logger.Session("stale-unclaimed-actual-lrps")

	rows, err := c.selectStaleUnclaimedLRPs(logger, c.db, now)
	if err != nil {
		logger.Error("failed-query", err)
		return
	}

	for rows.Next() {
		var index int
		schedulingInfo, err := c.fetchDesiredLRPSchedulingInfoAndMore(logger, rows, &index)
		if err == nil {
			c.addStartRequestFromSchedulingInfo(logger, schedulingInfo, index)
			logger.Info("creating-start-request",
				lager.Data{"reason": "stale-unclaimed-lrp", "process_guid": schedulingInfo.ProcessGuid, "index": index})
		}
	}

	if rows.Err() != nil {
		logger.Error("failed-getting-next-row", rows.Err())
	}

	return
}

// Adds CRASHED Actual LRPs that can be restarted to the list of start requests
// and transitions them to UNCLAIMED.
func (c *convergence) crashedActualLRPs(logger lager.Logger, now time.Time) {
	logger = logger.Session("crashed-actual-lrps")
	restartCalculator := models.NewDefaultRestartCalculator()

	rows, err := c.selectCrashedLRPs(logger, c.db)
	if err != nil {
		logger.Error("failed-query", err)
		return
	}

	type crashedActualLRP struct {
		lrpKey         models.ActualLRPKey
		schedulingInfo *models.DesiredLRPSchedulingInfo
		index          int
	}
	lrps := []crashedActualLRP{}

	for rows.Next() {
		var index int
		actual := &models.ActualLRP{}

		schedulingInfo, err := c.fetchDesiredLRPSchedulingInfoAndMore(logger, rows, &index, &actual.Since, &actual.CrashCount)
		if err != nil {
			continue
		}

		actual.ActualLRPKey = models.NewActualLRPKey(schedulingInfo.ProcessGuid, int32(index), schedulingInfo.Domain)
		actual.State = models.ActualLRPStateCrashed

		if actual.ShouldRestartCrash(now, restartCalculator) {
			lrps = append(lrps, crashedActualLRP{
				lrpKey:         actual.ActualLRPKey,
				schedulingInfo: schedulingInfo,
				index:          index,
			})
			logger.Info("creating-start-request",
				lager.Data{"reason": "crashed-instance", "process_guid": actual.ProcessGuid, "index": index})
		}
	}

	for _, lrp := range lrps {
		key := lrp.lrpKey
		schedulingInfo := lrp.schedulingInfo
		index := lrp.index
		c.submit(func() {
			_, _, err := c.UnclaimActualLRP(logger, &key)
			if err != nil {
				logger.Error("failed-unclaiming-actual-lrp", err)
				return
			}

			c.addStartRequestFromSchedulingInfo(logger, schedulingInfo, index)
		})
	}

	if rows.Err() != nil {
		logger.Error("failed-getting-next-row", rows.Err())
	}

	return
}

// Adds orphaned Actual LRPs (ones with no corresponding Desired LRP) to the
// list of keys to retire.
func (c *convergence) orphanedActualLRPs(logger lager.Logger) {
	logger = logger.Session("orphaned-actual-lrps")

	rows, err := c.selectOrphanedActualLRPs(logger, c.db)
	if err != nil {
		logger.Error("failed-query", err)
		return
	}

	for rows.Next() {
		actualLRPKey := &models.ActualLRPKey{}

		err := rows.Scan(
			&actualLRPKey.ProcessGuid,
			&actualLRPKey.Index,
			&actualLRPKey.Domain,
		)
		if err != nil {
			logger.Error("failed-scanning", err)
			continue
		}

		c.addKeyToRetire(logger, actualLRPKey)
	}

	if rows.Err() != nil {
		logger.Error("failed-getting-next-row", rows.Err())
	}
}

// Creates and adds missing Actual LRPs to the list of start requests.
// Adds extra Actual LRPs  to the list of keys to retire.
func (c *convergence) lrpInstanceCounts(logger lager.Logger, domainSet map[string]struct{}) {
	logger = logger.Session("lrp-instance-counts")

	rows, err := c.selectLRPInstanceCounts(logger, c.db)
	if err != nil {
		logger.Error("failed-query", err)
		return
	}

	keys := []models.ActualLRPKey{}

	missingLRPCount := 0
	for rows.Next() {
		var existingIndicesStr sql.NullString
		var actualInstances int

		schedulingInfo, err := c.fetchDesiredLRPSchedulingInfoAndMore(logger, rows, &actualInstances, &existingIndicesStr)
		if err != nil {
			continue
		}

		indices := []int{}
		existingIndices := make(map[int]struct{})
		if existingIndicesStr.String != "" {
			for _, indexStr := range strings.Split(existingIndicesStr.String, ",") {
				index, err := strconv.Atoi(indexStr)
				if err != nil {
					logger.Error("cannot-parse-index", err, lager.Data{
						"index":                indexStr,
						"existing-indeces-str": existingIndicesStr,
					})
					return
				}
				existingIndices[index] = struct{}{}
			}
		}

		for i := 0; i < int(schedulingInfo.Instances); i++ {
			_, found := existingIndices[i]
			if found {
				continue
			}

			missingLRPCount++
			indices = append(indices, i)
			index := int32(i)
			keys = append(keys, models.ActualLRPKey{
				ProcessGuid: schedulingInfo.ProcessGuid,
				Domain:      schedulingInfo.Domain,
				Index:       index,
			})
			logger.Info("creating-start-request",
				lager.Data{"reason": "missing-instance", "process_guid": schedulingInfo.ProcessGuid, "index": index})
		}

		c.addStartRequestFromSchedulingInfo(logger, schedulingInfo, indices...)

		for index := range existingIndices {
			if index < int(schedulingInfo.Instances) {
				continue
			}

			if _, ok := domainSet[schedulingInfo.Domain]; ok {
				c.addKeyToRetire(logger, &models.ActualLRPKey{
					ProcessGuid: schedulingInfo.ProcessGuid,
					Index:       int32(index),
					Domain:      schedulingInfo.Domain,
				})
			}
		}
	}

	for _, key := range keys {
		lrpKey := key
		c.submit(func() {
			_, err := c.CreateUnclaimedActualLRP(logger, &lrpKey)
			if err != nil {
				logger.Error("failed-creating-missing-actual-lrp", err)
			}
		})
	}

	if rows.Err() != nil {
		logger.Error("failed-getting-next-row", rows.Err())
	}

	c.metronClient.SendMetric(missingLRPs, missingLRPCount)
}

// Unclaim Actual LRPs that have missing cells (not in the cell set passed to
// convergence) and add them to the list of start requests.
func (c *convergence) actualLRPsWithMissingCells(logger lager.Logger, cellSet models.CellSet) {
	logger = logger.Session("actual-lrps-with-missing-cells")

	keysWithMissingCells := make([]*models.ActualLRPKeyWithSchedulingInfo, 0)

	rows, err := c.selectLRPsWithMissingCells(logger, c.db, cellSet)
	if err != nil {
		logger.Error("failed-query", err)
		return
	}

	missingCellSet := make(map[string]struct{})
	for rows.Next() {
		var index int32
		var cellID string
		schedulingInfo, err := c.fetchDesiredLRPSchedulingInfoAndMore(logger, rows, &index, &cellID)
		if err == nil {
			keysWithMissingCells = append(keysWithMissingCells, &models.ActualLRPKeyWithSchedulingInfo{
				Key: &models.ActualLRPKey{
					ProcessGuid: schedulingInfo.ProcessGuid,
					Domain:      schedulingInfo.Domain,
					Index:       index,
				},
				SchedulingInfo: schedulingInfo,
			})
		}
		missingCellSet[cellID] = struct{}{}
	}

	if rows.Err() != nil {
		logger.Error("failed-getting-next-row", rows.Err())
	}

	cellIDs := []string{}
	for key, _ := range missingCellSet {
		cellIDs = append(cellIDs, key)
	}

	logger.Info("detected-missing-cells", lager.Data{"cell_ids": cellIDs})

	c.keysWithMissingCells = keysWithMissingCells
}

func (c *convergence) addStartRequestFromSchedulingInfo(logger lager.Logger, schedulingInfo *models.DesiredLRPSchedulingInfo, indices ...int) {
	if len(indices) == 0 {
		return
	}

	c.startRequestsMutex.Lock()
	defer c.startRequestsMutex.Unlock()

	if startRequest, ok := c.guidsToStartRequests[schedulingInfo.ProcessGuid]; ok {
		startRequest.Indices = append(startRequest.Indices, indices...)
		return
	}

	startRequest := auctioneer.NewLRPStartRequestFromSchedulingInfo(schedulingInfo, indices...)
	c.guidsToStartRequests[schedulingInfo.ProcessGuid] = &startRequest
}

func (c *convergence) addKeyToRetire(logger lager.Logger, key *models.ActualLRPKey) {
	c.keysMutex.Lock()
	defer c.keysMutex.Unlock()

	c.keysToRetire = append(c.keysToRetire, key)
}

func (c *convergence) submit(work func()) {
	c.poolWg.Add(1)
	c.pool.Submit(func() {
		defer c.poolWg.Done()
		work()
	})
}

func (c *convergence) result(logger lager.Logger) []*auctioneer.LRPStartRequest {
	c.poolWg.Wait()
	c.pool.Stop()

	c.startRequestsMutex.Lock()
	defer c.startRequestsMutex.Unlock()

	c.keysMutex.Lock()
	defer c.keysMutex.Unlock()

	startRequests := make([]*auctioneer.LRPStartRequest, 0, len(c.guidsToStartRequests))
	for _, startRequest := range c.guidsToStartRequests {
		startRequests = append(startRequests, startRequest)
	}

	c.metronClient.SendMetric(extraLRPs, len(c.keysToRetire))
	c.emitLRPMetrics(logger)

	return startRequests
}

func (db *SQLDB) pruneDomains(logger lager.Logger, now time.Time) {
	logger = logger.Session("prune-domains")

	_, err := db.delete(logger, db.db, domainsTable, "expire_time <= ?", now.UnixNano())
	if err != nil {
		logger.Error("failed-query", err)
	}
}

func (db *SQLDB) pruneEvacuatingActualLRPs(logger lager.Logger, cellSet models.CellSet) []models.Event {
	logger = logger.Session("prune-evacuating-actual-lrps")

	wheres := []string{"evacuating = ?"}
	bindings := []interface{}{true}

	if len(cellSet) > 0 {
		wheres = append(wheres, fmt.Sprintf("actual_lrps.cell_id NOT IN (%s)", helpers.QuestionMarks(len(cellSet))))

		for cellID := range cellSet {
			bindings = append(bindings, cellID)
		}
	}

	lrpsToDelete, err := db.getActualLRPS(logger, strings.Join(wheres, " AND "), bindings...)
	if err != nil {
		logger.Error("failed-fetching-evacuating-lrps-with-missing-cells", err)
	}

	_, err = db.delete(logger, db.db, actualLRPsTable, strings.Join(wheres, " AND "), bindings...)
	if err != nil {
		logger.Error("failed-query", err)
	}

	events := []models.Event{}
	for _, lrp := range lrpsToDelete {
		events = append(events, models.NewActualLRPRemovedEvent(lrp))
	}
	return events
}

func (db *SQLDB) domainSet(logger lager.Logger) (map[string]struct{}, error) {
	logger.Debug("listing-domains")
	domains, err := db.Domains(logger)
	if err != nil {
		logger.Error("failed-listing-domains", err)
		return nil, err
	}
	logger.Debug("succeeded-listing-domains")
	m := make(map[string]struct{}, len(domains))
	for _, domain := range domains {
		m[domain] = struct{}{}
	}
	return m, nil
}

func (db *SQLDB) emitDomainMetrics(logger lager.Logger, domainSet map[string]struct{}) {
	for domain := range domainSet {
		db.metronClient.SendMetric("Domain."+domain, 1)
	}
}

func (db *SQLDB) emitLRPMetrics(logger lager.Logger) {
	var err error
	logger = logger.Session("emit-lrp-metrics")
	claimedInstances, unclaimedInstances, runningInstances, crashedInstances, crashingDesireds := db.countActualLRPsByState(logger, db.db)

	desiredInstances := db.countDesiredInstances(logger, db.db)

	err = db.metronClient.SendMetric(unclaimedLRPs, unclaimedInstances)
	if err != nil {
		logger.Error("failed-sending-unclaimed-lrps-metric", err)
	}

	db.metronClient.SendMetric(claimedLRPs, claimedInstances)
	if err != nil {
		logger.Error("failed-sending-claimed-lrps-metric", err)
	}

	err = db.metronClient.SendMetric(runningLRPs, runningInstances)
	if err != nil {
		logger.Error("failed-sending-running-lrps-metric", err)
	}

	err = db.metronClient.SendMetric(crashedActualLRPs, crashedInstances)
	if err != nil {
		logger.Error("failed-sending-crashed-actual-lrps-metric", err)
	}

	err = db.metronClient.SendMetric(crashingDesiredLRPs, crashingDesireds)
	if err != nil {
		logger.Error("failed-sending-crashing-desired-lrps-metric", err)
	}

	err = db.metronClient.SendMetric(instanceLRPs, desiredInstances)
	if err != nil {
		logger.Error("failed-sending-desired-lrps-metric", err)
	}
}

func (db *SQLDB) GatherAndPruneLRPs(logger lager.Logger, cellSet models.CellSet) (*models.ConvergenceInput, error) {
	panic("not implemented")
}
