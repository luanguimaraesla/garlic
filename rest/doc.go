// Package rest provides a Chi-based HTTP server with error-aware route handlers
// and JSON response helpers.
//
// # Server
//
// [GetServer] implements a multiton pattern, each name maps to one [Server]:
//
//	srv := rest.GetServer("api")
//	srv.Router().Use(middleware.Logging, middleware.Tracing)
//	rest.RegisterApp(srv.Router(), myApp)
//	errc := srv.Listen(ctx, ":8080")
//
// # Graceful Shutdown
//
// When the context passed to [Server.Listen] is cancelled, the server
// drains in-flight requests before stopping. The shutdown timeout and
// an optional hook can be configured via [ServerOption] functions:
//
//	srv := rest.NewServer("api",
//	    rest.WithShutdownTimeout(10*time.Second),
//	    rest.WithOnShutdown(func(ctx context.Context) {
//	        db.Close()
//	        cache.Flush(ctx)
//	    }),
//	)
//
// [WithShutdownTimeout] sets the maximum time to wait for active
// connections to complete (default 30s). [WithOnShutdown] registers a
// callback invoked when shutdown begins; the provided context carries
// the shutdown deadline so cleanup work can respect the same timeout.
//
// # Unrouted Requests
//
// [NewServer], and therefore [GetServer], installs default NotFound and
// MethodNotAllowed handlers, so a request that matches no route gets the same
// canonical error DTO as any other garlic failure instead of chi's bare text
// page. Both write through [WriteError], with status 404 or 405 and a static
// hint.
//
// The 405 response carries no Allow header. chi drops its own Allow computation
// as soon as a custom MethodNotAllowed handler is installed and keeps the method
// set it computed unexported, and garlic deliberately does not rebuild it, so a
// client that needs the allowed methods has to consult the API documentation.
//
// Both handlers can be replaced through Router().NotFound and
// Router().MethodNotAllowed. chi copies the handlers into a sub-router at mount
// time and afterwards skips sub-routers that already carry one, so install an
// override before mounting if it should apply inside the mounts too; an override
// installed after a Mount replaces the root behavior while that sub-router keeps
// the garlic default.
//
// The handlers record no metrics of their own, because the monitoring middleware
// already instruments unrouted requests. A router with no routes at all is the
// exception: chi serves its NotFound directly, outside the middleware chain, so
// neither logging nor metrics middleware runs there.
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
// [WriteError] to produce an appropriate JSON response. [WriteError] is the one
// canonical error writer: the HTTP status comes from the error's kind, and what
// the body may say follows that kind's user or system classification, not the
// number. A user-class error crosses the wire in full. Every other error keeps
// its real HTTP status, but the body is sanitized to the generic kind for that
// status, so the standard status text, name, and code cross the wire while the
// specific kind's dynamic message and details do not. The specific kind's code
// is preserved as an origin reference so a client can still quote it to support.
//
// The classification and the status class do not have to agree numerically:
// [errors.Kind.CustomizeStatusCode] can put a system error under a 4xx status or
// a user error under a 5xx one, and the sanitization still follows the class.
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
// [WriteMessage] writes a simple {"message": "..."} response for non-error,
// informational payloads.
// [WriteError] converts an error to a sanitized [DTO] with the correct HTTP
// status code and is the canonical path for error responses.
// All return a [Response] whose Must method performs the write.
package rest
