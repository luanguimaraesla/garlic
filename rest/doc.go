// Package rest provides a Chi-based HTTP server with error-aware route handlers
// and JSON response helpers.
//
// # Server
//
// [GetServer] implements a multiton pattern — each name maps to one [Server]:
//
//	srv := rest.GetServer("api")
//	srv.Router().Use(middleware.Logging, middleware.Tracing)
//	rest.RegisterApp(srv.Router(), myApp)
//	errc := srv.Listen(ctx, ":8080")
//
// # Routes
//
// Route builders ([Get], [Post], [Put], [Patch], [Delete]) accept handler
// functions that return an error instead of writing failure responses directly:
//
//	rest.Get("/users/{id}", func(w http.ResponseWriter, r *http.Request) error {
//	    user, err := repo.Find(r.Context(), chi.URLParam(r, "id"))
//	    if err != nil {
//	        return err // automatically mapped to HTTP status via error kind
//	    }
//	    rest.WriteResponse(http.StatusOK, user).Must(w)
//	    return nil
//	})
//
// When a handler returns a non-nil error, the route wrapper logs it and calls
// [WriteError] to produce an appropriate JSON response. User errors are
// returned as-is; system errors are sanitized to prevent leaking internals.
//
// # App Interface
//
// Types implementing [App] expose a Routes method returning [Routes].
// Register them on a Chi router with [RegisterApp]:
//
//	type UserApp struct{ repo UserRepo }
//	func (a *UserApp) Routes() rest.Routes { return rest.Routes{rest.Get("/users", a.List)} }
//	rest.RegisterApp(srv.Router(), &UserApp{repo})
//
// # Response Helpers
//
// [WriteResponse] writes an arbitrary payload as JSON.
// [WriteMessage] writes a simple {"message": "..."} response.
// [WriteError] converts an error to a [DTO] with the correct HTTP status code.
// All return a [Response] whose Must method performs the write.
package rest
