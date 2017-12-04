package etcd

import (
	"path"
	"sync"
	"sync/atomic"
	"time"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"
	"github.com/coreos/go-etcd/etcd"
	"github.com/nu7hatch/gouuid"
)

type stateChange bool

const (
	stateDidNotChange stateChange = false
	stateDidChange    stateChange = false
)

func (db *ETCDDB) ActualLRPGroups(logger lager.Logger, filter models.ActualLRPFilter) ([]*models.ActualLRPGroup, error) {
	node, err := db.fetchRecursiveRaw(logger, ActualLRPSchemaRoot)
	bbsErr := models.ConvertError(err)
	if bbsErr != nil {
		if bbsErr.Type == models.Error_ResourceNotFound {
			return []*models.ActualLRPGroup{}, nil
		}
		return nil, err
	}
	if len(node.Nodes) == 0 {
		return []*models.ActualLRPGroup{}, nil
	}

	groups := []*models.ActualLRPGroup{}

	var workErr atomic.Value
	groupChan := make(chan []*models.ActualLRPGroup, len(node.Nodes))
	wg := sync.WaitGroup{}

	logger.Debug("performing-deserialization-work")
	for _, node := range node.Nodes {
		node := node

		wg.Add(1)
		go func() {
			defer wg.Done()
			g, err := db.parseActualLRPGroups(logger, node, filter)
			if err != nil {
				workErr.Store(err)
				return
			}
			groupChan <- g
		}()
	}

	go func() {
		wg.Wait()
		close(groupChan)
	}()

	for g := range groupChan {
		groups = append(groups, g...)
	}

	if err, ok := workErr.Load().(error); ok {
		logger.Error("failed-performing-deserialization-work", err)
		return []*models.ActualLRPGroup{}, models.ErrUnknownError
	}
	logger.Debug("succeeded-performing-deserialization-work", lager.Data{"num_actual_lrp_groups": len(groups)})

	return groups, nil
}

func (db *ETCDDB) ActualLRPGroupsByProcessGuid(logger lager.Logger, processGuid string) ([]*models.ActualLRPGroup, error) {
	node, err := db.fetchRecursiveRaw(logger, ActualLRPProcessDir(processGuid))
	bbsErr := models.ConvertError(err)
	if bbsErr != nil {
		if bbsErr.Type == models.Error_ResourceNotFound {
			return []*models.ActualLRPGroup{}, nil
		}
		return nil, err
	}
	if node.Nodes.Len() == 0 {
		return []*models.ActualLRPGroup{}, nil
	}

	return db.parseActualLRPGroups(logger, node, models.ActualLRPFilter{})
}

func (db *ETCDDB) ActualLRPGroupByProcessGuidAndIndex(logger lager.Logger, processGuid string, index int32) (*models.ActualLRPGroup, error) {
	group, err := db.rawActualLRPGroupByProcessGuidAndIndex(logger, processGuid, index)
	return group, err
}

func (db *ETCDDB) CreateUnclaimedActualLRP(logger lager.Logger, key *models.ActualLRPKey) (*models.ActualLRPGroup, error) {
	lrp, err := db.newUnclaimedActualLRP(key)
	if err != nil {
		return nil, models.ErrActualLRPCannotBeUnclaimed
	}

	return &models.ActualLRPGroup{Instance: lrp}, db.createRawActualLRP(logger, lrp)
}

func (db *ETCDDB) UnclaimActualLRP(logger lager.Logger, key *models.ActualLRPKey) (*models.ActualLRPGroup, *models.ActualLRPGroup, error) {
	actualLRP, modifiedIndex, err := db.rawActualLRPByProcessGuidAndIndex(logger, key.ProcessGuid, key.Index)
	bbsErr := models.ConvertError(err)
	if bbsErr != nil {
		return nil, nil, bbsErr
	}
	beforeActualLRP := *actualLRP

	if actualLRP.State == models.ActualLRPStateUnclaimed {
		logger.Debug("already-unclaimed")
		return nil, nil, models.ErrActualLRPCannotBeUnclaimed
	}

	actualLRP.State = models.ActualLRPStateUnclaimed
	actualLRP.ActualLRPKey = *key
	actualLRP.ActualLRPInstanceKey = models.ActualLRPInstanceKey{}
	actualLRP.ActualLRPNetInfo = models.EmptyActualLRPNetInfo()
	actualLRP.Since = db.clock.Now().UnixNano()
	actualLRP.ModificationTag.Increment()

	data, err := db.serializeModel(logger, actualLRP)
	if err != nil {
		return nil, nil, err
	}

	_, err = db.client.CompareAndSwap(ActualLRPSchemaPath(key.ProcessGuid, key.Index), data, 0, modifiedIndex)
	if err != nil {
		logger.Error("failed-compare-and-swap", err)
		return nil, nil, ErrorFromEtcdError(logger, err)
	}

	return &models.ActualLRPGroup{Instance: &beforeActualLRP}, &models.ActualLRPGroup{Instance: actualLRP}, nil
}

