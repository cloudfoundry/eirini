package migrations

import (
	"database/sql"
	"encoding/json"
	"errors"
	"path"
	"time"

	"code.cloudfoundry.org/bbs/db/etcd"
	"code.cloudfoundry.org/bbs/db/sqldb/helpers"
	"code.cloudfoundry.org/bbs/encryption"
	"code.cloudfoundry.org/bbs/format"
	"code.cloudfoundry.org/bbs/migration"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
)

func init() {
	AppendMigration(NewETCDToSQL())
}

type ETCDToSQL struct {
	serializer  format.Serializer
	storeClient etcd.StoreClient
	clock       clock.Clock
	rawSQLDB    *sql.DB
	dbFlavor    string
}

type ETCDToSQLDesiredLRP struct {
	// DesiredLRPKey
	ProcessGuid string
	Domain      string
	LogGuid     string

	Annotation string
	Instances  int32

	// DesiredLRPResource
	RootFS   string
	DiskMB   int32
	MemoryMB int32

	// Routes
	Routes []byte

	// ModificationTag
	ModificationTagEpoch string
	ModificationTagIndex uint32

	// DesiredLRPRunInfo
	RunInfo         []byte
	VolumePlacement []byte
}

type ETCDToSQLActualLRP struct {
	// ActualLRPKey
	ProcessGuid string
	Index       int32
	Domain      string

	// ActualLRPInstanceKey
	InstanceGuid string
	CellId       string

	ActualLRPNetInfo []byte

	CrashCount     int32
	CrashReason    string
	State          string
	PlacementError string
	Since          int64

	// ModificationTag
	ModificationTagEpoch string
	ModificationTagIndex uint32
}

type ETCDToSQLTask struct {
	TaskGuid         string
	Domain           string
	CreatedAt        int64
	UpdatedAt        int64
	FirstCompletedAt int64
	State            int32
	CellId           string
	Result           string
	Failed           bool
	FailureReason    string
	TaskDefinition   []byte
}

func NewETCDToSQL() migration.Migration {
	return &ETCDToSQL{}
}

func (e *ETCDToSQL) String() string {
	return "1461790966"
}

func (e *ETCDToSQL) Version() int64 {
	return 1461790966
}

func (e *ETCDToSQL) SetStoreClient(storeClient etcd.StoreClient) {
	e.storeClient = storeClient
}

func (e *ETCDToSQL) SetCryptor(cryptor encryption.Cryptor) {
	e.serializer = format.NewSerializer(cryptor)
}

func (e *ETCDToSQL) SetRawSQLDB(db *sql.DB) {
	e.rawSQLDB = db
}

func (e *ETCDToSQL) RequiresSQL() bool         { return true }
func (e *ETCDToSQL) SetClock(c clock.Clock)    { e.clock = c }
func (e *ETCDToSQL) SetDBFlavor(flavor string) { e.dbFlavor = flavor }

func (e *ETCDToSQL) Up(logger lager.Logger) error {
	logger = logger.Session("etcd-to-sql")
	logger.Info("truncating-tables")

	// Ignore the error as the tables may not exist
	_ = dropTables(e.rawSQLDB)

	err := createTables(logger, e.rawSQLDB, e.dbFlavor)
	if err != nil {
		return err
	}

	err = createIndices(logger, e.rawSQLDB)
	if err != nil {
		return err
	}

	if e.storeClient == nil {
		logger.Info("skipping-migration-because-no-etcd-configured")
		return nil
	}

	if err := e.migrateDomains(logger); err != nil {
		return err
	}

	if err := e.migrateDesiredLRPs(logger); err != nil {
		return err
	}

	if err := e.migrateActualLRPs(logger); err != nil {
		return err
	}

	if err := e.migrateTasks(logger); err != nil {
		return err
	}

	return nil
}

func (e *ETCDToSQL) Down(logger lager.Logger) error {
	return errors.New("not implemented")
}

func dropTables(db *sql.DB) error {
	tableNames := []string{
		"domains",
		"tasks",
		"desired_lrps",
		"actual_lrps",
	}
	for _, tableName := range tableNames {
		_, err := db.Exec("DROP TABLE IF EXISTS " + tableName)
		if err != nil {
			return err
		}
	}
	return nil
}

