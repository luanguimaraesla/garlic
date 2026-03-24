//go:build unit
// +build unit

package errors

import (
	"fmt"
	"net/http"
	"testing"
)

// --- wrap() behavior ---

func TestWrap_copiesDetailsFromWrappedError(t *testing.T) {
	inner := New(KindNotFoundError, "not found",
		Hint("check the ID"),
	)

	outer := Propagate(inner, "service failed")
	if outer.Details["hint"] != "check the ID" {
		t.Errorf("expected hint to be copied from wrapped error, got %v", outer.Details["hint"])
	}
}

func TestWrap_nilError_doesNotPanic(t *testing.T) {
	e := From(KindError, nil, "no cause")
	if e.cause != nil {
		t.Error("expected nil cause")
	}
}

func TestWrap_stdlibError_doesNotCopyDetails(t *testing.T) {
	stdErr := fmt.Errorf("stdlib error")
	e := Propagate(stdErr, "wrapped")
	if len(e.Details) != 0 {
		t.Errorf("expected empty Details when wrapping stdlib error, got %v", e.Details)
	}
}

// --- Propagate kind inheritance ---

func TestPropagate_inheritsKindFromErrorT(t *testing.T) {
	inner := New(KindNotFoundError, "not found")
	outer := Propagate(inner, "service layer")

	if !outer.Kind().Is(KindNotFoundError) {
		t.Errorf("expected NotFoundError kind, got %s", outer.Kind().Name)
	}
}

func TestPropagate_defaultsToKindErrorForStdlib(t *testing.T) {
	outer := Propagate(fmt.Errorf("raw"), "service layer")

	if outer.Kind() != KindError {
		t.Errorf("expected KindError for stdlib error, got %s", outer.Kind().Name)
	}
}

func TestPropagateAs_overridesKind(t *testing.T) {
	inner := New(KindSystemError, "db down")
	outer := PropagateAs(KindNotFoundError, inner, "not found")

	if !outer.Kind().Is(KindNotFoundError) {
		t.Errorf("expected NotFoundError, got %s", outer.Kind().Name)
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
	err := Propagate(cause, "wrapper")

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
	err := New(KindValidationError, "bad input",
		Hint("check field %s", "email"),
	)

	if err.Details["hint"] != "check field email" {
		t.Errorf("want 'check field email', got %v", err.Details["hint"])
	}
}

func TestHint_lastOneWins(t *testing.T) {
	err := New(KindValidationError, "bad input",
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

	err := tmpl.Propagate(cause)
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
	if KindValidationError.StatusCode() != http.StatusBadRequest {
		t.Errorf("ValidationError: want 400, got %d", KindValidationError.StatusCode())
	}
	if KindDatabaseRecordNotFoundError.StatusCode() != http.StatusNotFound {
		t.Errorf("DatabaseRecordNotFoundError: want 404, got %d", KindDatabaseRecordNotFoundError.StatusCode())
	}
}

func TestKind_FQN_buildsFullHierarchy(t *testing.T) {
	fqn := KindValidationError.FQN()
	want := "ValidationError::InvalidRequestError::UserError::Error"
	if fqn != want {
		t.Errorf("FQN: want %s, got %s", want, fqn)
	}
}

func TestKind_Is_matchesSelfAndParents(t *testing.T) {
	if !KindValidationError.Is(KindValidationError) {
		t.Error("should match self")
	}
	if !KindValidationError.Is(KindInvalidRequestError) {
		t.Error("should match parent")
	}
	if !KindValidationError.Is(KindUserError) {
		t.Error("should match grandparent")
	}
	if KindValidationError.Is(KindSystemError) {
		t.Error("should not match unrelated kind")
	}
}

// --- DTO ---

func TestErrorDTO_containsKindAndDetails(t *testing.T) {
	err := New(KindValidationError, "email invalid",
		Hint("provide a valid email"),
	)

	dto := err.ErrorDTO()
	if dto.Name != KindValidationError.FQN() {
		t.Errorf("name: want %s, got %s", KindValidationError.FQN(), dto.Name)
	}
	if dto.Code != KindValidationError.Code {
		t.Errorf("code: want %s, got %s", KindValidationError.Code, dto.Code)
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