func (db *ETCDDB) ClaimActualLRP(logger lager.Logger, processGuid string, index int32, instanceKey *models.ActualLRPInstanceKey) (*models.ActualLRPGroup, *models.ActualLRPGroup, error) {
	logger = logger.WithData(lager.Data{"process_guid": processGuid, "index": index, "actual_lrp_instance_key": instanceKey})
	logger.Info("starting")

	lrp, prevIndex, err := db.rawActualLRPByProcessGuidAndIndex(logger, processGuid, index)
	if err != nil {
		logger.Error("failed", err)
		return nil, nil, err
	}
	beforeActualLRP := *lrp

	if !lrp.AllowsTransitionTo(&lrp.ActualLRPKey, instanceKey, models.ActualLRPStateClaimed) {
		return nil, nil, models.ErrActualLRPCannotBeClaimed
	}

	if lrp.State == models.ActualLRPStateClaimed && lrp.ActualLRPInstanceKey.Equal(instanceKey) {
		return &models.ActualLRPGroup{Instance: &beforeActualLRP}, &models.ActualLRPGroup{Instance: lrp}, nil
	}

	lrp.PlacementError = ""
	lrp.State = models.ActualLRPStateClaimed
	lrp.ActualLRPInstanceKey = *instanceKey
	lrp.ActualLRPNetInfo = models.ActualLRPNetInfo{}
	lrp.ModificationTag.Increment()
	lrp.Since = db.clock.Now().UnixNano()

	err = lrp.Validate()
	if err != nil {
		logger.Error("failed", err)
		return nil, nil, models.NewError(models.Error_InvalidRecord, err.Error())
	}

	lrpData, serializeErr := db.serializeModel(logger, lrp)
	if serializeErr != nil {
		return nil, nil, serializeErr
	}

	_, err = db.client.CompareAndSwap(ActualLRPSchemaPath(processGuid, index), lrpData, 0, prevIndex)
	if err != nil {
		logger.Error("compare-and-swap-failed", err)
		return nil, nil, models.ErrActualLRPCannotBeClaimed
	}
	logger.Info("succeeded")

	return &models.ActualLRPGroup{Instance: &beforeActualLRP}, &models.ActualLRPGroup{Instance: lrp}, nil
}

