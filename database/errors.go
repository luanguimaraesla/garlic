package database

import "github.com/luanguimaraesla/garlic/errors"

var (
	KindDatabaseRecordNotFoundError = errors.Get("DatabaseRecordNotFoundError")
	KindDatabaseTransactionError    = errors.Get("DatabaseTransactionError")
)
