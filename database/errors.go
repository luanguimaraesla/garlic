package database

import (
	"net/http"

	"github.com/luanguimaraesla/garlic/errors"
)

// Database error kinds classify record-not-found and transaction failures. They
// descend from the relevant garlic base kinds; importing this package registers
// them with the errors registry.
var (
	KindDatabaseRecordNotFoundError = &errors.Kind{
		Name:        "DatabaseRecordNotFoundError",
		Code:        "C00008",
		Description: "The requested database record was not found.",
		Parent:      errors.KindNotFoundError,
	}

	KindDatabaseTransactionError = &errors.Kind{
		Name:        "DatabaseTransactionError",
		Code:        "C00009",
		Description: "An error occurred during a database transaction.",
		Parent:      errors.KindForStatus(http.StatusInternalServerError),
	}
)

func init() {
	errors.Register(
		KindDatabaseRecordNotFoundError,
		KindDatabaseTransactionError,
	)
}
