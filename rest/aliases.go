package rest

import chi "github.com/go-chi/chi/v5"

// Type aliases for chi types so consumers can use the rest package
// instead of importing github.com/go-chi/chi/v5 directly.
type (
	Router       = chi.Router
	RouteContext = chi.Context
)

// RouteCtxKey is the context key used by chi to store route context.
var RouteCtxKey = chi.RouteCtxKey

// NewRouteContext creates a new chi route context, useful for testing.
var NewRouteContext = chi.NewRouteContext

// URLParam returns the URL parameter from a request's route context.
var URLParam = chi.URLParam
