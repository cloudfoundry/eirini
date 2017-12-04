package helpers

import (
	"database/sql"

	"code.cloudfoundry.org/lager"
)

func (h *sqlHelper) Upsert(
	logger lager.Logger,
	q Queryable,
	table string,
	attributes SQLAttributes,
	wheres string,
	whereBindings ...interface{},
) (sql.Result, error) {
	res, err := h.Update(
		logger,
		q,
		table,
		attributes,
		wheres,
		whereBindings...,
	)
	if err != nil {
		return nil, err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		// this should never happen
		logger.Error("failed-getting-rows-affected", err)
		return nil, err
	}

	if rowsAffected > 0 {
		return res, nil
	}

	res, err = h.Insert(
		logger,
		q,
		table,
		attributes,
	)
	if err != nil {
		return nil, err
	}

	return res, nil
}