func (db *ETCDDB) StartActualLRP(logger lager.Logger, key *models.ActualLRPKey, instanceKey *models.ActualLRPInstanceKey, netInfo *models.ActualLRPNetInfo) (*models.ActualLRPGroup, *models.ActualLRPGroup, error) {
	logger = logger.WithData(lager.Data{
		"actual_lrp_key":          key,
		"actual_lrp_instance_key": instanceKey,
		"net_info":                netInfo,
	})

	lrp, prevIndex, err := db.rawActualLRPByProcessGuidAndIndex(logger, key.ProcessGuid, key.Index)
	bbsErr := models.ConvertError(err)
	if bbsErr != nil {
		if bbsErr.Type == models.Error_ResourceNotFound {
			lrp, err := db.createRunningActualLRP(logger, key, instanceKey, netInfo)
			return nil, &models.ActualLRPGroup{Instance: lrp}, err
		}
		logger.Error("failed-to-get-actual-lrp", err)
		return nil, nil, err
	}
	beforeActualLRP := *lrp

	if lrp.ActualLRPKey.Equal(key) &&
		lrp.ActualLRPInstanceKey.Equal(instanceKey) &&
		lrp.ActualLRPNetInfo.Equal(netInfo) &&
		lrp.State == models.ActualLRPStateRunning {
		lrpGroup := &models.ActualLRPGroup{Instance: lrp}
		return lrpGroup, lrpGroup, nil
	}

	if !lrp.AllowsTransitionTo(key, instanceKey, models.ActualLRPStateRunning) {
		logger.Error("failed-to-transition-actual-lrp-to-started", nil)
		return nil, nil, models.ErrActualLRPCannotBeStarted
	}

	logger.Info("starting")
	defer logger.Info("completed")

	lrp.ModificationTag.Increment()
	lrp.State = models.ActualLRPStateRunning
	lrp.Since = db.clock.Now().UnixNano()
	lrp.ActualLRPInstanceKey = *instanceKey
	lrp.ActualLRPNetInfo = *netInfo
	lrp.PlacementError = ""

	lrpData, serializeErr := db.serializeModel(logger, lrp)
	if serializeErr != nil {
		return nil, nil, serializeErr
	}

	_, err = db.client.CompareAndSwap(ActualLRPSchemaPath(key.ProcessGuid, key.Index), lrpData, 0, prevIndex)
	if err != nil {
		logger.Error("failed", err)
		return nil, nil, models.ErrActualLRPCannotBeStarted
	}

	return &models.ActualLRPGroup{Instance: &beforeActualLRP}, &models.ActualLRPGroup{Instance: lrp}, nil
}

func (db *ETCDDB) CrashActualLRP(logger lager.Logger, key *models.ActualLRPKey, instanceKey *models.ActualLRPInstanceKey, errorMessage string) (*models.ActualLRPGroup, *models.ActualLRPGroup, bool, error) {
	logger = logger.WithData(lager.Data{"actual_lrp_key": key, "actual_lrp_instance_key": instanceKey})
	logger.Info("starting")

	lrp, prevIndex, err := db.rawActualLRPByProcessGuidAndIndex(logger, key.ProcessGuid, key.Index)
	if err != nil {
		logger.Error("failed-to-get-actual-lrp", err)
		return nil, nil, false, err
	}
	beforeActualLRP := *lrp

	latestChangeTime := time.Duration(db.clock.Now().UnixNano() - lrp.Since)

	var newCrashCount int32
	if latestChangeTime > models.CrashResetTimeout && lrp.State == models.ActualLRPStateRunning {
		newCrashCount = 1
	} else {
		newCrashCount = lrp.CrashCount + 1
	}

	logger.Debug("retrieved-lrp")
	if !lrp.AllowsTransitionTo(key, instanceKey, models.ActualLRPStateCrashed) {
		logger.Error("failed-to-transition-to-crashed", nil, lager.Data{"from_state": lrp.State, "same_instance_key": lrp.ActualLRPInstanceKey.Equal(instanceKey)})
		return nil, nil, false, models.ErrActualLRPCannotBeCrashed
	}

	lrp.State = models.ActualLRPStateCrashed
	lrp.Since = db.clock.Now().UnixNano()
	lrp.CrashCount = newCrashCount
	lrp.ActualLRPInstanceKey = models.ActualLRPInstanceKey{}
	lrp.ActualLRPNetInfo = models.EmptyActualLRPNetInfo()
	lrp.ModificationTag.Increment()
	lrp.CrashReason = errorMessage

	var immediateRestart bool
	if lrp.ShouldRestartImmediately(models.NewDefaultRestartCalculator()) {
		lrp.State = models.ActualLRPStateUnclaimed
		immediateRestart = true
	}

	lrpData, serializeErr := db.serializeModel(logger, lrp)
	if serializeErr != nil {
		return nil, nil, false, serializeErr
	}

	_, err = db.client.CompareAndSwap(ActualLRPSchemaPath(key.ProcessGuid, key.Index), lrpData, 0, prevIndex)
	if err != nil {
		logger.Error("failed", err)
		return nil, nil, false, models.ErrActualLRPCannotBeCrashed
	}

	logger.Info("succeeded")
	return &models.ActualLRPGroup{Instance: &beforeActualLRP}, &models.ActualLRPGroup{Instance: lrp}, immediateRestart, nil
}

