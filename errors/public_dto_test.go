//go:build unit

package errors

import (
	"net/http"
	"strings"
	"testing"
)

func TestPublicDTO_userError_exposedInFull(t *testing.T) {
	err := New(KindInvalidRequestError, "email is invalid", Hint("use a valid email"))

	dto := err.PublicDTO()

	if dto.Error != "email is invalid" {
		t.Errorf("user error message should be exposed, got %q", dto.Error)
	}
	if dto.Code != KindInvalidRequestError.Code {
		t.Errorf("code = %q, want %q", dto.Code, KindInvalidRequestError.Code)
	}
	if dto.Details["hint"] != "use a valid email" {
		t.Error("user error details/hints should be exposed")
	}
}

func TestPublicDTO_systemError_sanitized(t *testing.T) {
	err := New(KindSystemError, "connection to 10.0.0.5 failed",
		Hint("internal detail"), Context(Field("secret", "value")))

	dto := err.PublicDTO()

	status := KindSystemError.StatusCode()
	if dto.Code != KindSystemError.Code {
		t.Errorf("code = %q, want the kind code %q", dto.Code, KindSystemError.Code)
	}
	if dto.Error != http.StatusText(status) {
		t.Errorf("error = %q, want the standard status text %q", dto.Error, http.StatusText(status))
	}
	if dto.Name != "" {
		t.Errorf("system error name must not cross the wire, got %q", dto.Name)
	}
	if strings.Contains(dto.Error, "10.0.0.5") {
		t.Error("the dynamic system message must not leak")
	}
	if len(dto.Details) != 0 {
		t.Errorf("system error details must be stripped, got %v", dto.Details)
	}
}

func TestPublicDTO_rootError_protected(t *testing.T) {
	dto := New(KindError, "raw internals").PublicDTO()
	if dto.Code != KindError.Code {
		t.Errorf("root error code = %q, want %q", dto.Code, KindError.Code)
	}
	if dto.Error != http.StatusText(http.StatusInternalServerError) {
		t.Errorf("root error should be protected like a 500 system error, got %q", dto.Error)
	}
	if dto.Name != "" {
		t.Errorf("root error name must not cross the wire, got %q", dto.Name)
	}
	if dto.Error == "raw internals" {
		t.Error("root error dynamic message leaked")
	}
}

// A domain-specific system kind (as a downstream package would register) sends
// its own code so a client can quote it to support, but nothing else: the name,
// the static description, and the dynamic message all stay server-side.
func TestPublicDTO_tertiarySystemKind_codeOnly(t *testing.T) {
	kind := &Kind{
		Name:        "TemporalUnavailable",
		Code:        "K000569",
		Description: "the temporal cluster is unreachable",
		Parent:      KindForStatus(http.StatusServiceUnavailable),
	}

	dto := New(kind, "dial tcp 10.0.0.5:7233: connection refused").PublicDTO()

	if dto.Code != kind.Code {
		t.Errorf("code = %q, want the specific kind code %q as a support reference", dto.Code, kind.Code)
	}
	if dto.Name != "" {
		t.Errorf("leaf kind name leaked: %q", dto.Name)
	}
	if dto.Error != http.StatusText(http.StatusServiceUnavailable) {
		t.Errorf("error = %q, want standard status text %q",
			dto.Error, http.StatusText(http.StatusServiceUnavailable))
	}
	if strings.Contains(dto.Error, "temporal") || strings.Contains(dto.Error, "10.0.0.5") {
		t.Errorf("leaf description or dynamic message leaked: %q", dto.Error)
	}
}

func TestUserErrorHierarchy_isUserClass(t *testing.T) {
	if !KindInvalidRequestError.Is(KindUserError) {
		t.Error("InvalidRequest should descend from UserError")
	}
	if !KindNotFoundError.Is(KindUserError) {
		t.Error("NotFound should descend from UserError")
	}
}

func TestErrorT_Description(t *testing.T) {
	err := New(KindNotFoundError, "user 7 not found")
	if err.Description() != KindNotFoundError.Description {
		t.Errorf("Description() = %q, want %q", err.Description(), KindNotFoundError.Description)
	}
}
