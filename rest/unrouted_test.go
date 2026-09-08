//go:build unit

package rest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"sort"
	"testing"

	chi "github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/luanguimaraesla/garlic/errors"
	"github.com/luanguimaraesla/garlic/logging"
)

// customMethod is registered once for the whole package because chi's method
// table is global and registering twice from separate tests would race.
const customMethod = "LINK"

func TestMain(m *testing.M) {
	chi.RegisterMethod(customMethod)
	os.Exit(m.Run())
}

func noop(http.ResponseWriter, *http.Request) {}

// routeTable registers the same routes on any chi router, so a garlic server and
// a plain chi router can be compared side by side.
type routeTable func(chi.Router)

func serve(handler http.Handler, method, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(method, target, nil))

	return rec
}

func decodeDTO(t *testing.T, rec *httptest.ResponseRecorder) *errors.DTO {
	t.Helper()

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}

	var dto errors.DTO
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatalf("decoding the error body %q: %v", rec.Body.String(), err)
	}

	return &dto
}

// allowHeader serves the request and returns the Allow values, sorted because
// chi builds its own header from map iteration and does not promise an order.
func allowHeader(t *testing.T, handler http.Handler, method, target string) []string {
	t.Helper()

	rec := serve(handler, method, target)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}

	allow := append([]string(nil), rec.Result().Header.Values("Allow")...)
	sort.Strings(allow)

	return allow
}

// assertAllowMatchesChi is the arbiter for the rebuilt Allow header: the garlic
// server and an unmodified chi router carrying the same routes must advertise
// the same method set.
func assertAllowMatchesChi(t *testing.T, routes routeTable, method, target string) []string {
	t.Helper()

	server := NewServer("differential")
	routes(server.Router())

	reference := chi.NewRouter()
	routes(reference)

	garlic := allowHeader(t, server.Router(), method, target)
	chiAllow := allowHeader(t, reference, method, target)

	if !reflect.DeepEqual(garlic, chiAllow) {
		t.Errorf("Allow = %v, chi says %v", garlic, chiAllow)
	}

	return garlic
}

func TestUnrouted_notFoundServesTheCanonicalDTO(t *testing.T) {
	servers = nil

	for name, router := range map[string]chi.Router{
		"NewServer": NewServer("nf").Router(),
		"GetServer": GetServer("nf-multiton").Router(),
	} {
		t.Run(name, func(t *testing.T) {
			router.Get("/items", noop)

			rec := serve(router, http.MethodGet, "/missing")
			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404", rec.Code)
			}

			dto := decodeDTO(t, rec)
			kind, ok := dto.Decode()
			if !ok {
				t.Fatalf("kind %q is not registered", dto.Code)
			}
			if kind.Kind() != errors.KindForStatus(http.StatusNotFound) {
				t.Errorf("kind = %s, want the 404 kind", kind.Kind().Name)
			}
			if !kind.Kind().Is(errors.KindUserError) {
				t.Error("an unrouted 404 should be a user-class error")
			}
			if dto.Details["hint"] == nil {
				t.Error("the DTO should carry a hint")
			}
		})
	}
}

func TestUnrouted_methodNotAllowedServesTheCanonicalDTO(t *testing.T) {
	server := NewServer("mna")
	server.Router().Get("/items", noop)

	rec := serve(server.Router(), http.MethodPost, "/items")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}

	dto := decodeDTO(t, rec)
	kind, ok := dto.Decode()
	if !ok {
		t.Fatalf("kind %q is not registered", dto.Code)
	}
	if kind.Kind() != errors.KindForStatus(http.StatusMethodNotAllowed) {
		t.Errorf("kind = %s, want the 405 kind", kind.Kind().Name)
	}
	if dto.Details["hint"] == nil {
		t.Error("the DTO should carry a hint")
	}
	if got := rec.Result().Header.Values("Allow"); !reflect.DeepEqual(got, []string{http.MethodGet}) {
		t.Errorf("Allow = %v, want [GET]", got)
	}
}

