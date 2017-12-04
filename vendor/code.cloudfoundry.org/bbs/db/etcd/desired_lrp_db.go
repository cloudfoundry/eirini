package etcd

import (
	"sync"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"
	"github.com/coreos/go-etcd/etcd"
	"github.com/nu7hatch/gouuid"
)

type guidSet struct {
	set map[string]struct{}
}

func newGuidSet() guidSet {
	return guidSet{
		set: map[string]struct{}{},
	}
}

func (g guidSet) Add(guid string) {
	g.set[guid] = struct{}{}
}

func (g guidSet) Merge(other guidSet) {
	for guid := range other.set {
		g.set[guid] = struct{}{}
	}
}

func (g guidSet) ToMap() map[string]struct{} {
	return g.set
}

func (db *ETCDDB) DesiredLRPs(logger lager.Logger, filter models.DesiredLRPFilter) ([]*models.DesiredLRP, error) {
	logger = logger.WithData(lager.Data{"filter": filter})
	logger.Info("start")
	defer logger.Info("complete")

	desireds, _, err := db.desiredLRPs(logger, filter)
	if err != nil {
		logger.Error("failed", err)
	}
	return desireds, err
}

func (db *ETCDDB) DesiredLRPSchedulingInfos(logger lager.Logger, filter models.DesiredLRPFilter) ([]*models.DesiredLRPSchedulingInfo, error) {
	logger = logger.WithData(lager.Data{"filter": filter})
	logger.Info("start")
	defer logger.Info("complete")

	root, err := db.fetchRecursiveRaw(logger, DesiredLRPSchedulingInfoSchemaRoot)
	bbsErr := models.ConvertError(err)
	if bbsErr != nil {
		if bbsErr.Type == models.Error_ResourceNotFound {
			return []*models.DesiredLRPSchedulingInfo{}, nil
		}
		return nil, err
	}

	schedulingInfoMap, _ := db.deserializeScheduleInfos(logger, root.Nodes, filter)

	schedulingInfos := make([]*models.DesiredLRPSchedulingInfo, 0, len(schedulingInfoMap))
	for _, schedulingInfo := range schedulingInfoMap {
		schedulingInfos = append(schedulingInfos, schedulingInfo)
	}
	return schedulingInfos, nil
}

func (db *ETCDDB) desiredLRPs(logger lager.Logger, filter models.DesiredLRPFilter) ([]*models.DesiredLRP, guidSet, error) {
	root, err := db.fetchRecursiveRaw(logger, DesiredLRPComponentsSchemaRoot)
	bbsErr := models.ConvertError(err)
	if bbsErr != nil {
		if bbsErr.Type == models.Error_ResourceNotFound {
			return []*models.DesiredLRP{}, newGuidSet(), nil
		}
		return nil, newGuidSet(), err
	}
	if root.Nodes.Len() == 0 {
		return []*models.DesiredLRP{}, newGuidSet(), nil
	}

	var schedules map[string]*models.DesiredLRPSchedulingInfo
	var runs map[string]*models.DesiredLRPRunInfo
	var malformedInfos guidSet
	var malformedRunInfos guidSet
	var wg sync.WaitGroup
	for i := range root.Nodes {
		node := root.Nodes[i]
		switch node.Key {
		case DesiredLRPSchedulingInfoSchemaRoot:
			wg.Add(1)
			go func() {
				defer wg.Done()
				schedules, malformedInfos = db.deserializeScheduleInfos(logger, node.Nodes, filter)
			}()
		case DesiredLRPRunInfoSchemaRoot:
			wg.Add(1)
			go func() {
				defer wg.Done()
				runs, malformedRunInfos = db.deserializeRunInfos(logger, node.Nodes, filter)
			}()
		default:
			logger.Error("unexpected-etcd-key", nil, lager.Data{"key": node.Key})
		}
	}

	wg.Wait()

	desiredLRPs := []*models.DesiredLRP{}
	for processGuid, schedule := range schedules {
		desired := models.NewDesiredLRP(*schedule, *runs[processGuid])
		desiredLRPs = append(desiredLRPs, &desired)
	}

	malformedInfos.Merge(malformedRunInfos)
	return desiredLRPs, malformedInfos, nil
}

func (db *ETCDDB) deserializeScheduleInfos(logger lager.Logger, nodes etcd.Nodes, filter models.DesiredLRPFilter) (map[string]*models.DesiredLRPSchedulingInfo, guidSet) {
	logger.Debug("deserializing-scheduling-infos", lager.Data{"count": len(nodes)})

	components := make(map[string]*models.DesiredLRPSchedulingInfo)
	malformedModels := newGuidSet()

	for i := range nodes {
		node := nodes[i]
		model := new(models.DesiredLRPSchedulingInfo)
		err := db.deserializeModel(logger, node, model)
		if err != nil {
			logger.Error("failed-parsing-desired-lrp-scheduling-info", err)
			malformedModels.Add(model.ProcessGuid)
			continue
		}
		if filter.Domain == "" || model.Domain == filter.Domain {
			components[model.ProcessGuid] = model
		}
	}

	return components, malformedModels
}