func (db *ETCDDB) FailActualLRP(logger lager.Logger, key *models.ActualLRPKey, errorMessage string) (*models.ActualLRPGroup, *models.ActualLRPGroup, error) {
	logger = logger.WithData(lager.Data{"actual_lrp_key": key, "error_message": errorMessage})
	logger.Info("starting")
	lrp, prevIndex, err := db.rawActualLRPByProcessGuidAndIndex(logger, key.ProcessGuid, key.Index)
	if err != nil {
		logger.Error("failed-to-get-actual-lrp", err)
		return nil, nil, err
	}
	beforeActualLRP := *lrp

	if lrp.State != models.ActualLRPStateUnclaimed {
		return nil, nil, models.ErrActualLRPCannotBeFailed
	}

	lrp.ModificationTag.Increment()
	lrp.PlacementError = errorMessage
	lrp.Since = db.clock.Now().UnixNano()

	lrpData, serialErr := db.serializeModel(logger, lrp)
	if serialErr != nil {
		return nil, nil, serialErr
	}

	_, err = db.client.CompareAndSwap(ActualLRPSchemaPath(key.ProcessGuid, key.Index), lrpData, 0, prevIndex)
	if err != nil {
		logger.Error("failed", err)
		return nil, nil, models.ErrActualLRPCannotBeFailed
	}

	logger.Info("succeeded")
	return &models.ActualLRPGroup{Instance: &beforeActualLRP}, &models.ActualLRPGroup{Instance: lrp}, nil
}

func (db *ETCDDB) RemoveActualLRP(logger lager.Logger, processGuid string, index int32, instanceKey *models.ActualLRPInstanceKey) error {
	logger = logger.WithData(lager.Data{"process_guid": processGuid, "index": index, "instance_key": instanceKey})

	lrp, prevIndex, err := db.rawActualLRPByProcessGuidAndIndex(logger, processGuid, index)
	if err != nil {
		return err
	}

	if instanceKey != nil && !lrp.ActualLRPInstanceKey.Equal(instanceKey) {
		logger.Debug("not-found", lager.Data{"actual_lrp_instance_key": lrp.ActualLRPInstanceKey, "instance_key": instanceKey})
		return models.ErrResourceNotFound
	}

	return db.removeActualLRP(logger, lrp, prevIndex)
}

func (db *ETCDDB) removeActualLRP(logger lager.Logger, lrp *models.ActualLRP, prevIndex uint64) error {
	logger.Info("starting")
	_, err := db.client.CompareAndDelete(ActualLRPSchemaPath(lrp.ProcessGuid, lrp.Index), prevIndex)
	if err != nil {
		logger.Error("failed", err)
		return models.ErrActualLRPCannotBeRemoved
	}
	logger.Info("succeeded")
	return nil
}

func (db *ETCDDB) parseActualLRPGroups(logger lager.Logger, node *etcd.Node, filter models.ActualLRPFilter) ([]*models.ActualLRPGroup, error) {
	var groups = []*models.ActualLRPGroup{}

	logger.Debug("performing-parsing-actual-lrp-groups")
	for _, indexNode := range node.Nodes {
		group := &models.ActualLRPGroup{}
		for _, instanceNode := range indexNode.Nodes {
			var lrp models.ActualLRP
			deserializeErr := db.deserializeModel(logger, instanceNode, &lrp)
			if deserializeErr != nil {
				logger.Error("failed-parsing-actual-lrp-groups", deserializeErr, lager.Data{"key": instanceNode.Key})
				return []*models.ActualLRPGroup{}, deserializeErr
			}
			if filter.Domain != "" && lrp.Domain != filter.Domain {
				continue
			}
			if filter.CellID != "" && lrp.CellId != filter.CellID {
				continue
			}

			if isInstanceActualLRPNode(instanceNode) {
				group.Instance = &lrp
			}

			if isEvacuatingActualLRPNode(instanceNode) {
				group.Evacuating = &lrp
			}
		}

		if group.Instance != nil || group.Evacuating != nil {
			groups = append(groups, group)
		}
	}
	logger.Debug("succeeded-performing-parsing-actual-lrp-groups", lager.Data{"num_actual_lrp_groups": len(groups)})

	return groups, nil
}