func TestUnrouted_allowMatchesChi(t *testing.T) {
	cases := []struct {
		name   string
		routes routeTable
		method string
		target string
		want   []string
	}{
		{
			name:   "simple mismatch",
			routes: func(r chi.Router) { r.Get("/items", noop); r.Delete("/items", noop) },
			method: http.MethodPost,
			target: "/items",
			want:   []string{http.MethodDelete, http.MethodGet},
		},
		{
			name:   "custom registered method",
			routes: func(r chi.Router) { r.Method(customMethod, "/items", http.HandlerFunc(noop)) },
			method: http.MethodPost,
			target: "/items",
			want:   []string{customMethod},
		},
		{
			name:   "encoded path",
			routes: func(r chi.Router) { r.Get("/a/{p}", noop) },
			method: http.MethodPost,
			target: "/a/b%2Fc",
			want:   []string{http.MethodGet},
		},
		{
			name: "root middleware rewrites the route path",
			routes: func(r chi.Router) {
				r.Use(func(next http.Handler) http.Handler {
					return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
						chi.RouteContext(req.Context()).RoutePath = "/items"
						next.ServeHTTP(w, req)
					})
				})
				r.Get("/items", noop)
			},
			method: http.MethodPost,
			target: "/anything",
			want:   []string{http.MethodGet},
		},
		{
			name: "mounted sub-router",
			routes: func(r chi.Router) {
				r.Get("/direct", noop)
				sub := chi.NewRouter()
				sub.Get("/items", noop)
				r.Mount("/api", sub)
			},
			method: http.MethodPost,
			target: "/api/items",
			want:   []string{http.MethodGet},
		},
		{
			name: "nested mounts",
			routes: func(r chi.Router) {
				inner := chi.NewRouter()
				inner.Get("/items", noop)
				outer := chi.NewRouter()
				outer.Mount("/v1", inner)
				r.Mount("/api", outer)
			},
			method: http.MethodPost,
			target: "/api/v1/items",
			want:   []string{http.MethodGet},
		},
		{
			name: "conflicting root path",
			routes: func(r chi.Router) {
				r.Put("/items", noop)
				sub := chi.NewRouter()
				sub.Get("/items", noop)
				r.Mount("/api", sub)
			},
			method: http.MethodPost,
			target: "/api/items",
			want:   []string{http.MethodGet},
		},
		{
			name: "cross-tree accumulation",
			routes: func(r chi.Router) {
				r.Put("/api/items", noop)
				sub := chi.NewRouter()
				sub.Get("/items", noop)
				r.Mount("/api", sub)
			},
			method: http.MethodPost,
			target: "/api/items",
			want:   []string{http.MethodGet, http.MethodPut},
		},
		{
			name: "mount endpoint",
			routes: func(r chi.Router) {
				sub := chi.NewRouter()
				sub.Get("/", noop)
				r.Mount("/api", sub)
			},
			method: http.MethodPost,
			target: "/api",
			want:   []string{http.MethodGet},
		},
		{
			name: "mount endpoint with a trailing slash",
			routes: func(r chi.Router) {
				sub := chi.NewRouter()
				sub.Get("/", noop)
				r.Mount("/api", sub)
			},
			method: http.MethodPost,
			target: "/api/",
			want:   []string{http.MethodGet},
		},
		{
			name: "mount endpoint beside a root route",
			routes: func(r chi.Router) {
				sub := chi.NewRouter()
				sub.Get("/", noop)
				r.Mount("/api", sub)
				r.Put("/api", noop)
			},
			method: http.MethodPost,
			target: "/api",
			want:   []string{http.MethodGet},
		},
		{
			name: "root route beside a parameter mount endpoint",
			routes: func(r chi.Router) {
				r.Get("/api", noop)
				sub := chi.NewRouter()
				sub.Put("/", noop)
				r.Mount("/{tenant}", sub)
			},
			method: http.MethodPost,
			target: "/api",
			want:   []string{http.MethodGet, http.MethodPut},
		},
		{
			name: "trailing-slash root route beside a parameter mount endpoint",
			routes: func(r chi.Router) {
				r.Get("/api/", noop)
				sub := chi.NewRouter()
				sub.Put("/", noop)
				r.Mount("/{tenant}", sub)
			},
			method: http.MethodPost,
			target: "/api",
			want:   []string{http.MethodPut},
		},
		{
			name: "concrete route beside a parameter mount endpoint",
			routes: func(r chi.Router) {
				r.Put("/t/7", noop)
				sub := chi.NewRouter()
				sub.Get("/", noop)
				r.Mount("/t/{id}", sub)
			},
			method: http.MethodPost,
			target: "/t/7",
			want:   []string{http.MethodGet, http.MethodPut},
		},
		{
			name: "trailing-slash concrete route beside a parameter mount endpoint",
			routes: func(r chi.Router) {
				r.Put("/t/7/", noop)
				sub := chi.NewRouter()
				sub.Get("/", noop)
				r.Mount("/t/{id}", sub)
			},
			method: http.MethodPost,
			target: "/t/7",
			want:   []string{http.MethodGet},
		},
		{
			name: "nested mount endpoint",
			routes: func(r chi.Router) {
				inner := chi.NewRouter()
				inner.Get("/", noop)
				outer := chi.NewRouter()
				outer.Mount("/v1", inner)
				r.Mount("/api", outer)
			},
			method: http.MethodPost,
			target: "/api/v1",
			want:   []string{http.MethodGet},
		},
		{
			name: "param-carrying mount endpoint",
			routes: func(r chi.Router) {
				sub := chi.NewRouter()
				sub.Get("/", noop)
				r.Mount("/t/{tid}/api", sub)
			},
			method: http.MethodPost,
			target: "/t/7/api",
			want:   []string{http.MethodGet},
		},
		{
			name: "root route beside a catch-all mount",
			routes: func(r chi.Router) {
				sub := chi.NewRouter()
				sub.Put("/items", noop)
				r.Get("/", noop)
				r.Mount("/", sub)
			},
			method: http.MethodPost,
			target: "/",
			want:   []string{http.MethodGet},
		},
		{
			name: "param-carrying mount",
			routes: func(r chi.Router) {
				sub := chi.NewRouter()
				sub.Get("/items", noop)
				r.Mount("/t/{tid}/api", sub)
			},
			method: http.MethodPost,
			target: "/t/7/api/items",
			want:   []string{http.MethodGet},
		},
		{
			name:   "unsupported method on an existing path",
			routes: func(r chi.Router) { r.Get("/items", noop) },
			method: "FROB",
			target: "/items",
			want:   nil,
		},
		{
			name:   "unsupported method on a missing path",
			routes: func(r chi.Router) { r.Get("/items", noop) },
			method: "FROB",
			target: "/nowhere",
			want:   nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := assertAllowMatchesChi(t, tc.routes, tc.method, tc.target)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Allow = %v, want %v", got, tc.want)
			}
		})
	}
}