func (db *ETCDDB) deserializeRunInfos(logger lager.Logger, nodes etcd.Nodes, filter models.DesiredLRPFilter) (map[string]*models.DesiredLRPRunInfo, guidSet) {
	logger.Info("deserializing-run-infos", lager.Data{"count": len(nodes)})

	components := make(map[string]*models.DesiredLRPRunInfo, len(nodes))
	malformedModels := newGuidSet()

	for i := range nodes {
		node := nodes[i]
		model := new(models.DesiredLRPRunInfo)
		err := db.deserializeModel(logger, node, model)
		if err != nil {
			logger.Error("failed-parsing-desired-lrp-run-info", err)
			malformedModels.Add(model.ProcessGuid)
			continue
		}
		if filter.Domain == "" || model.Domain == filter.Domain {
			components[model.ProcessGuid] = model
		}
	}

	return components, malformedModels
}

func (db *ETCDDB) rawDesiredLRPSchedulingInfo(logger lager.Logger, processGuid string) (*models.DesiredLRPSchedulingInfo, uint64, error) {
	node, err := db.fetchRaw(logger, DesiredLRPSchedulingInfoSchemaPath(processGuid))
	if err != nil {
		logger.Error("failed-to-fetch-existing-scheduling-info", err)
		return nil, 0, err
	}

	model := new(models.DesiredLRPSchedulingInfo)
	err = db.deserializeModel(logger, node, model)
	if err != nil {
		logger.Error("failed-parsing-desired-lrp-scheduling-info", err)
		return nil, 0, err
	}

	return model, node.ModifiedIndex, nil
}

func (db *ETCDDB) rawDesiredLRPRunInfo(logger lager.Logger, processGuid string) (*models.DesiredLRPRunInfo, error) {
	node, err := db.fetchRaw(logger, DesiredLRPRunInfoSchemaPath(processGuid))
	if err != nil {
		return nil, err
	}

	model := new(models.DesiredLRPRunInfo)
	err = db.deserializeModel(logger, node, model)
	if err != nil {
		logger.Error("failed-parsing-desired-lrp-run-info", err)
		return nil, err
	}

	return model, nil
}

func (db *ETCDDB) rawDesiredLRPByProcessGuid(logger lager.Logger, processGuid string) (*models.DesiredLRP, uint64, error) {
	var wg sync.WaitGroup

	var schedulingInfo *models.DesiredLRPSchedulingInfo
	var runInfo *models.DesiredLRPRunInfo
	var schedulingErr, runErr error

	var index uint64
	wg.Add(1)
	go func() {
		defer wg.Done()
		schedulingInfo, index, schedulingErr = db.rawDesiredLRPSchedulingInfo(logger, processGuid)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		runInfo, runErr = db.rawDesiredLRPRunInfo(logger, processGuid)
	}()

	wg.Wait()

	if schedulingErr != nil {
		return nil, 0, schedulingErr
	}

	if runErr != nil {
		return nil, 0, runErr
	}

	desiredLRP := models.NewDesiredLRP(*schedulingInfo, *runInfo)
	return &desiredLRP, index, nil
}

func (db *ETCDDB) DesiredLRPByProcessGuid(logger lager.Logger, processGuid string) (*models.DesiredLRP, error) {
	lrp, _, err := db.rawDesiredLRPByProcessGuid(logger, processGuid)
	return lrp, err
}

// DesireLRP creates a DesiredLRPSchedulingInfo and a DesiredLRPRunInfo. In order
// to ensure that the complete model is available and there are no races in
// Desired Watches, DesiredLRPRunInfo is created before DesiredLRPSchedulingInfo.
func (db *ETCDDB) DesireLRP(logger lager.Logger, desiredLRP *models.DesiredLRP) error {
	logger = logger.WithData(lager.Data{"process_guid": desiredLRP.ProcessGuid})
	logger.Info("starting")
	defer logger.Info("complete")

	schedulingInfo, runInfo := desiredLRP.CreateComponents(db.clock.Now())

	err := db.createDesiredLRPRunInfo(logger, &runInfo)
	if err != nil {
		return err
	}

	schedulingErr := db.createDesiredLRPSchedulingInfo(logger, &schedulingInfo)
	if schedulingErr != nil {
		logger.Info("deleting-orphaned-run-info")
		_, err = db.client.Delete(DesiredLRPRunInfoSchemaPath(desiredLRP.ProcessGuid), true)
		if err != nil {
			logger.Error("failed-deleting-orphaned-run-info", err)
		}
		return schedulingErr
	}

	return nil
}