func (db *ETCDDB) unclaimActualLRP(
	logger lager.Logger,
	actualLRPKey *models.ActualLRPKey,
	actualLRPInstanceKey *models.ActualLRPInstanceKey,
) (stateChange, error) {
	logger = logger.Session("unclaim-actual-lrp")
	logger.Info("starting")

	lrp, storeIndex, err := db.rawActualLRPByProcessGuidAndIndex(logger, actualLRPKey.ProcessGuid, actualLRPKey.Index)
	if err != nil {
		return stateDidNotChange, err
	}

	changed, err := db.unclaimActualLRPWithIndex(logger, lrp, storeIndex, actualLRPKey, actualLRPInstanceKey)
	if err != nil {
		return changed, err
	}

	logger.Info("succeeded")
	return changed, nil
}

func (db *ETCDDB) unclaimActualLRPWithIndex(
	logger lager.Logger,
	lrp *models.ActualLRP,
	storeIndex uint64,
	actualLRPKey *models.ActualLRPKey,
	actualLRPInstanceKey *models.ActualLRPInstanceKey,
) (change stateChange, err error) {
	logger = logger.Session("unclaim-actual-lrp-with-index")
	defer logger.Debug("complete", lager.Data{"state_change": change, "error": err})

	if !lrp.ActualLRPKey.Equal(actualLRPKey) {
		logger.Error("failed-actual-lrp-key-differs", models.ErrActualLRPCannotBeUnclaimed)
		return stateDidNotChange, models.ErrActualLRPCannotBeUnclaimed
	}

	if lrp.State == models.ActualLRPStateUnclaimed {
		logger.Info("already-unclaimed")
		return stateDidNotChange, nil
	}

	if !lrp.ActualLRPInstanceKey.Equal(actualLRPInstanceKey) {
		logger.Error("failed-actual-lrp-instance-key-differs", models.ErrActualLRPCannotBeUnclaimed)
		return stateDidNotChange, models.ErrActualLRPCannotBeUnclaimed
	}

	lrp.Since = db.clock.Now().UnixNano()
	lrp.State = models.ActualLRPStateUnclaimed
	lrp.ActualLRPInstanceKey = models.ActualLRPInstanceKey{}
	lrp.ActualLRPNetInfo = models.EmptyActualLRPNetInfo()
	lrp.ModificationTag.Increment()

	err = lrp.Validate()
	if err != nil {
		logger.Error("failed-to-validate-unclaimed-lrp", err)
		return stateDidNotChange, models.NewError(models.Error_InvalidRecord, err.Error())
	}

	lrpData, serialErr := db.serializeModel(logger, lrp)
	if serialErr != nil {
		logger.Error("failed-to-marshal-unclaimed-lrp", serialErr)
		return stateDidNotChange, serialErr
	}

	_, err = db.client.CompareAndSwap(ActualLRPSchemaPath(actualLRPKey.ProcessGuid, actualLRPKey.Index), lrpData, 0, storeIndex)
	if err != nil {
		logger.Error("failed-to-compare-and-swap", err)
		return stateDidNotChange, models.ErrActualLRPCannotBeUnclaimed
	}

	logger.Debug("changed-to-unclaimed")
	return stateDidChange, nil
}

func (db *ETCDDB) rawActualLRPGroupByProcessGuidAndIndex(logger lager.Logger, processGuid string, index int32) (*models.ActualLRPGroup, error) {
	node, err := db.fetchRecursiveRaw(logger, ActualLRPIndexDir(processGuid, index))
	if err != nil {
		return nil, err
	}

	group := models.ActualLRPGroup{}
	for _, instanceNode := range node.Nodes {
		var lrp models.ActualLRP
		deserializeErr := db.deserializeModel(logger, instanceNode, &lrp)
		if deserializeErr != nil {
			logger.Error("failed-parsing-actual-lrp", deserializeErr, lager.Data{"key": instanceNode.Key})
			return nil, deserializeErr
		}

		if isInstanceActualLRPNode(instanceNode) {
			group.Instance = &lrp
		}

		if isEvacuatingActualLRPNode(instanceNode) {
			group.Evacuating = &lrp
		}
	}

	if group.Evacuating == nil && group.Instance == nil {
		return nil, models.ErrResourceNotFound
	}

	return &group, nil
}

