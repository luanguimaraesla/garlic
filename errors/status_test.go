//go:build unit

package errors

import (
	"net/http"
	"testing"
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