// A middleware that rewrites the route path from inside a mounted router is the
// one case the rebuilt header cannot reproduce: chi accumulated its Allow while
// routing the original path, and the method set it computed is unexported. The
// probe therefore sees only what the rewritten tail can reach.
func TestUnrouted_allowDivergesOnMountInternalRewrite(t *testing.T) {
	routes := func(r chi.Router) {
		sub := chi.NewRouter()
		sub.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				chi.RouteContext(req.Context()).RoutePath = "/other"
				next.ServeHTTP(w, req)
			})
		})
		sub.Get("/items", noop)
		sub.Put("/other", noop)
		r.Mount("/api", sub)
		r.Delete("/api/items", noop)
	}

	server := NewServer("divergence")
	routes(server.Router())

	reference := chi.NewRouter()
	routes(reference)

	garlic := allowHeader(t, server.Router(), http.MethodPost, "/api/items")
	if !reflect.DeepEqual(garlic, []string{http.MethodPut}) {
		t.Errorf("Allow = %v, want the rewritten tail's [PUT]", garlic)
	}

	chiAllow := allowHeader(t, reference, http.MethodPost, "/api/items")
	if reflect.DeepEqual(garlic, chiAllow) {
		t.Errorf("the divergence is gone: chi now also reports %v, revisit rest/doc.go", chiAllow)
	}
}

func TestUnrouted_overridesReplaceTheDefaults(t *testing.T) {
	server := NewServer("overrides")
	server.Router().Get("/items", noop)
	server.Router().NotFound(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("custom 404"))
	})
	server.Router().MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = w.Write([]byte("custom 405"))
	})

	if got := serve(server.Router(), http.MethodGet, "/missing").Body.String(); got != "custom 404" {
		t.Errorf("404 body = %q, want the override", got)
	}
	if got := serve(server.Router(), http.MethodPost, "/items").Body.String(); got != "custom 405" {
		t.Errorf("405 body = %q, want the override", got)
	}
}

