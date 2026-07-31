//go:build unit
// +build unit

package errors

import (
	"fmt"
	"net/http"
	"testing"
)

// trackingOpt counts how often it is applied, so a test can prove that a nil
// input short-circuits before any option runs.
type trackingOpt struct {
	calls int
}

func (o *trackingOpt) Opt(*ErrorT) {
	o.calls++
}

// requireNilError observes the result at the error interface boundary, so a
// typed nil pointer returned as an error still fails.
func requireNilError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("expected a nil error interface, got %v of type %T", err, err)
	}
}

// requireErrorT asserts the outer dynamic type instead of walking the chain, so
// a result that only wraps an *ErrorT deeper down cannot pass by accident.
func requireErrorT(t *testing.T, err error) *ErrorT {
	t.Helper()
	e, ok := err.(*ErrorT)
	if !ok {
		t.Fatalf("expected result of dynamic type *ErrorT, got %T", err)
	}

	return e
}

// --- nil input contract ---

func TestPropagate_nilReturnsNil(t *testing.T) {
	opt := &trackingOpt{}

	requireNilError(t, Propagate(nil, "must not be built", opt))

	if opt.calls != 0 {
		t.Errorf("expected no option to run for a nil error, got %d calls", opt.calls)
	}
}

func TestPropagateAs_nilReturnsNil(t *testing.T) {
	opt := &trackingOpt{}

	requireNilError(t, PropagateAs(KindSystemError, nil, "must not be built", opt))

	if opt.calls != 0 {
		t.Errorf("expected no option to run for a nil error, got %d calls", opt.calls)
	}
}

func TestFrom_nilReturnsNil(t *testing.T) {
	opt := &trackingOpt{}

	requireNilError(t, From(KindError, nil, "no cause", opt))

	if opt.calls != 0 {
		t.Errorf("expected no option to run for a nil error, got %d calls", opt.calls)
	}
}

func TestTemplate_propagateNilReturnsNil(t *testing.T) {
	templateOpt := &trackingOpt{}
	callerOpt := &trackingOpt{}
	tmpl := Template(KindNotFoundError, "not found", templateOpt)

	requireNilError(t, tmpl.Propagate(nil, callerOpt))

	if templateOpt.calls != 0 {
		t.Errorf("expected no template option to run for a nil error, got %d calls", templateOpt.calls)
	}
	if callerOpt.calls != 0 {
		t.Errorf("expected no caller option to run for a nil error, got %d calls", callerOpt.calls)
	}
}

// --- wrap() behavior ---

func TestWrap_copiesDetailsFromWrappedError(t *testing.T) {
	inner := New(KindNotFoundError, "not found",
		Hint("check the ID"),
	)

	outer := requireErrorT(t, Propagate(inner, "service failed"))
	if outer.Details["hint"] != "check the ID" {
		t.Errorf("expected hint to be copied from wrapped error, got %v", outer.Details["hint"])
	}
}

func TestFrom_nonNilPreservesConstruction(t *testing.T) {
	cause := fmt.Errorf("root cause")
	err := requireErrorT(t, From(KindNotFoundError, cause, "not found", Hint("check the ID")))

	if err.Kind() != KindNotFoundError {
		t.Errorf("kind: want NotFoundError, got %s", err.Kind().Name)
	}
	if err.message != "not found" {
		t.Errorf("message: want 'not found', got %q", err.message)
	}
	if err.Unwrap() != cause {
		t.Error("From should keep the exact cause")
	}
	if err.Details["hint"] != "check the ID" {
		t.Errorf("hint: want 'check the ID', got %v", err.Details["hint"])
	}
	if len(err.Troubleshooting.ReverseTrace) != 0 {
		t.Errorf("From should not append a reverse trace, got %v", err.Troubleshooting.ReverseTrace)
	}
}

func TestWrap_stdlibError_doesNotCopyDetails(t *testing.T) {
	stdErr := fmt.Errorf("stdlib error")
	e := requireErrorT(t, Propagate(stdErr, "wrapped"))
	if len(e.Details) != 0 {
		t.Errorf("expected empty Details when wrapping stdlib error, got %v", e.Details)
	}
}

// --- Propagate kind inheritance ---

func TestPropagate_inheritsKindFromErrorT(t *testing.T) {
	inner := New(KindNotFoundError, "not found")
	outer := requireErrorT(t, Propagate(inner, "service layer"))

	if !outer.Kind().Is(KindNotFoundError) {
		t.Errorf("expected NotFoundError kind, got %s", outer.Kind().Name)
	}
}

func TestPropagate_defaultsToKindErrorForStdlib(t *testing.T) {
	outer := requireErrorT(t, Propagate(fmt.Errorf("raw"), "service layer"))

	if outer.Kind() != KindError {
		t.Errorf("expected KindError for stdlib error, got %s", outer.Kind().Name)
	}
}

func TestPropagateAs_overridesKind(t *testing.T) {
	inner := New(KindSystemError, "db down")
	outer := requireErrorT(t, PropagateAs(KindNotFoundError, inner, "not found"))

	if !outer.Kind().Is(KindNotFoundError) {
		t.Errorf("expected NotFoundError, got %s", outer.Kind().Name)
	}
}

