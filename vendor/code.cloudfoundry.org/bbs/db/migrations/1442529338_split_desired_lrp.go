package migrations

import (
	"database/sql"
	"errors"

	"code.cloudfoundry.org/bbs/db/deprecations"
	"code.cloudfoundry.org/bbs/db/etcd"
	"code.cloudfoundry.org/bbs/encryption"
	"code.cloudfoundry.org/bbs/format"
	"code.cloudfoundry.org/bbs/migration"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
)

func init() {
	AppendMigration(NewSplitDesiredLRP())
}

type SplitDesiredLRP struct {
	serializer  format.Serializer
	storeClient etcd.StoreClient
}

func NewSplitDesiredLRP() migration.Migration {
	return &SplitDesiredLRP{}
}

func (m *SplitDesiredLRP) Version() int64 {
	return 1442529338
}

func (m *SplitDesiredLRP) SetStoreClient(storeClient etcd.StoreClient) {
	m.storeClient = storeClient
}

func (m *SplitDesiredLRP) SetCryptor(cryptor encryption.Cryptor) {
	m.serializer = format.NewSerializer(cryptor)
}

func (m *SplitDesiredLRP) RequiresSQL() bool {
	return false
}

func (m *SplitDesiredLRP) SetRawSQLDB(rawSQLDB *sql.DB) {}
func (m *SplitDesiredLRP) SetClock(clock.Clock)         {}
func (m *SplitDesiredLRP) SetDBFlavor(string)           {}

func (m *SplitDesiredLRP) Up(logger lager.Logger) error {
	_, err := m.storeClient.Delete(etcd.DesiredLRPSchedulingInfoSchemaRoot, true)
	if err != nil {
		logger.Error("failed-to-delete-dir", err, lager.Data{"key": etcd.DesiredLRPSchedulingInfoSchemaRoot})
	}

	_, err = m.storeClient.Delete(etcd.DesiredLRPRunInfoSchemaRoot, true)
	if err != nil {
		logger.Error("failed-to-delete-dir", err, lager.Data{"key": etcd.DesiredLRPRunInfoSchemaRoot})
	}

	response, err := m.storeClient.Get(deprecations.DesiredLRPSchemaRoot, false, true)
	if err != nil {
		err = etcd.ErrorFromEtcdError(logger, err)
		if err != models.ErrResourceNotFound {
			return err
		}
	}

	if response != nil {
		desiredLRPRootNode := response.Node
		for _, node := range desiredLRPRootNode.Nodes {
			var desiredLRP models.DesiredLRP
			err := m.serializer.Unmarshal(logger, []byte(node.Value), &desiredLRP)
			if err != nil {
				logger.Error("failed-unmarshaling-desired-lrp", err, lager.Data{"process_guid": desiredLRP.ProcessGuid})
				continue
			}

			m.WriteRunInfo(logger, desiredLRP)
			m.WriteSchedulingInfo(logger, desiredLRP)

		}

		_, err = m.storeClient.Delete(deprecations.DesiredLRPSchemaRoot, true)
		if err != nil {
			logger.Error("failed-to-delete-dir", err, lager.Data{"key": deprecations.DesiredLRPSchemaRoot})
		}
	}

	return nil
}

func (m *SplitDesiredLRP) Down(logger lager.Logger) error {
	return errors.New("not implemented")
}

func (m *SplitDesiredLRP) WriteRunInfo(logger lager.Logger, desiredLRP models.DesiredLRP) {
	environmentVariables := make([]models.EnvironmentVariable, len(desiredLRP.EnvironmentVariables))
	for i := range desiredLRP.EnvironmentVariables {
		environmentVariables[i] = *desiredLRP.EnvironmentVariables[i]
	}

	egressRules := make([]models.SecurityGroupRule, len(desiredLRP.EgressRules))
	for i := range desiredLRP.EgressRules {
		egressRules[i] = *desiredLRP.EgressRules[i]
	}

	runInfo := models.DesiredLRPRunInfo{
		DesiredLRPKey:        desiredLRP.DesiredLRPKey(),
		EnvironmentVariables: environmentVariables,
		Setup:                desiredLRP.Setup,
		Action:               desiredLRP.Action,
		Monitor:              desiredLRP.Monitor,
		StartTimeoutMs:       desiredLRP.StartTimeoutMs,
		Privileged:           desiredLRP.Privileged,
		CpuWeight:            desiredLRP.CpuWeight,
		Ports:                desiredLRP.Ports,
		EgressRules:          egressRules,
		LogSource:            desiredLRP.LogSource,
		MetricsGuid:          desiredLRP.MetricsGuid,
	}

	runInfoPayload, marshalErr := m.serializer.Marshal(logger, format.ENCRYPTED_PROTO, &runInfo)
	if marshalErr != nil {
		logger.Error("failed-marshaling-run-info", marshalErr, lager.Data{"process_guid": runInfo.ProcessGuid})
	}

	_, setErr := m.storeClient.Set(etcd.DesiredLRPRunInfoSchemaPath(runInfo.ProcessGuid), runInfoPayload, etcd.NO_TTL)
	if setErr != nil {
		logger.Error("failed-set-of-run-info", marshalErr, lager.Data{"process_guid": runInfo.ProcessGuid})
	}
}

func (m *SplitDesiredLRP) WriteSchedulingInfo(logger lager.Logger, desiredLRP models.DesiredLRP) {
	schedulingInfo := models.DesiredLRPSchedulingInfo{
		DesiredLRPKey:      desiredLRP.DesiredLRPKey(),
		Annotation:         desiredLRP.Annotation,
		Instances:          desiredLRP.Instances,
		DesiredLRPResource: desiredLRP.DesiredLRPResource(),
	}
	if desiredLRP.Routes != nil {
		schedulingInfo.Routes = *desiredLRP.Routes
	}
	if desiredLRP.ModificationTag != nil {
		schedulingInfo.ModificationTag = *desiredLRP.ModificationTag
	}

	schedulingInfoPayload, marshalErr := m.serializer.Marshal(logger, format.ENCRYPTED_PROTO, &schedulingInfo)
	if marshalErr != nil {
		logger.Error("failed-marshaling-scheduling-info", marshalErr, lager.Data{"process_guid": schedulingInfo.ProcessGuid})
	}

	_, setErr := m.storeClient.Set(etcd.DesiredLRPSchedulingInfoSchemaPath(desiredLRP.ProcessGuid), schedulingInfoPayload, etcd.NO_TTL)
	if setErr != nil {
		logger.Error("failed-set-of-scheduling-info", marshalErr, lager.Data{"process_guid": schedulingInfo.ProcessGuid})
	}
}
