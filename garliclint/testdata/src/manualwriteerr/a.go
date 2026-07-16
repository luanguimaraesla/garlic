package manualwriteerr

import (
	"net/http"

	"github.com/luanguimaraesla/garlic/errors"
	"github.com/luanguimaraesla/garlic/rest"
)

func invalid(w http.ResponseWriter, r *http.Request) error {
	http.Error(w, "bad request", http.StatusBadRequest) // want "\\[G6.01\\]"
	err := errors.New(errors.KindError, "failed")
	rest.WriteError(err).Must(w) // want "\\[G6.06\\]"
	return err
}
