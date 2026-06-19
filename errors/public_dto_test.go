//go:build unit

package errors

import "testing"

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

	if dto.Error != KindSystemError.Description {
		t.Errorf("system error should expose the kind Description, got %q", dto.Error)
	}
	if dto.Error == "connection to 10.0.0.5 failed" {
		t.Error("the dynamic system message must not leak")
	}
	if dto.Code != KindSystemError.Code {
		t.Errorf("code should be preserved, got %q", dto.Code)
	}
	if len(dto.Details) != 0 {
		t.Errorf("system error details must be stripped, got %v", dto.Details)
	}
}

func TestPublicDTO_rootError_protected(t *testing.T) {
	dto := New(KindError, "raw internals").PublicDTO()
	if dto.Error != KindError.Description {
		t.Errorf("root error should be protected like a system error, got %q", dto.Error)
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
