//go:build unit

package errors

import (
	"net/http"
	"testing"

	"go.uber.org/zap/zapcore"
)

// statusOf reads the status of a propagated error, which is always backed by an
// *ErrorT.
func statusOf(t *testing.T, err error) int {
	t.Helper()

	e, ok := err.(*ErrorT)
	if !ok {
		t.Fatalf("propagation returned %T, want *ErrorT", err)
	}

	return e.StatusCode()
}

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

func TestCustomizeStatusCode_preservesExactCode(t *testing.T) {
	for _, code := range []int{499, 599} {
		e := New(KindForStatus(code).CustomizeStatusCode(code), "upstream")
		if got := e.StatusCode(); got != code {
			t.Errorf("StatusCode() = %d, want %d", got, code)
		}
	}
}

func TestCustomizeStatusCode_ignoresNonPositiveCode(t *testing.T) {
	e := New(KindNotFoundError.CustomizeStatusCode(0).CustomizeStatusCode(-7), "missing")
	if got := e.StatusCode(); got != http.StatusNotFound {
		t.Errorf("StatusCode() = %d, want 404", got)
	}
}

func TestCustomizeStatusCode_survivesPropagation(t *testing.T) {
	newCause := func() *ErrorT { return New(KindSystemError.CustomizeStatusCode(599), "upstream") }

	cases := map[string]error{
		"Propagate":   Propagate(newCause(), "calling upstream"),
		"PropagateAs": PropagateAs(KindError, newCause(), "calling upstream"),
		"From":        From(KindError, newCause(), "calling upstream"),
		"Template":    Template(KindError, "calling upstream").Propagate(newCause()),
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
			if got := e.Kind().StatusCode(); got != 599 {
				t.Errorf("Kind().StatusCode() = %d, want 599", got)
			}
		})
	}

	if got := KindError.StatusCode(); got != http.StatusInternalServerError {
		t.Errorf("inheriting a status changed the registered kind to %d", got)
	}
}

func TestCustomizeStatusCode_targetWinsOverTheCause(t *testing.T) {
	cause := New(KindSystemError.CustomizeStatusCode(599), "upstream")

	e, ok := From(KindError.CustomizeStatusCode(http.StatusBadGateway), cause, "calling upstream").(*ErrorT)
	if !ok {
		t.Fatal("From should return an *ErrorT")
	}
	if got := e.StatusCode(); got != http.StatusBadGateway {
		t.Errorf("StatusCode() = %d, want 502", got)
	}
}

// A kind customized to the status it already defaults to still has to beat the
// one carried by the cause, otherwise a deliberate 400 would silently become the
// upstream's status.
func TestCustomizeStatusCode_explicitDefaultBeatsAnInheritedStatus(t *testing.T) {
	cause := New(KindSystemError.CustomizeStatusCode(599), "upstream")

	explicit, ok := PropagateAs(KindUserError.CustomizeStatusCode(http.StatusBadRequest), cause, "rejecting").(*ErrorT)
	if !ok {
		t.Fatal("PropagateAs should return an *ErrorT")
	}
	if got := explicit.StatusCode(); got != http.StatusBadRequest {
		t.Errorf("an explicit 400 reports %d, want 400", got)
	}

	inherited, ok := PropagateAs(KindUserError, cause, "rejecting").(*ErrorT)
	if !ok {
		t.Fatal("PropagateAs should return an *ErrorT")
	}
	if got := inherited.StatusCode(); got != 599 {
		t.Errorf("a default 400 reports %d, want the inherited 599", got)
	}
}

// The other half of the distinction above, on the cause side: a status the cause
// pinned on purpose is carried outward even when the wrapping call retypes the
// error, while a cause that merely reports its kind's default leaves the outer
// default alone.
func TestCustomizeStatusCode_explicitDefaultCauseSurvivesRetyping(t *testing.T) {
	pinned := func() *ErrorT { return New(KindUserError.CustomizeStatusCode(http.StatusBadRequest), "rejecting") }
	plain := func() *ErrorT { return New(KindUserError, "rejecting") }

	cases := map[string]struct {
		pinnedCause error
		plainCause  error
	}{
		"PropagateAs": {
			pinnedCause: PropagateAs(KindSystemError, pinned(), "handling"),
			plainCause:  PropagateAs(KindSystemError, plain(), "handling"),
		},
		"From": {
			pinnedCause: From(KindSystemError, pinned(), "handling"),
			plainCause:  From(KindSystemError, plain(), "handling"),
		},
		"Template": {
			pinnedCause: Template(KindSystemError, "handling").Propagate(pinned()),
			plainCause:  Template(KindSystemError, "handling").Propagate(plain()),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := statusOf(t, tc.pinnedCause); got != http.StatusBadRequest {
				t.Errorf("a cause pinned to 400 reports %d after retyping, want 400", got)
			}
			if got := statusOf(t, tc.plainCause); got != http.StatusInternalServerError {
				t.Errorf("a cause defaulting to 400 reports %d, want the outer 500", got)
			}
		})
	}
}

