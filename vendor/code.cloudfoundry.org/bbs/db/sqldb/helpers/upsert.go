package helpers

import (
	"code.cloudfoundry.org/lager"
)

// Upsert insert a record if it doesn't exist or update the record if one
// already exists.  Returns true if a new record was inserted in the database.
func (h *sqlHelper) Upsert(
	logger lager.Logger,
	q Queryable,
	table string,
	attributes SQLAttributes,
	wheres string,
	whereBindings ...interface{},
) (bool, error) {
	res, err := h.Update(
		logger,
		q,
		table,
		attributes,
		wheres,
		whereBindings...,
	)
	if err != nil {
		return false, err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		// this should never happen
		logger.Error("failed-getting-rows-affected", err)
		return false, err
	}

	if rowsAffected > 0 {
		return false, nil
	}

	res, err = h.Insert(
		logger,
		q,
		table,
		attributes,
	)
	if err != nil {
		return false, err
	}

	return true, nil
}
