//go:build unit

package rest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	chi "github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/luanguimaraesla/garlic/errors"
	"github.com/luanguimaraesla/garlic/logging"
)

func noop(http.ResponseWriter, *http.Request) {}

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
	if got := rec.Result().Header.Values("Allow"); len(got) != 0 {
		t.Errorf("Allow = %v, want no header", got)
	}
}

// The 405 answer is the same canonical DTO wherever the mismatch happens, and it
// never carries a generated Allow header.
func TestUnrouted_methodNotAllowedCarriesNoAllowHeader(t *testing.T) {
	server := NewServer("no-allow")
	server.Router().Get("/items", noop)

	sub := chi.NewRouter()
	sub.Get("/things", noop)
	server.Router().Mount("/api", sub)

	for name, target := range map[string]string{
		"root route":    "/items",
		"mounted route": "/api/things",
	} {
		t.Run(name, func(t *testing.T) {
			rec := serve(server.Router(), http.MethodPost, target)

			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want 405", rec.Code)
			}
			if got := rec.Result().Header.Values("Allow"); len(got) != 0 {
				t.Errorf("Allow = %v, want no header", got)
			}
			if dto := decodeDTO(t, rec); dto.Code != errors.KindForStatus(http.StatusMethodNotAllowed).Code {
				t.Errorf("kind = %q, want the 405 kind", dto.Code)
			}
		})
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
