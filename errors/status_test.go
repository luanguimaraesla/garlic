//go:build unit

package errors

import (
	"net/http"
	"testing"

	"go.uber.org/zap/zapcore"
)

func TestKindForStatus_exactForStandardStatuses(t *testing.T) {
	statuses := []int{
		http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden,
		http.StatusNotFound, http.StatusRequestTimeout, http.StatusConflict,
		http.StatusUnprocessableEntity, http.StatusTooManyRequests,
		http.StatusInternalServerError, http.StatusNotImplemented,
		http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout,
	}

	for _, s := range statuses {
		k := KindForStatus(s)
		if got := k.StatusCode(); got != s {
			t.Errorf("KindForStatus(%d).StatusCode() = %d, want %d", s, got, s)
		}
	}
}

func TestKindForStatus_secondaryKindsAreGeneric(t *testing.T) {
	// Every status resolves to its generic secondary kind, never to a tertiary
	// semantic kind.
	if KindForStatus(http.StatusUnauthorized) == KindAuthError {
		t.Error("401 should resolve to a secondary kind, not KindAuthError")
	}
	if KindForStatus(http.StatusForbidden) == KindForbiddenError {
		t.Error("403 should resolve to a secondary kind, not KindForbiddenError")
	}
	if KindForStatus(http.StatusNotFound) == KindNotFoundError {
		t.Error("404 should resolve to a secondary kind, not KindNotFoundError")
	}

	// Tertiary semantic kinds descend from their matching secondary kind.
	if !KindAuthError.Is(KindForStatus(http.StatusUnauthorized)) {
		t.Error("KindAuthError should descend from the 401 secondary kind")
	}
	if !KindForbiddenError.Is(KindForStatus(http.StatusForbidden)) {
		t.Error("KindForbiddenError should descend from the 403 secondary kind")
	}
	if !KindNotFoundError.Is(KindForStatus(http.StatusNotFound)) {
		t.Error("KindNotFoundError should descend from the 404 secondary kind")
	}
}

func TestClassForStatus_only4xxIsUserClass(t *testing.T) {
	if classForStatus(404) != KindUserError {
		t.Error("4xx should be user-class")
	}
	if classForStatus(302) != KindSystemError {
		t.Error("3xx should be system-class")
	}
	if classForStatus(503) != KindSystemError {
		t.Error("5xx should be system-class")
	}
}

func TestKindForStatus_classMembership(t *testing.T) {
	if !KindForStatus(http.StatusNotFound).Is(KindUserError) {
		t.Error("404 should be a user-class error")
	}
	if !KindForStatus(http.StatusServiceUnavailable).Is(KindSystemError) {
		t.Error("503 should be a system-class error")
	}
	if KindForStatus(http.StatusServiceUnavailable).Is(KindUserError) {
		t.Error("503 should not be a user-class error")
	}
}

func TestKindForStatus_standardKindsAreRegistered(t *testing.T) {
	// A generic (non-semantic) standard status is registered at init and
	// discoverable through the registry.
	k := KindForStatus(http.StatusInternalServerError) // 500 -> secondary S-kind
	if got, ok := LookupByCode(k.Code); !ok || got != k {
		t.Errorf("generic kind should be discoverable via LookupByCode: got %v, %v", got, ok)
	}
	if GetByCode(k.Code) != k {
		t.Error("generic kind should be discoverable via GetByCode")
	}
	if Get(k.Name) != k {
		t.Error("generic kind should be discoverable via Get")
	}
}

func TestKindForStatus_nonStandardFallsBackToClass(t *testing.T) {
	// 599 is not a standard status, so it falls back to its class base.
	if KindForStatus(599) != KindSystemError {
		t.Error("a non-standard 5xx status should fall back to KindSystemError")
	}
	// 460 is not a standard status, so it falls back to the 4xx class base.
	if KindForStatus(460) != KindUserError {
		t.Error("a non-standard 4xx status should fall back to KindUserError")
	}
}

func TestStatusCode_fallsBackToKind(t *testing.T) {
	cases := []struct {
		name string
		kind *Kind
		want int
	}{
		{"user class", KindUserError, http.StatusBadRequest},
		{"system class", KindSystemError, http.StatusInternalServerError},
		{"secondary", KindForStatus(http.StatusBadGateway), http.StatusBadGateway},
		{"tertiary", KindNotFoundError, http.StatusNotFound},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := New(tc.kind, "boom").StatusCode(); got != tc.want {
				t.Errorf("StatusCode() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestStatus_preservesExactCode(t *testing.T) {
	for _, code := range []int{499, 599} {
		e := New(KindForStatus(code), "upstream", Status(code))
		if got := e.StatusCode(); got != code {
			t.Errorf("StatusCode() = %d, want %d", got, code)
		}
	}
}

func TestStatus_ignoresNonPositiveCode(t *testing.T) {
	e := New(KindNotFoundError, "missing", Status(0), Status(-7))
	if got := e.StatusCode(); got != http.StatusNotFound {
		t.Errorf("StatusCode() = %d, want 404", got)
	}
}

func TestStatus_survivesPropagation(t *testing.T) {
	cause := New(KindSystemError, "upstream", Status(599))

	cases := map[string]error{
		"Propagate":   Propagate(cause, "calling upstream"),
		"PropagateAs": PropagateAs(KindError, cause, "calling upstream"),
		"From":        From(KindError, cause, "calling upstream"),
	}

	for name, err := range cases {
		t.Run(name, func(t *testing.T) {
			e, ok := err.(*ErrorT)
			if !ok {
				t.Fatalf("%s returned %T, want *ErrorT", name, err)
			}
			if got := e.StatusCode(); got != 599 {
				t.Errorf("StatusCode() = %d, want 599", got)
			}
		})
	}
}

func TestStatus_optWinsOverCarriedValue(t *testing.T) {
	cause := New(KindSystemError, "upstream", Status(599))

	e, ok := From(KindError, cause, "calling upstream", Status(502)).(*ErrorT)
	if !ok {
		t.Fatal("From should return an *ErrorT")
	}
	if got := e.StatusCode(); got != http.StatusBadGateway {
		t.Errorf("StatusCode() = %d, want 502", got)
	}
}

func TestStatus_leavesKindMatchingUntouched(t *testing.T) {
	e := New(KindNotFoundError, "missing", Status(599))

	if !IsKind(e, KindNotFoundError) || !IsKind(e, KindUserError) {
		t.Error("an overridden status should not change kind matching")
	}
	if got := e.Kind().StatusCode(); got != http.StatusNotFound {
		t.Errorf("Kind().StatusCode() = %d, want 404", got)
	}
}

func TestStatus_reachesTheZapStatusField(t *testing.T) {
	enc := zapcore.NewMapObjectEncoder()
	if err := New(KindSystemError, "upstream", Status(599)).MarshalLogObject(enc); err != nil {
		t.Fatalf("MarshalLogObject: %v", err)
	}

	if got := enc.Fields["error_status_code"]; got != 599 {
		t.Errorf("error_status_code = %v, want 599", got)
	}
}