func createTables(logger lager.Logger, db *sql.DB, flavor string) error {
	var createTablesSQL = []string{
		helpers.RebindForFlavor(createDomainSQL, flavor),
		helpers.RebindForFlavor(createDesiredLRPsSQL, flavor),
		helpers.RebindForFlavor(createActualLRPsSQL, flavor),
		helpers.RebindForFlavor(createTasksSQL, flavor),
	}

	logger.Info("creating-tables")
	for _, query := range createTablesSQL {
		logger.Info("creating the table", lager.Data{"query": query})
		_, err := db.Exec(query)
		if err != nil {
			logger.Error("failed-creating-tables", err)
			return err
		}
		logger.Info("created the table", lager.Data{"query": query})
	}

	return nil
}

func createIndices(logger lager.Logger, db *sql.DB) error {
	logger.Info("creating-indices")
	createIndicesSQL := []string{}
	createIndicesSQL = append(createIndicesSQL, createDomainsIndices...)
	createIndicesSQL = append(createIndicesSQL, createDesiredLRPsIndices...)
	createIndicesSQL = append(createIndicesSQL, createActualLRPsIndices...)
	createIndicesSQL = append(createIndicesSQL, createTasksIndices...)

	for _, query := range createIndicesSQL {
		logger.Info("creating the index", lager.Data{"query": query})
		_, err := db.Exec(query)
		if err != nil {
			logger.Error("failed-creating-index", err)
			return err
		}
		logger.Info("created the index", lager.Data{"query": query})
	}

	return nil
}

func (e *ETCDToSQL) migrateDomains(logger lager.Logger) error {
	logger = logger.Session("migrating-domains")
	logger.Debug("starting")
	defer logger.Debug("finished")

	response, err := e.storeClient.Get(etcd.DomainSchemaRoot, false, true)
	if err != nil {
		logger.Error("failed-fetching-domains", err)
	}

	if response != nil {
		for _, node := range response.Node.Nodes {
			domain := path.Base(node.Key)
			expireTime := e.clock.Now().UnixNano() + int64(time.Second)*node.TTL

			_, err := e.rawSQLDB.Exec(helpers.RebindForFlavor(`
				INSERT INTO domains
				(domain, expire_time)
				VALUES (?, ?)
		  `, e.dbFlavor), domain, expireTime)
			if err != nil {
				logger.Error("failed-inserting-domain", err)
				continue
			}
		}
	}

	return nil
}

func (e *ETCDToSQL) migrateDesiredLRPs(logger lager.Logger) error {
	logger = logger.Session("migrating-desired-lrp-scheduling-infos")
	logger.Debug("starting")
	defer logger.Debug("finished")
	response, err := e.storeClient.Get(etcd.DesiredLRPSchedulingInfoSchemaRoot, false, true)
	if err != nil {
		logger.Error("failed-fetching-desired-lrp-scheduling-infos", err)
	}

	schedInfos := make(map[string]*models.DesiredLRPSchedulingInfo)

	if response != nil {
		for _, node := range response.Node.Nodes {
			model := new(models.DesiredLRPSchedulingInfo)
			err := e.serializer.Unmarshal(logger, []byte(node.Value), model)
			if err != nil {
				logger.Error("failed-to-deserialize-desired-lrp-scheduling-info", err)
				continue
			}
			schedInfos[path.Base(node.Key)] = model
		}
	}

	logger.Info("migrating-desired-lrp-run-infos")
	response, err = e.storeClient.Get(etcd.DesiredLRPRunInfoSchemaRoot, false, true)
	if err != nil {
		logger.Error("failed-fetching-desired-lrp-run-infos", err)
	}

	if response != nil {
		for _, node := range response.Node.Nodes {
			schedInfo := schedInfos[path.Base(node.Key)]
			routeData, err := json.Marshal(schedInfo.Routes)
			if err != nil {
				logger.Error("failed-to-marshal-routes", err)
				continue
			}

			if schedInfo.VolumePlacement == nil {
				schedInfo.VolumePlacement = &models.VolumePlacement{}
			}

			volumePlacementData, err := e.serializer.Marshal(logger, format.ENCRYPTED_PROTO, schedInfo.VolumePlacement)
			if err != nil {
				logger.Error("failed-marshalling-volume-placements", err)
			}

			_, err = e.rawSQLDB.Exec(helpers.RebindForFlavor(`
				INSERT INTO desired_lrps
					(process_guid, domain, log_guid, annotation, instances, memory_mb,
					disk_mb, rootfs, volume_placement, routes, modification_tag_epoch,
					modification_tag_index, run_info)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			`, e.dbFlavor), schedInfo.ProcessGuid, schedInfo.Domain, schedInfo.LogGuid, schedInfo.Annotation,
				schedInfo.Instances, schedInfo.MemoryMb, schedInfo.DiskMb, schedInfo.RootFs, volumePlacementData,
				routeData, schedInfo.ModificationTag.Epoch, schedInfo.ModificationTag.Index, []byte(node.Value))
			if err != nil {
				logger.Error("failed-inserting-desired-lrp", err)
				continue
			}
		}
	}

	return nil
}

