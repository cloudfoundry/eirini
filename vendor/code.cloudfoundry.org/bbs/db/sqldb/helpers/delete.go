package helpers

import (
	"database/sql"
	"fmt"

	"code.cloudfoundry.org/lager"
)

// DELETE FROM <table> WHERE ...
func (h *sqlHelper) Delete(
	logger lager.Logger,
	q Queryable,
	table string,
	wheres string,
	whereBindings ...interface{},
) (sql.Result, error) {
	query := fmt.Sprintf("DELETE FROM %s\n", table)

	if len(wheres) > 0 {
		query += "WHERE " + wheres
	}

	return q.Exec(h.Rebind(query), whereBindings...)
}