// A template that pins its own status keeps it whatever the cause reports, and
// using the template again changes neither its definition nor the registered
// kind it was built from.
func TestCustomizeStatusCode_customizedTemplateTargetWinsAndIsReusable(t *testing.T) {
	causes := map[string]*Kind{
		"a non-standard 599": KindSystemError.CustomizeStatusCode(599),
		"an explicit 404":    KindNotFoundError.CustomizeStatusCode(http.StatusNotFound),
		"an explicit 400":    KindUserError.CustomizeStatusCode(http.StatusBadRequest),
	}

	cases := map[string]struct {
		target *Kind
		want   int
	}{
		"target pinned to another status": {KindSystemError.CustomizeStatusCode(http.StatusBadGateway), http.StatusBadGateway},
		"target pinned to its default":    {KindSystemError.CustomizeStatusCode(http.StatusInternalServerError), http.StatusInternalServerError},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			template := Template(tc.target, "calling upstream")

			for causeName, causeKind := range causes {
				got := statusOf(t, template.Propagate(New(causeKind, "upstream")))
				if got != tc.want {
					t.Errorf("a cause carrying %s makes the template report %d, want %d", causeName, got, tc.want)
				}
			}

			if got := template.New().StatusCode(); got != tc.want {
				t.Errorf("the reused template reports %d, want %d", got, tc.want)
			}
		})
	}

	if got := KindSystemError.StatusCode(); got != http.StatusInternalServerError {
		t.Errorf("the registered kind reports %d after templated propagations, want 500", got)
	}
}

func TestCustomizeStatusCode_leavesKindMatchingUntouched(t *testing.T) {
	e := New(KindNotFoundError.CustomizeStatusCode(599), "missing")

	if !IsKind(e, KindNotFoundError) || !IsKind(e, KindUserError) {
		t.Error("a customized status should not change kind matching")
	}
	if e.Code() != KindNotFoundError.Code || e.ErrorDTO().Name != KindNotFoundError.FQN() {
		t.Errorf("the wire DTO %+v should be the one the original kind produces", e.ErrorDTO())
	}
	if got := KindNotFoundError.StatusCode(); got != http.StatusNotFound {
		t.Errorf("the registered kind reports %d, want 404", got)
	}
}

// ErrorT.StatusCode delegates to the kind, so the two must never disagree.
func TestCustomizeStatusCode_agreesWithTheKind(t *testing.T) {
	e := New(KindNotFoundError.CustomizeStatusCode(599), "missing")

	if e.StatusCode() != e.Kind().StatusCode() {
		t.Errorf("StatusCode() = %d, Kind().StatusCode() = %d", e.StatusCode(), e.Kind().StatusCode())
	}
	if got := e.StatusCode(); got != 599 {
		t.Errorf("StatusCode() = %d, want 599", got)
	}
}

func TestCustomizeStatusCode_reachesTheZapStatusField(t *testing.T) {
	enc := zapcore.NewMapObjectEncoder()
	if err := New(KindSystemError.CustomizeStatusCode(599), "upstream").MarshalLogObject(enc); err != nil {
		t.Fatalf("MarshalLogObject: %v", err)
	}

	if got := enc.Fields["error_status_code"]; got != 599 {
		t.Errorf("error_status_code = %v, want 599", got)
	}
	if got := enc.Fields["code"]; got != KindSystemError.Code {
		t.Errorf("code = %v, want %q", got, KindSystemError.Code)
	}
	if got := enc.Fields["kind"]; got != KindSystemError.FQN() {
		t.Errorf("kind = %v, want %q", got, KindSystemError.FQN())
	}
}