func (e *ETCDToSQL) migrateActualLRPs(logger lager.Logger) error {
	logger = logger.Session("migrating-actual-lrps")
	logger.Debug("starting")
	defer logger.Debug("finished")
	response, err := e.storeClient.Get(etcd.ActualLRPSchemaRoot, false, true)
	if err != nil {
		logger.Error("failed-fetching-actual-lrps", err)
	}

	if response != nil {
		for _, parent := range response.Node.Nodes {
			for _, indices := range parent.Nodes {
				for _, node := range indices.Nodes {
					// we're going to explicitly ignore evacuating lrps for simplicity's sake
					if path.Base(node.Key) == "instance" {
						actualLRP := new(models.ActualLRP)
						err := e.serializer.Unmarshal(logger, []byte(node.Value), actualLRP)
						if err != nil {
							logger.Error("failed-to-deserialize-actual-lrp", err)
							continue
						}

						netInfoData, err := e.serializer.Marshal(logger, format.ENCRYPTED_PROTO, &actualLRP.ActualLRPNetInfo)
						if err != nil {
							logger.Error("failed-to-marshal-net-info", err)
						}

						_, err = e.rawSQLDB.Exec(helpers.RebindForFlavor(`
							INSERT INTO actual_lrps
								(process_guid, instance_index, domain, instance_guid, cell_id,
								net_info, crash_count, crash_reason, state, placement_error, since,
								modification_tag_epoch, modification_tag_index)
							VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
						`, e.dbFlavor), actualLRP.ProcessGuid, actualLRP.Index, actualLRP.Domain, actualLRP.InstanceGuid,
							actualLRP.CellId, netInfoData, actualLRP.CrashCount, actualLRP.CrashReason,
							actualLRP.State, actualLRP.PlacementError, actualLRP.Since,
							actualLRP.ModificationTag.Epoch, actualLRP.ModificationTag.Index)
						if err != nil {
							logger.Error("failed-inserting-actual-lrp", err)
							continue
						}
					}
				}
			}
		}
	}

	return nil
}

func (e *ETCDToSQL) migrateTasks(logger lager.Logger) error {
	logger = logger.Session("migrating-tasks")
	logger.Debug("starting")
	defer logger.Debug("finished")
	response, err := e.storeClient.Get(etcd.TaskSchemaRoot, false, true)
	if err != nil {
		logger.Error("failed-fetching-tasks", err)
	}

	if response != nil {
		for _, node := range response.Node.Nodes {
			task := new(models.Task)
			err := e.serializer.Unmarshal(logger, []byte(node.Value), task)
			if err != nil {
				logger.Error("failed-to-deserialize-task", err)
				continue
			}

			definitionData, err := e.serializer.Marshal(logger, format.ENCRYPTED_PROTO, task.TaskDefinition)

			_, err = e.rawSQLDB.Exec(helpers.RebindForFlavor(`
							INSERT INTO tasks
								(guid, domain, updated_at, created_at, first_completed_at,
								state, cell_id, result, failed, failure_reason,
								task_definition)
							VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
						`, e.dbFlavor),
				task.TaskGuid, task.Domain, task.UpdatedAt, task.CreatedAt,
				task.FirstCompletedAt, task.State, task.CellId, task.Result,
				task.Failed, task.FailureReason, definitionData)
			if err != nil {
				logger.Error("failed-inserting-task", err)
				continue
			}
		}
	}

	return nil
}