func (db *ETCDDB) rawActualLRPByProcessGuidAndIndex(logger lager.Logger, processGuid string, index int32) (*models.ActualLRP, uint64, error) {
	logger.Debug("raw-actual-lrp-by-process-guid-and-index")
	node, err := db.fetchRaw(logger, ActualLRPSchemaPath(processGuid, index))
	if err != nil {
		return nil, 0, err
	}

	lrp := new(models.ActualLRP)
	deserializeErr := db.deserializeModel(logger, node, lrp)
	if deserializeErr != nil {
		return nil, 0, deserializeErr
	}

	return lrp, node.ModifiedIndex, nil
}

func isInstanceActualLRPNode(node *etcd.Node) bool {
	return path.Base(node.Key) == ActualLRPInstanceKey
}

func isEvacuatingActualLRPNode(node *etcd.Node) bool {
	return path.Base(node.Key) == ActualLRPEvacuatingKey
}

func (db *ETCDDB) createActualLRP(logger lager.Logger, desiredLRP *models.DesiredLRP, index int32) error {
	logger = logger.Session("create-actual-lrp")
	var err error
	if index >= desiredLRP.Instances {
		err = models.NewError(models.Error_InvalidRecord, "Index too large")
		logger.Error("actual-lrp-index-too-large", err, lager.Data{"actual_index": index, "desired_instances": desiredLRP.Instances})
		return err
	}

	guid, err := uuid.NewV4()
	if err != nil {
		return err
	}

	actualLRP := &models.ActualLRP{
		ActualLRPKey: models.NewActualLRPKey(
			desiredLRP.ProcessGuid,
			index,
			desiredLRP.Domain,
		),
		State: models.ActualLRPStateUnclaimed,
		Since: db.clock.Now().UnixNano(),
		ModificationTag: models.ModificationTag{
			Epoch: guid.String(),
			Index: 0,
		},
	}

	err = db.createRawActualLRP(logger, actualLRP)
	if err != nil {
		return err
	}

	return nil
}

func (db *ETCDDB) newRunningActualLRP(key *models.ActualLRPKey, instanceKey *models.ActualLRPInstanceKey, netInfo *models.ActualLRPNetInfo) (*models.ActualLRP, error) {
	guid, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}

	return &models.ActualLRP{
		ActualLRPKey:         *key,
		ActualLRPInstanceKey: *instanceKey,
		ActualLRPNetInfo:     *netInfo,
		Since:                db.clock.Now().UnixNano(),
		State:                models.ActualLRPStateRunning,
		ModificationTag: models.ModificationTag{
			Epoch: guid.String(),
			Index: 0,
		},
	}, nil
}

func (db *ETCDDB) newUnclaimedActualLRP(key *models.ActualLRPKey) (*models.ActualLRP, error) {
	guid, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}

	return &models.ActualLRP{
		ActualLRPKey: *key,
		Since:        db.clock.Now().UnixNano(),
		State:        models.ActualLRPStateUnclaimed,
		ModificationTag: models.ModificationTag{
			Epoch: guid.String(),
			Index: 0,
		},
	}, nil
}

func (db *ETCDDB) createRawActualLRP(logger lager.Logger, lrp *models.ActualLRP) error {
	logger = logger.Session("creating-raw-actual-lrp", lager.Data{"actual_lrp": lrp})
	logger.Debug("starting")
	defer logger.Debug("complete")

	lrpData, err := db.serializeModel(logger, lrp)
	if err != nil {
		logger.Error("failed-to-marshal-actual-lrp", err, lager.Data{"actual_lrp": lrp})
		return err
	}

	_, err = db.client.Create(ActualLRPSchemaPath(lrp.ProcessGuid, lrp.Index), lrpData, 0)
	if err != nil {
		logger.Error("failed-to-create-actual-lrp", err)
		return models.ErrActualLRPCannotBeStarted
	}

	return nil
}

func (db *ETCDDB) createRunningActualLRP(logger lager.Logger, key *models.ActualLRPKey, instanceKey *models.ActualLRPInstanceKey, netInfo *models.ActualLRPNetInfo) (*models.ActualLRP, error) {
	lrp, err := db.newRunningActualLRP(key, instanceKey, netInfo)
	if err != nil {
		return nil, models.ErrActualLRPCannotBeStarted
	}

	return lrp, db.createRawActualLRP(logger, lrp)
}
