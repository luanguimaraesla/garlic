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

func register(routes map[string]func(http.ResponseWriter, *http.Request) error) {
	routes["/x"] = func(w http.ResponseWriter, r *http.Request) error {
		http.Error(w, "bad request", http.StatusBadRequest) // want "\\[G6.01\\]"
		return nil
	}
}

func closureHelper(w http.ResponseWriter, r *http.Request) error {
	fail := func(msg string) {
		http.Error(w, msg, http.StatusInternalServerError) // want "\\[G6.01\\]"
	}
	fail("boom")
	return nil
}

func deferredClosure(w http.ResponseWriter, r *http.Request) error {
	defer func() {
		http.Error(w, "deferred", http.StatusInternalServerError) // want "\\[G6.01\\]"
	}()
	return nil
}
