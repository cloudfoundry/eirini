package migrations

import (
	"database/sql"
	"errors"

	"code.cloudfoundry.org/bbs/db/etcd"
	"code.cloudfoundry.org/bbs/encryption"
	"code.cloudfoundry.org/bbs/format"
	"code.cloudfoundry.org/bbs/migration"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
)

func init() {
	AppendMigration(NewTimeoutMilliseconds())
}

type TimeoutToMilliseconds struct {
	serializer  format.Serializer
	storeClient etcd.StoreClient
}

func NewTimeoutMilliseconds() migration.Migration {
	return &TimeoutToMilliseconds{}
}

func (b *TimeoutToMilliseconds) Version() int64 {
	return 1451635200
}

func (b *TimeoutToMilliseconds) SetStoreClient(storeClient etcd.StoreClient) {
	b.storeClient = storeClient
}

func (b *TimeoutToMilliseconds) SetCryptor(cryptor encryption.Cryptor) {
	b.serializer = format.NewSerializer(cryptor)
}

func (b *TimeoutToMilliseconds) Up(logger lager.Logger) error {
	response, err := b.storeClient.Get(etcd.TaskSchemaRoot, false, true)
	if err != nil {
		logger.Error("failed-fetching-tasks", err)
	}

	if response != nil {
		for _, node := range response.Node.Nodes {
			task := new(models.Task)
			err := b.serializer.Unmarshal(logger, []byte(node.Value), task)
			if err != nil {
				logger.Error("failed-to-deserialize-task", err)
				continue
			}

			updateTimeoutInAction(logger, task.Action)

			value, err := b.serializer.Marshal(logger, format.ENCODED_PROTO, task)
			if err != nil {
				return err
			}

			_, err = b.storeClient.CompareAndSwap(node.Key, value, etcd.NO_TTL, node.ModifiedIndex)
			if err != nil {
				return err
			}
		}
	}

	// Do DesiredLRP update
	response, err = b.storeClient.Get(etcd.DesiredLRPRunInfoSchemaRoot, false, true)
	if err != nil {
		logger.Error("failed-fetching-desired-lrp-run-info", err)
	}

	if response != nil {
		for _, node := range response.Node.Nodes {
			runInfo := new(models.DesiredLRPRunInfo)
			err := b.serializer.Unmarshal(logger, []byte(node.Value), runInfo)
			if err != nil {
				logger.Error("failed-to-deserialize-desired-lrp-run-info", err)
				continue
			}
			logger.Info("update-run-info", lager.Data{"deprecated_timeout_ns": runInfo.DeprecatedStartTimeoutS})
			runInfo.StartTimeoutMs = int64(runInfo.DeprecatedStartTimeoutS) * 1000
			updateTimeoutInAction(logger, runInfo.GetMonitor())
			updateTimeoutInAction(logger, runInfo.GetSetup())
			updateTimeoutInAction(logger, runInfo.GetAction())

			value, err := b.serializer.Marshal(logger, format.ENCODED_PROTO, runInfo)
			if err != nil {
				return err
			}

			_, err = b.storeClient.CompareAndSwap(node.Key, value, etcd.NO_TTL, node.ModifiedIndex)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (b *TimeoutToMilliseconds) Down(logger lager.Logger) error {
	return errors.New("not implemented")
}

func (b *TimeoutToMilliseconds) RequiresSQL() bool {
	return false
}

func (b *TimeoutToMilliseconds) SetRawSQLDB(*sql.DB)  {}
func (b *TimeoutToMilliseconds) SetClock(clock.Clock) {}
func (b *TimeoutToMilliseconds) SetDBFlavor(string)   {}

func updateTimeoutInAction(logger lager.Logger, action *models.Action) {
	if action == nil {
		logger.Debug("no-action-to-convert")
		return
	}

	a := action.GetValue()
	switch actionModel := a.(type) {
	case *models.RunAction, *models.DownloadAction, *models.UploadAction:
		return

	case *models.TimeoutAction:
		timeoutAction := actionModel
		timeoutAction.TimeoutMs = timeoutAction.DeprecatedTimeoutNs / 1000000

	case *models.EmitProgressAction:
		updateTimeoutInAction(logger, actionModel.Action)

	case *models.TryAction:
		updateTimeoutInAction(logger, actionModel.Action)

	case *models.ParallelAction:
		for _, subaction := range actionModel.Actions {
			updateTimeoutInAction(logger, subaction)
		}

	case *models.SerialAction:
		for _, subaction := range actionModel.Actions {
			updateTimeoutInAction(logger, subaction)
		}

	case *models.CodependentAction:
		for _, subaction := range actionModel.Actions {
			updateTimeoutInAction(logger, subaction)
		}
	}
}
