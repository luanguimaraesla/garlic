package manualwriteerr

import (
	"net/http"

	"github.com/luanguimaraesla/garlic/errors"
	"github.com/luanguimaraesla/garlic/rest"
)

func valid(w http.ResponseWriter, r *http.Request) error {
	if r == nil {
		return errors.New(errors.KindError, "missing request")
	}
	rest.WriteMessage(http.StatusOK, "ok").Must(w)
	return nil
}

func writeUnavailable(w http.ResponseWriter, code int) error {
	http.Error(w, "unavailable", code)
	return nil
}