// chi copies the unrouted handlers into a sub-router at mount time and later
// skips sub-routers that already carry one, so the order of Mount and the
// override decides what the sub-router serves.
func TestUnrouted_overrideOrderDecidesMountedBehavior(t *testing.T) {
	cases := map[string]struct {
		status   int
		install  func(chi.Router, http.HandlerFunc)
		method   string
		target   string
		unrouted string
	}{
		"not found": {
			status:   http.StatusNotFound,
			install:  func(r chi.Router, h http.HandlerFunc) { r.NotFound(h) },
			method:   http.MethodGet,
			target:   "/api/missing",
			unrouted: "/missing",
		},
		"method not allowed": {
			status:   http.StatusMethodNotAllowed,
			install:  func(r chi.Router, h http.HandlerFunc) { r.MethodNotAllowed(h) },
			method:   http.MethodPost,
			target:   "/api/items",
			unrouted: "/root",
		},
	}

	for name, tc := range cases {
		custom := func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(tc.status)
			_, _ = w.Write([]byte("custom"))
		}

		mount := func(r chi.Router) {
			sub := chi.NewRouter()
			sub.Get("/items", noop)
			r.Mount("/api", sub)
			r.Get("/root", noop)
		}

		t.Run(name+": override before mount propagates", func(t *testing.T) {
			server := NewServer(name + "-before")
			tc.install(server.Router(), custom)
			mount(server.Router())

			if got := serve(server.Router(), tc.method, tc.target).Body.String(); got != "custom" {
				t.Errorf("mounted body = %q, want the override", got)
			}
		})

		t.Run(name+": override after mount keeps the garlic default inside the mount", func(t *testing.T) {
			server := NewServer(name + "-after")
			mount(server.Router())
			tc.install(server.Router(), custom)

			rec := serve(server.Router(), tc.method, tc.target)
			if dto := decodeDTO(t, rec); dto.Code != errors.KindForStatus(tc.status).Code {
				t.Errorf("mounted kind = %q, want the garlic default", dto.Code)
			}
			if got := serve(server.Router(), tc.method, tc.unrouted).Body.String(); got != "custom" {
				t.Errorf("root body = %q, want the override", got)
			}
		})
	}
}

func TestUnrouted_mountedRoutersInheritTheDefaults(t *testing.T) {
	server := NewServer("mounted")

	plain := chi.NewRouter()
	plain.Get("/items", noop)
	server.Router().Mount("/api", plain)

	opinionated := chi.NewRouter()
	opinionated.Get("/items", noop)
	opinionated.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("sub 404"))
	})
	server.Router().Mount("/own", opinionated)

	if dto := decodeDTO(t, serve(server.Router(), http.MethodGet, "/api/missing")); dto.Details["hint"] == nil {
		t.Error("a plain mounted router should inherit the garlic 404")
	}
	if got := serve(server.Router(), http.MethodGet, "/own/missing").Body.String(); got != "sub 404" {
		t.Errorf("mounted 404 body = %q, want the sub-router's own handler", got)
	}
}

func TestUnrouted_emptyRouterServesTheDefault404(t *testing.T) {
	rec := serve(NewServer("empty").Router(), http.MethodGet, "/anything")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if dto := decodeDTO(t, rec); dto.Code != errors.KindForStatus(http.StatusNotFound).Code {
		t.Errorf("kind = %q, want the 404 kind", dto.Code)
	}
}

func TestUnrouted_middlewareStillRegistersAndRuns(t *testing.T) {
	server := NewServer("middleware")
	server.Router().Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Marker", "seen")
			next.ServeHTTP(w, r)
		})
	})
	server.Router().Get("/items", noop)

	for _, tc := range []struct{ method, target string }{
		{http.MethodGet, "/missing"},
		{http.MethodPost, "/items"},
	} {
		rec := serve(server.Router(), tc.method, tc.target)
		if got := rec.Header().Get("X-Marker"); got != "seen" {
			t.Errorf("%s %s: X-Marker = %q, middleware did not run", tc.method, tc.target, got)
		}
	}
}

func TestUnrouted_worksWithAndWithoutAContextLogger(t *testing.T) {
	server := NewServer("logger")
	server.Router().Get("/items", noop)

	if rec := serve(server.Router(), http.MethodPost, "/items"); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status without a context logger = %d, want 405", rec.Code)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/items", nil)
	server.Router().ServeHTTP(rec, req.WithContext(logging.SetContextLogger(req.Context(), zap.NewNop())))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status with a context logger = %d, want 405", rec.Code)
	}
}
