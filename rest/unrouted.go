package rest

import (
	"context"
	"net/http"
	"sort"
	"strings"

	chi "github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/luanguimaraesla/garlic/errors"
	"github.com/luanguimaraesla/garlic/logging"
)

// notFoundHandler answers a request whose path matches no route with the
// canonical error DTO, so an unrouted 404 reads like every other garlic error
// instead of chi's bare text page.
func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	logUnrouted(r, "No route matches the request path.")

	WriteError(errors.Mirror(
		errors.KindForStatus(http.StatusNotFound),
		errors.Hint("No route matches the requested path."),
	)).Must(w)
}

// methodNotAllowedHandler answers a method mismatch with the canonical error
// DTO. chi drops its own Allow computation as soon as a custom handler is
// installed and keeps the method set it computed private, so the header is
// rebuilt here by probing the router.
func methodNotAllowedHandler(w http.ResponseWriter, r *http.Request) {
	logUnrouted(r, "No route matches the request method.")

	for _, method := range allowedMethods(r) {
		w.Header().Add("Allow", method)
	}

	WriteError(errors.Mirror(
		errors.KindForStatus(http.StatusMethodNotAllowed),
		errors.Hint("The request method is not allowed for this path; see the Allow header."),
	)).Must(w)
}

// allowedMethods rebuilds the Allow header chi would have produced. It asks the
// root router which methods can serve this request, because the routing context
// always points at the root while a mounted router has already shifted the path
// it routes.
//
// It advertises nothing outside a chi router, and nothing for a request method
// chi does not know: that check runs before any path lookup, so chi answers such
// a request with a bare 405 even on a path that exists.
func allowedMethods(r *http.Request) []string {
	rctx := chi.RouteContext(r.Context())
	if rctx == nil || rctx.Routes == nil {
		return nil
	}

	method := rctx.RouteMethod
	if method == "" {
		method = r.Method
	}
	if !methodSupported(method) {
		return nil
	}

	path, ok := rootRoutePath(rctx, r)
	if !ok {
		logUnrouted(r, "Cannot rebuild the root routing path; omitting the Allow header.")
		return nil
	}

	var allowed []string
	for _, candidate := range candidateMethods(rctx.Routes) {
		if routeMatches(rctx.Routes, candidate, path) {
			allowed = append(allowed, candidate)
		}
	}

	return allowed
}

// routeMatches reports whether routes can serve method at path, following a
// match that stopped at a mount endpoint into the router mounted there. chi
// answers the two endpoints of a mount, /api and /api/, with a stub that accepts
// every method before consulting what was mounted, so taking that answer at face
// value would advertise methods no route can serve.
func routeMatches(routes chi.Routes, method, path string) bool {
	// Match mutates the context it is handed, so every probe gets a fresh one.
	rctx := chi.NewRouteContext()
	if !routes.Match(rctx, method, path) {
		return false
	}

	mounted, ok := mountEndpoint(routes, rctx.RoutePatterns)
	if !ok {
		return true
	}

	// A mount endpoint is a static pattern, so it consumed the whole path and
	// leaves the mounted router routing its own root.
	return routeMatches(mounted, method, "/")
}

// mountEndpoint returns the router mounted at the endpoint the matched patterns
// stopped on, and reports false when they stopped on an ordinary route. Every
// pattern but the last is the wildcard chi records for a mount, so the walk
// descends through them to reach the level the last one belongs to.
func mountEndpoint(routes chi.Routes, patterns []string) (chi.Routes, bool) {
	if len(patterns) == 0 {
		return nil, false
	}

	level := routes
	for _, pattern := range patterns[:len(patterns)-1] {
		next, ok := mountedAt(level, pattern)
		if !ok {
			return nil, false
		}

		level = next
	}

	last := patterns[len(patterns)-1]
	if hasRoute(level, last) {
		return nil, false
	}

	return mountedAt(level, strings.TrimSuffix(last, "/")+"/*")
}

// hasRoute reports whether pattern names a route of its own. chi lists a mount
// under its wildcard alone, so a listed pattern belongs to a real handler and
// cannot be a mount stub.
func hasRoute(routes chi.Routes, pattern string) bool {
	for _, route := range routes.Routes() {
		if route.Pattern == pattern && route.SubRoutes == nil {
			return true
		}
	}

	return false
}

// mountedAt returns the router chi mounted under pattern.
func mountedAt(routes chi.Routes, pattern string) (chi.Routes, bool) {
	for _, route := range routes.Routes() {
		if route.Pattern == pattern && route.SubRoutes != nil {
			return route.SubRoutes, true
		}
	}

	return nil, false
}