// Propagation must carry every piece of the inner error outward untouched and
// add exactly one trace entry for the boundary it just crossed.
func TestPropagate_nonNilPreservesMetadataAndTrace(t *testing.T) {
	inner := New(KindNotFoundError, "record not found",
		Hint("check the ID"),
		Context(Field("resource_id", "abc")),
		StackTrace(),
	)
	innerTrace := append([]string(nil), inner.Troubleshooting.ReverseTrace...)
	innerStack := inner.Troubleshooting.StackTrace
	if innerStack == "" {
		t.Fatal("the seeded stack trace should not be empty")
	}

	outer := requireErrorT(t, Propagate(inner, "service failed"))

	if outer.Kind() != KindNotFoundError {
		t.Errorf("kind: want NotFoundError, got %s", outer.Kind().Name)
	}
	if outer.Unwrap() != error(inner) {
		t.Error("propagation should keep the exact cause")
	}
	if outer.Details["hint"] != "check the ID" {
		t.Errorf("details: want the inner hint, got %v", outer.Details["hint"])
	}
	if outer.Troubleshooting.StackTrace != innerStack {
		t.Errorf("stack trace: want the inner stack trace, got %q", outer.Troubleshooting.StackTrace)
	}

	// Context is keyed by caller function name.
	if len(outer.Troubleshooting.Context) != 1 {
		t.Fatalf("troubleshooting context: want 1 caller entry, got %d", len(outer.Troubleshooting.Context))
	}
	for caller, ctx := range outer.Troubleshooting.Context {
		fields, ok := ctx.(map[string]any)
		if !ok {
			t.Fatalf("troubleshooting context for %s: want map[string]any, got %T", caller, ctx)
		}
		if len(fields) != 1 {
			t.Errorf("troubleshooting context for %s: want 1 field, got %v", caller, fields)
		}
		if fields["resource_id"] != "abc" {
			t.Errorf("resource_id: want abc, got %v", fields["resource_id"])
		}
	}

	trace := outer.Troubleshooting.ReverseTrace
	if len(trace) != len(innerTrace)+1 {
		t.Fatalf("reverse trace: want exactly one appended entry over %d, got %d", len(innerTrace), len(trace))
	}
	for i, want := range innerTrace {
		if trace[i] != want {
			t.Errorf("reverse trace entry %d: want %q, got %q", i, want, trace[i])
		}
	}
}

// --- Error chain ---

func TestError_chainsMessages(t *testing.T) {
	cause := fmt.Errorf("connection refused")
	err := Propagate(cause, "failed to connect")

	want := "failed to connect: connection refused"
	if err.Error() != want {
		t.Errorf("want %q, got %q", want, err.Error())
	}
}

func TestUnwrap_returnsCause(t *testing.T) {
	cause := fmt.Errorf("root cause")
	err := requireErrorT(t, Propagate(cause, "wrapper"))

	if err.Unwrap() != cause {
		t.Error("Unwrap should return the cause")
	}
}

// --- Multilayer propagation (handler -> service -> repo) ---

func TestMultilayerPropagation_preservesKindAndChain(t *testing.T) {
	// Simulate repo -> service -> handler propagation
	repoErr := New(KindNotFoundError, "record not found in database",
		Hint("check the ID"),
	)
	svcErr := Propagate(repoErr, "failed to get user")
	handlerErr := Propagate(svcErr, "failed to read user")

	// Kind should be preserved through the chain
	if !IsKind(handlerErr, KindNotFoundError) {
		t.Error("NotFoundError kind lost through propagation chain")
	}
	if !IsKind(handlerErr, KindUserError) {
		t.Error("parent UserError kind should match through hierarchy")
	}

	// Message chain should be readable
	want := "failed to read user: failed to get user: record not found in database"
	if handlerErr.Error() != want {
		t.Errorf("message chain:\nwant: %s\ngot:  %s", want, handlerErr.Error())
	}
}

// --- Error context ---

func TestContext_fieldsAvailableInTroubleshooting(t *testing.T) {
	err := New(KindSystemError, "cache miss",
		Context(
			Field("cache_key", "user:123"),
			Field("ttl", 300),
		),
	)

	if err.Troubleshooting.Context == nil {
		t.Fatal("expected non-nil Context in Troubleshooting")
	}

	// Context is keyed by caller function name
	found := false
	for _, ctx := range err.Troubleshooting.Context {
		if m, ok := ctx.(map[string]any); ok {
			if m["cache_key"] == "user:123" {
				found = true
			}
		}
	}
	if !found {
		t.Error("cache_key field not found in Troubleshooting.Context")
	}
}

func TestHint_appearsInDetails(t *testing.T) {
	err := New(KindInvalidRequestError, "bad input",
		Hint("check field %s", "email"),
	)

	if err.Details["hint"] != "check field email" {
		t.Errorf("want 'check field email', got %v", err.Details["hint"])
	}
}