func (db *ETCDDB) createDesiredLRPSchedulingInfo(logger lager.Logger, schedulingInfo *models.DesiredLRPSchedulingInfo) error {
	epochGuid, err := uuid.NewV4()
	if err != nil {
		logger.Error("failed-to-generate-epoch", err)
		return models.ErrUnknownError
	}

	schedulingInfo.ModificationTag = models.NewModificationTag(epochGuid.String(), 0)

	serializedSchedInfo, err := db.serializeModel(logger, schedulingInfo)
	if err != nil {
		logger.Error("failed-to-serialize", err)
		return err
	}

	_, err = db.client.Create(DesiredLRPSchedulingInfoSchemaPath(schedulingInfo.ProcessGuid), serializedSchedInfo, NO_TTL)
	err = ErrorFromEtcdError(logger, err)
	if err != nil {
		logger.Error("failed-persisting-scheduling-info", err)
		return err
	}

	return nil
}

func (db *ETCDDB) updateDesiredLRPSchedulingInfo(logger lager.Logger, schedulingInfo *models.DesiredLRPSchedulingInfo, index uint64) error {
	value, err := db.serializeModel(logger, schedulingInfo)
	if err != nil {
		logger.Error("failed-to-serialize-scheduling-info", err)
		return err
	}

	_, err = db.client.CompareAndSwap(DesiredLRPSchedulingInfoSchemaPath(schedulingInfo.ProcessGuid), value, NO_TTL, index)
	if err != nil {
		logger.Error("failed-to-CAS-scheduling-info", err)
		return ErrorFromEtcdError(logger, err)
	}

	return nil
}

func (db *ETCDDB) createDesiredLRPRunInfo(logger lager.Logger, runInfo *models.DesiredLRPRunInfo) error {
	serializedRunInfo, err := db.serializeModel(logger, runInfo)
	if err != nil {
		logger.Error("failed-to-serialize", err)
		return err
	}

	_, err = db.client.Create(DesiredLRPRunInfoSchemaPath(runInfo.ProcessGuid), serializedRunInfo, NO_TTL)
	if err != nil {
		logger.Error("failed-persisting-run-info", err)
		return ErrorFromEtcdError(logger, err)
	}

	return nil
}

func (db *ETCDDB) UpdateDesiredLRP(logger lager.Logger, processGuid string, update *models.DesiredLRPUpdate) (*models.DesiredLRP, error) {
	logger.Info("starting")
	defer logger.Info("complete")

	var schedulingInfo *models.DesiredLRPSchedulingInfo
	var err error
	var beforeDesiredLRP *models.DesiredLRP

	for i := 0; i < 2; i++ {
		var index uint64

		beforeDesiredLRP, index, err = db.rawDesiredLRPByProcessGuid(logger, processGuid)
		if err != nil {
			logger.Error("failed-to-fetch-desired-lrp", err)
			break
		}

		schedulingInfoValue := beforeDesiredLRP.DesiredLRPSchedulingInfo()
		schedulingInfo = &schedulingInfoValue
		schedulingInfo.ApplyUpdate(update)

		err = db.updateDesiredLRPSchedulingInfo(logger, schedulingInfo, index)
		if err != nil {
			logger.Error("update-scheduling-info-failed", err)
			modelErr := models.ConvertError(err)
			if modelErr != models.ErrResourceConflict {
				break
			}
			// Retry on CAS fail
			continue
		}

		break
	}

	if err != nil {
		return nil, err
	}

	return beforeDesiredLRP, nil
}

// RemoveDesiredLRP deletes the DesiredLRPSchedulingInfo and the DesiredLRPRunInfo
// from the database. We delete DesiredLRPSchedulingInfo first because the system
// uses it to determine wheter the lrp is present. In the event that only the
// RunInfo fails to delete, the orphaned DesiredLRPRunInfo will be garbage
// collected later by convergence.
func (db *ETCDDB) RemoveDesiredLRP(logger lager.Logger, processGuid string) error {
	logger = logger.WithData(lager.Data{"process_guid": processGuid})
	logger.Info("starting")
	defer logger.Info("complete")

	_, schedulingInfoErr := db.client.Delete(DesiredLRPSchedulingInfoSchemaPath(processGuid), true)
	schedulingInfoErr = ErrorFromEtcdError(logger, schedulingInfoErr)
	if schedulingInfoErr != nil && schedulingInfoErr != models.ErrResourceNotFound {
		logger.Error("failed-deleting-scheduling-info", schedulingInfoErr)
		return schedulingInfoErr
	}

	_, runInfoErr := db.client.Delete(DesiredLRPRunInfoSchemaPath(processGuid), true)
	runInfoErr = ErrorFromEtcdError(logger, runInfoErr)
	if runInfoErr != nil && runInfoErr != models.ErrResourceNotFound {
		logger.Error("failed-deleting-run-info", runInfoErr)
		return runInfoErr
	}

	if schedulingInfoErr == models.ErrResourceNotFound && runInfoErr == models.ErrResourceNotFound {
		// If neither component of the desired LRP exists, don't bother trying to delete running instances
		return models.ErrResourceNotFound
	}

	return nil
}
