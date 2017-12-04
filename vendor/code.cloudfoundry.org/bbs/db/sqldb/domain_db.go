package sqldb

import (
	"database/sql"
	"math"
	"time"

	"code.cloudfoundry.org/bbs/db/sqldb/helpers"
	"code.cloudfoundry.org/lager"
)

func (db *SQLDB) Domains(logger lager.Logger) ([]string, error) {
	logger = logger.Session("domains")
	logger.Debug("starting")
	defer logger.Debug("complete")

	var results []string
	err := db.transact(logger, func(logger lager.Logger, tx *sql.Tx) error {
		expireTime := db.clock.Now().Round(time.Second).UnixNano()
		rows, err := db.all(logger, tx, domainsTable,
			domainColumns, helpers.NoLockRow,
			"expire_time > ?", expireTime,
		)
		if err != nil {
			logger.Error("failed-query", err)
			return err
		}

		defer rows.Close()

		var domain string
		for rows.Next() {
			err = rows.Scan(&domain)
			if err != nil {
				logger.Error("failed-scan-row", err)
				return err
			}
			results = append(results, domain)
		}

		if rows.Err() != nil {
			logger.Error("failed-fetching-row", err)
			return err
		}

		return nil
	})

	return results, err
}

func (db *SQLDB) UpsertDomain(logger lager.Logger, domain string, ttl uint32) error {
	logger = logger.Session("upsert-domain", lager.Data{"domain": domain, "ttl": ttl})
	logger.Debug("starting")
	defer logger.Debug("complete")

	return db.transact(logger, func(logger lager.Logger, tx *sql.Tx) error {
		expireTime := db.clock.Now().Add(time.Duration(ttl) * time.Second).UnixNano()
		if ttl == 0 {
			expireTime = math.MaxInt64
		}

		_, err := db.upsert(logger, tx, domainsTable,
			helpers.SQLAttributes{"domain": domain, "expire_time": expireTime},
			"domain = ?", domain,
		)
		if err != nil {
			logger.Error("failed-inserting-domain", err)
		}
		return err
	})
}