const createDomainSQL = `CREATE TABLE domains(
	domain VARCHAR(255) PRIMARY KEY,
	expire_time BIGINT DEFAULT 0
);`

const createDesiredLRPsSQL = `CREATE TABLE desired_lrps(
	process_guid VARCHAR(255) PRIMARY KEY,
	domain VARCHAR(255) NOT NULL,
	log_guid VARCHAR(255) NOT NULL,
	annotation MEDIUMTEXT,
	instances INT NOT NULL,
	memory_mb INT NOT NULL,
	disk_mb INT NOT NULL,
	rootfs VARCHAR(255) NOT NULL,
	routes MEDIUMTEXT NOT NULL,
	volume_placement MEDIUMTEXT NOT NULL,
	modification_tag_epoch VARCHAR(255) NOT NULL,
	modification_tag_index INT,
	run_info MEDIUMTEXT NOT NULL
);`

const createActualLRPsSQL = `CREATE TABLE actual_lrps(
	process_guid VARCHAR(255),
	instance_index INT,
	evacuating BOOL DEFAULT false,
	domain VARCHAR(255) NOT NULL,
	state VARCHAR(255) NOT NULL,
	instance_guid VARCHAR(255) NOT NULL DEFAULT '',
	cell_id VARCHAR(255) NOT NULL DEFAULT '',
	placement_error VARCHAR(255) NOT NULL DEFAULT '',
	since BIGINT DEFAULT 0,
	net_info MEDIUMTEXT NOT NULL,
	modification_tag_epoch VARCHAR(255) NOT NULL,
	modification_tag_index INT,
	crash_count INT NOT NULL DEFAULT 0,
	crash_reason VARCHAR(255) NOT NULL DEFAULT '',
	expire_time BIGINT DEFAULT 0,

	PRIMARY KEY(process_guid, instance_index, evacuating)
);`

const createTasksSQL = `CREATE TABLE tasks(
	guid VARCHAR(255) PRIMARY KEY,
	domain VARCHAR(255) NOT NULL,
	updated_at BIGINT DEFAULT 0,
	created_at BIGINT DEFAULT 0,
	first_completed_at BIGINT DEFAULT 0,
	state INT,
	cell_id VARCHAR(255) NOT NULL DEFAULT '',
	result MEDIUMTEXT,
	failed BOOL DEFAULT false,
	failure_reason VARCHAR(255) NOT NULL DEFAULT '',
	task_definition MEDIUMTEXT NOT NULL
);`

var createDomainsIndices = []string{
	`CREATE INDEX domains_expire_time_idx ON domains (expire_time)`,
}

var createDesiredLRPsIndices = []string{
	`CREATE INDEX desired_lrps_domain_idx ON desired_lrps (domain)`,
}

var createActualLRPsIndices = []string{
	`CREATE INDEX actual_lrps_domain_idx ON actual_lrps (domain)`,
	`CREATE INDEX actual_lrps_cell_id_idx ON actual_lrps (cell_id)`,
	`CREATE INDEX actual_lrps_since_idx ON actual_lrps (since)`,
	`CREATE INDEX actual_lrps_state_idx ON actual_lrps (state)`,
	`CREATE INDEX actual_lrps_expire_time_idx ON actual_lrps (expire_time)`,
}

var createTasksIndices = []string{
	`CREATE INDEX tasks_domain_idx ON tasks (domain)`,
	`CREATE INDEX tasks_state_idx ON tasks (state)`,
	`CREATE INDEX tasks_cell_id_idx ON tasks (cell_id)`,
	`CREATE INDEX tasks_updated_at_idx ON tasks (updated_at)`,
	`CREATE INDEX tasks_created_at_idx ON tasks (created_at)`,
	`CREATE INDEX tasks_first_completed_at_idx ON tasks (first_completed_at)`,
}