// methodSupported reports whether chi can route the given request method. chi
// keeps its method table private, so the only way to ask is to build a throwaway
// router that answers every method it knows. Building it per call, on the rare
// 405 path, keeps methods added through chi.RegisterMethod visible however late
// they were registered.
func methodSupported(method string) bool {
	probe := chi.NewRouter()
	probe.Handle("/", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	return probe.Match(chi.NewRouteContext(), method, "/")
}

// candidateMethods lists every method registered anywhere in the tree, sorted so
// the header is deterministic. chi's own Allow order comes from map iteration
// and is not.
func candidateMethods(routes chi.Routes) []string {
	found := map[string]struct{}{}
	collectMethods(routes, found)

	methods := make([]string, 0, len(found))
	for method := range found {
		methods = append(methods, method)
	}
	sort.Strings(methods)

	return methods
}

func collectMethods(routes chi.Routes, found map[string]struct{}) {
	for _, route := range routes.Routes() {
		for method := range route.Handlers {
			// "*" is chi's aggregate entry for a catch-all registration; the
			// per-method keys beside it already name the real methods.
			if method != "*" {
				found[method] = struct{}{}
			}
		}

		if route.SubRoutes != nil {
			collectMethods(route.SubRoutes, found)
		}
	}
}

// rootRoutePath rebuilds the path as the root router sees it. Mounting shifts
// the routed path past the mount prefix, so probing the root with that tail
// alone would miss every route outside the mount. The patterns matched on the
// way in are replayed, with the parameter values they captured, to restore the
// prefix.
//
// It reports false when patterns and values do not line up, so the caller can
// omit the header instead of advertising a wrong one.
func rootRoutePath(rctx *chi.Context, r *http.Request) (string, bool) {
	var prefix strings.Builder

	used := 0
	for _, pattern := range rctx.RoutePatterns {
		if used > len(rctx.URLParams.Values) {
			return "", false
		}

		hop, consumed, ok := expandPattern(pattern, rctx.URLParams.Values[used:])
		if !ok {
			return "", false
		}

		prefix.WriteString(hop)
		used += consumed
	}

	tail := activeRoutePath(rctx, r)

	// A mount endpoint already spells the whole path out, and the root a mount
	// hands down is a synthetic child rather than something the caller wrote.
	// Appending it would probe /api/ for a request to /api, and chi routes those
	// two paths separately.
	if tail == "/" && atMountEndpoint(rctx.RoutePatterns) {
		return prefix.String(), true
	}

	return joinRoutePath(prefix.String(), tail), true
}

// atMountEndpoint reports whether the last pattern matched on the way in is a
// complete endpoint, as opposed to the wildcard prefix under which a mount also
// answers everything below it.
func atMountEndpoint(patterns []string) bool {
	if len(patterns) == 0 {
		return false
	}

	return !strings.HasSuffix(patterns[len(patterns)-1], "/*")
}

// joinRoutePath glues a replayed prefix to the path the current router is
// routing. A request that lands on the trailing-slash endpoint of a mount is
// recorded with that slash already in the pattern, while the tail a mount hands
// down always opens with one, and the doubled separator matches no route.
func joinRoutePath(prefix, tail string) string {
	if strings.HasSuffix(prefix, "/") && strings.HasPrefix(tail, "/") {
		return prefix + tail[1:]
	}

	return prefix + tail
}

// activeRoutePath is the path the current router is routing, in chi's own
// preference order, so both a rewritten route path and an encoded URL survive.
func activeRoutePath(rctx *chi.Context, r *http.Request) string {
	if rctx.RoutePath != "" {
		return rctx.RoutePath
	}
	if r.URL.RawPath != "" {
		return r.URL.RawPath
	}
	if r.URL.Path != "" {
		return r.URL.Path
	}

	return "/"
}

// expandPattern turns one matched mount pattern back into the concrete prefix it
// stood for, substituting the recorded parameter values in order, and reports
// how many values it consumed.
func expandPattern(pattern string, values []string) (string, int, bool) {
	// A mount registers its subtree under a trailing wildcard, which chi records
	// as one more (blanked) parameter value.
	wildcard := 0
	if trimmed := strings.TrimSuffix(pattern, "/*"); trimmed != pattern {
		wildcard = 1
		pattern = trimmed
	}

	var prefix strings.Builder

	used := 0
	for {
		open := strings.IndexByte(pattern, '{')
		if open < 0 {
			prefix.WriteString(pattern)
			break
		}

		end := closingBrace(pattern[open:])
		if end < 0 || used >= len(values) {
			return "", 0, false
		}

		prefix.WriteString(pattern[:open])
		prefix.WriteString(values[used])
		used++
		pattern = pattern[open+end+1:]
	}

	if used+wildcard > len(values) {
		return "", 0, false
	}

	return prefix.String(), used + wildcard, true
}

// closingBrace returns the index of the brace closing the one that opens s, or
// -1 when they are unbalanced. Nesting matters because a chi parameter can carry
// a regex, as in {id:[0-9]{3}}.
func closingBrace(s string) int {
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}

	return -1
}

func logUnrouted(r *http.Request, message string) {
	unroutedLogger(r.Context()).Warn(message,
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
	)
}

// unroutedLogger prefers the request logger the middleware installs and falls
// back to the global one, because an unrouted request can reach these handlers
// before the middleware chain, or entirely outside it when the router holds no
// routes at all.
func unroutedLogger(ctx context.Context) *zap.Logger {
	if logger, ok := ctx.Value(logging.LoggerKey).(*zap.Logger); ok && logger != nil {
		return logger
	}

	return logging.Global()
}
