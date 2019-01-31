package sqldb

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"code.cloudfoundry.org/bbs/db"
	"code.cloudfoundry.org/bbs/db/sqldb/helpers"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"
)

func (sqldb *SQLDB) ConvergeLRPs(logger lager.Logger, cellSet models.CellSet) db.ConvergenceResult {
	logger.Info("starting")
	defer logger.Info("completed")

	now := sqldb.clock.Now()
	sqldb.pruneDomains(logger, now)
	events, instanceEvents := sqldb.pruneEvacuatingActualLRPs(logger, cellSet)
	domainSet, err := sqldb.domainSet(logger)
	if err != nil {
		return db.ConvergenceResult{}
	}

	converge := newConvergence(sqldb)
	converge.staleUnclaimedActualLRPs(logger, now)
	converge.actualLRPsWithMissingCells(logger, cellSet)
	converge.lrpInstanceCounts(logger, domainSet)
	converge.orphanedActualLRPs(logger)
	converge.orphanedSuspectActualLRPs(logger)
	converge.extraSuspectActualLRPs(logger)
	converge.suspectActualLRPsWithExistingCells(logger, cellSet)
	converge.suspectRunningActualLRPs(logger)
	converge.suspectClaimedActualLRPs(logger)
	converge.crashedActualLRPs(logger, now)

	return db.ConvergenceResult{
		MissingLRPKeys:               converge.missingLRPKeys,
		UnstartedLRPKeys:             converge.unstartedLRPKeys,
		KeysToRetire:                 converge.keysToRetire,
		SuspectLRPKeysToRetire:       converge.suspectKeysToRetire,
		KeysWithMissingCells:         converge.ordinaryKeysWithMissingCells,
		MissingCellIds:               converge.missingCellIds,
		Events:                       events,
		InstanceEvents:               instanceEvents,
		SuspectKeysWithExistingCells: converge.suspectKeysWithExistingCells,
		SuspectRunningKeys:           converge.suspectRunningKeys,
		SuspectClaimedKeys:           converge.suspectClaimedKeys,
	}
}

type convergence struct {
	*SQLDB

	ordinaryKeysWithMissingCells []*models.ActualLRPKeyWithSchedulingInfo
	missingCellIds               []string
	suspectKeysWithExistingCells []*models.ActualLRPKey

	suspectKeysToRetire []*models.ActualLRPKey

	suspectRunningKeys []*models.ActualLRPKey
	suspectClaimedKeys []*models.ActualLRPKey

	keysToRetire []*models.ActualLRPKey

	missingLRPKeys []*models.ActualLRPKeyWithSchedulingInfo

	unstartedLRPKeys []*models.ActualLRPKeyWithSchedulingInfo
}

func newConvergence(db *SQLDB) *convergence {
	return &convergence{
		SQLDB: db,
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
		if err != nil {
			continue
		}
		key := models.NewActualLRPKey(schedulingInfo.ProcessGuid, int32(index), schedulingInfo.Domain)
		c.unstartedLRPKeys = append(c.unstartedLRPKeys, &models.ActualLRPKeyWithSchedulingInfo{
			Key:            &key,
			SchedulingInfo: schedulingInfo,
		})
		logger.Info("creating-start-request",
			lager.Data{"reason": "stale-unclaimed-lrp", "process_guid": schedulingInfo.ProcessGuid, "index": index})
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
			c.unstartedLRPKeys = append(c.unstartedLRPKeys, &models.ActualLRPKeyWithSchedulingInfo{
				Key:            &actual.ActualLRPKey,
				SchedulingInfo: schedulingInfo,
			})
			logger.Info("creating-start-request",
				lager.Data{"reason": "crashed-instance", "process_guid": actual.ProcessGuid, "index": index})
		}
	}

	if rows.Err() != nil {
		logger.Error("failed-getting-next-row", rows.Err())
	}

	return
}

func scanActualLRPs(logger lager.Logger, rows *sql.Rows) []*models.ActualLRPKey {
	var actualLRPKeys []*models.ActualLRPKey
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

		actualLRPKeys = append(actualLRPKeys, actualLRPKey)
	}

	if rows.Err() != nil {
		logger.Error("failed-getting-next-row", rows.Err())
	}
	return actualLRPKeys
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

	c.keysToRetire = append(c.keysToRetire, scanActualLRPs(logger, rows)...)
}

func (c *convergence) extraSuspectActualLRPs(logger lager.Logger) {
	logger = logger.Session("extra-suspect-lrps")

	rows, err := c.selectExtraSuspectActualLRPs(logger, c.db)
	if err != nil {
		logger.Error("failed-query", err)
		return
	}

	c.suspectKeysToRetire = append(c.suspectKeysToRetire, scanActualLRPs(logger, rows)...)
}

func (c *convergence) orphanedSuspectActualLRPs(logger lager.Logger) {
	logger = logger.Session("orphaned-suspect-lrps")

	rows, err := c.selectOrphanedSuspectActualLRPs(logger, c.db)
	if err != nil {
		logger.Error("failed-query", err)
		return
	}

	c.suspectKeysToRetire = append(c.suspectKeysToRetire, scanActualLRPs(logger, rows)...)
}

func (c *convergence) suspectRunningActualLRPs(logger lager.Logger) {
	logger = logger.Session("suspect-running-lrps")

	rows, err := c.selectSuspectRunningActualLRPs(logger, c.db)
	if err != nil {
		logger.Error("failed-query", err)
		return
	}

	c.suspectRunningKeys = scanActualLRPs(logger, rows)
}