func TestHint_lastOneWins(t *testing.T) {
	err := New(KindInvalidRequestError, "bad input",
		Hint("first"),
		Hint("second"),
	)

	if err.Details["hint"] != "second" {
		t.Errorf("expected last hint to win, got %v", err.Details["hint"])
	}
}

// --- Template ---

func TestTemplate_callerOptsOverrideTemplateOpts(t *testing.T) {
	tmpl := Template(KindNotFoundError, "not found",
		Hint("template hint"),
	)

	err := tmpl.New(Hint("caller hint"))
	if err.Details["hint"] != "caller hint" {
		t.Errorf("caller hint should override template hint, got %v", err.Details["hint"])
	}
}

func TestTemplate_propagatePreservesKind(t *testing.T) {
	tmpl := Template(KindNotFoundError, "not found")
	cause := fmt.Errorf("sql: no rows")

	err := requireErrorT(t, tmpl.Propagate(cause))
	if !IsKind(err, KindNotFoundError) {
		t.Errorf("expected NotFoundError kind, got %s", err.Kind().Name)
	}
	if err.Unwrap() != cause {
		t.Error("Propagate should wrap the cause")
	}
}

func TestTemplate_callerContextMergesWithTemplate(t *testing.T) {
	tmpl := Template(KindNotFoundError, "not found",
		Hint("template hint"),
	)

	err := tmpl.New(
		Context(Field("resource_id", "abc")),
	)

	if err.Troubleshooting.Context == nil {
		t.Fatal("expected context from caller opts")
	}
}

// --- Kind hierarchy ---

func TestKind_StatusCode_traversesHierarchy(t *testing.T) {
	if KindInvalidRequestError.StatusCode() != http.StatusBadRequest {
		t.Errorf("ValidationError: want 400, got %d", KindInvalidRequestError.StatusCode())
	}
	if KindNotFoundError.StatusCode() != http.StatusNotFound {
		t.Errorf("NotFoundError: want 404, got %d", KindNotFoundError.StatusCode())
	}
}

func TestKind_FQN_buildsFullHierarchy(t *testing.T) {
	fqn := KindInvalidRequestError.FQN()
	want := "InvalidRequestError::HTTP400Error::UserError::Error"
	if fqn != want {
		t.Errorf("FQN: want %s, got %s", want, fqn)
	}
}

func TestKind_Is_matchesSelfAndParents(t *testing.T) {
	if !KindInvalidRequestError.Is(KindInvalidRequestError) {
		t.Error("should match self")
	}
	if !KindInvalidRequestError.Is(KindForStatus(http.StatusBadRequest)) {
		t.Error("should match secondary parent")
	}
	if !KindInvalidRequestError.Is(KindUserError) {
		t.Error("should match ancestor")
	}
	if KindInvalidRequestError.Is(KindSystemError) {
		t.Error("should not match unrelated kind")
	}
}

// --- DTO ---

func TestErrorDTO_containsKindAndDetails(t *testing.T) {
	err := New(KindInvalidRequestError, "email invalid",
		Hint("provide a valid email"),
	)

	dto := err.ErrorDTO()
	if dto.Name != KindInvalidRequestError.FQN() {
		t.Errorf("name: want %s, got %s", KindInvalidRequestError.FQN(), dto.Name)
	}
	if dto.Code != KindInvalidRequestError.Code {
		t.Errorf("code: want %s, got %s", KindInvalidRequestError.Code, dto.Code)
	}
	if dto.Error != "email invalid" {
		t.Errorf("error: want 'email invalid', got %s", dto.Error)
	}
	if dto.Details["hint"] != "provide a valid email" {
		t.Errorf("hint: want 'provide a valid email', got %v", dto.Details["hint"])
	}
}

func TestNewDTO_wrapsNonErrorT(t *testing.T) {
	stdErr := fmt.Errorf("plain error")
	dto := NewDTO(stdErr)

	if dto.Error != "plain error" {
		t.Errorf("error: want 'plain error', got %s", dto.Error)
	}
	if dto.Code != KindError.Code {
		t.Errorf("code: want %s (KindError), got %s", KindError.Code, dto.Code)
	}
}

// --- Register ---

func TestGetByCode_panicsForUnknown(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for unknown code")
		}
	}()
	GetByCode("NONEXISTENT")
}

func TestGet_panicsForUnknown(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for unknown name")
		}
	}()
	Get("NonexistentErrorKind")
}

// --- RedactedString ---

func TestRedactedString_shortValue(t *testing.T) {
	entry := RedactedString("key", "abcd")
	if entry.Value() != REDACTION_PLACEHOLDER {
		t.Errorf("short values should be fully redacted, got %v", entry.Value())
	}
}

func TestRedactedString_partiallyReveals(t *testing.T) {
	entry := RedactedString("key", "abc123def456")
	val, ok := entry.Value().(string)
	if !ok {
		t.Fatal("expected string value")
	}
	if val == "abc123def456" {
		t.Error("value should be partially redacted")
	}
	if val == REDACTION_PLACEHOLDER {
		t.Error("value should not be fully redacted for long strings")
	}
}