func (c *convergence) suspectClaimedActualLRPs(logger lager.Logger) {
	logger = logger.Session("suspect-running-lrps")

	rows, err := c.selectSuspectClaimedActualLRPs(logger, c.db)
	if err != nil {
		logger.Error("failed-query", err)
		return
	}

	c.suspectClaimedKeys = scanActualLRPs(logger, rows)
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
						"existing-indices-str": existingIndicesStr,
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

			indices = append(indices, i)
			index := int32(i)
			c.missingLRPKeys = append(c.missingLRPKeys, &models.ActualLRPKeyWithSchedulingInfo{
				Key: &models.ActualLRPKey{
					ProcessGuid: schedulingInfo.ProcessGuid,
					Domain:      schedulingInfo.Domain,
					Index:       index,
				},
				SchedulingInfo: schedulingInfo,
			})
			logger.Info("creating-start-request",
				lager.Data{"reason": "missing-instance", "process_guid": schedulingInfo.ProcessGuid, "index": index})
		}

		for index := range existingIndices {
			if index < int(schedulingInfo.Instances) {
				continue
			}

			// only take destructive actions for fresh domains
			if _, ok := domainSet[schedulingInfo.Domain]; ok {
				c.keysToRetire = append(c.keysToRetire, &models.ActualLRPKey{
					ProcessGuid: schedulingInfo.ProcessGuid,
					Index:       int32(index),
					Domain:      schedulingInfo.Domain,
				})
			}
		}
	}

	if rows.Err() != nil {
		logger.Error("failed-getting-next-row", rows.Err())
	}
}

// Unclaim Actual LRPs that have missing cells (not in the cell set passed to
// convergence) and add them to the list of start requests.
func (c *convergence) suspectActualLRPsWithExistingCells(logger lager.Logger, cellSet models.CellSet) {
	logger = logger.Session("suspect-lrps-with-existing-cells")

	if len(cellSet) == 0 {
		return
	}

	rows, err := c.selectSuspectLRPsWithExistingCells(logger, c.db, cellSet)
	if err != nil {
		logger.Error("failed-query", err)
		return
	}

	c.suspectKeysWithExistingCells = scanActualLRPs(logger, rows)
}

// Unclaim Actual LRPs that have missing cells (not in the cell set passed to
// convergence) and add them to the list of start requests.
func (c *convergence) actualLRPsWithMissingCells(logger lager.Logger, cellSet models.CellSet) {
	logger = logger.Session("actual-lrps-with-missing-cells")

	var ordinaryKeysWithMissingCells []*models.ActualLRPKeyWithSchedulingInfo

	rows, err := c.selectLRPsWithMissingCells(logger, c.db, cellSet)
	if err != nil {
		logger.Error("failed-query", err)
		return
	}

	missingCellSet := make(map[string]struct{})
	for rows.Next() {
		var index int32
		var cellID string
		var presence models.ActualLRP_Presence
		schedulingInfo, err := c.fetchDesiredLRPSchedulingInfoAndMore(logger, rows, &index, &cellID, &presence)
		if err == nil && presence == models.ActualLRP_Ordinary {
			ordinaryKeysWithMissingCells = append(ordinaryKeysWithMissingCells, &models.ActualLRPKeyWithSchedulingInfo{
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

	for key, _ := range missingCellSet {
		c.missingCellIds = append(c.missingCellIds, key)
	}

	if len(c.missingCellIds) > 0 {
		logger.Info("detected-missing-cells", lager.Data{"cell_ids": c.missingCellIds})
	}

	c.ordinaryKeysWithMissingCells = ordinaryKeysWithMissingCells
}

func (db *SQLDB) pruneDomains(logger lager.Logger, now time.Time) {
	logger = logger.Session("prune-domains")

	err := db.transact(logger, func(logger lager.Logger, tx helpers.Tx) error {
		domains, err := db.domains(logger, tx, time.Time{})
		if err != nil {
			return err
		}

		for _, d := range domains {
			if d.expiresAt.After(now) {
				continue
			}

			logger.Info("pruning-domain", lager.Data{"domain": d.name, "expire-at": d.expiresAt})
			_, err := db.delete(logger, tx, domainsTable, "domain = ? ", d.name)
			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		logger.Error("cannot-prune-domains", err)
	}
}

func (db *SQLDB) pruneEvacuatingActualLRPs(logger lager.Logger, cellSet models.CellSet) ([]models.Event, []models.Event) {
	logger = logger.Session("prune-evacuating-actual-lrps")

	wheres := []string{"presence = ?"}
	bindings := []interface{}{models.ActualLRP_Evacuating}

	if len(cellSet) > 0 {
		wheres = append(wheres, fmt.Sprintf("actual_lrps.cell_id NOT IN (%s)", helpers.QuestionMarks(len(cellSet))))

		for cellID := range cellSet {
			bindings = append(bindings, cellID)
		}
	}

	lrpsToDelete, err := db.getActualLRPs(logger, strings.Join(wheres, " AND "), bindings...)
	if err != nil {
		logger.Error("failed-fetching-evacuating-lrps-with-missing-cells", err)
	}

	_, err = db.delete(logger, db.db, actualLRPsTable, strings.Join(wheres, " AND "), bindings...)
	if err != nil {
		logger.Error("failed-query", err)
	}

	var events []models.Event
	var instanceEvents []models.Event
	for _, lrp := range lrpsToDelete {
		events = append(events, models.NewActualLRPRemovedEvent(lrp.ToActualLRPGroup()))
		instanceEvents = append(instanceEvents, models.NewActualLRPInstanceRemovedEvent(lrp))
	}
	return events, instanceEvents
}

func (db *SQLDB) domainSet(logger lager.Logger) (map[string]struct{}, error) {
	logger.Debug("listing-domains")
	domains, err := db.FreshDomains(logger)
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
