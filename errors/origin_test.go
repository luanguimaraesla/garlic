//go:build unit

package errors

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestOverride_keepsOriginPrivate(t *testing.T) {
	origin := New(KindSystemError, "connection to 10.0.0.5 refused")
	err := Override(KindNotFoundError, origin, "resource not found")

	if err.Error() != "resource not found" {
		t.Errorf("Error() = %q, want the visible message only", err.Error())
	}
	if strings.Contains(err.Error(), "10.0.0.5") {
		t.Error("the origin must not surface in Error()")
	}
	if !err.HasOrigin() {
		t.Error("HasOrigin() = false, want true")
	}
	if err.Origin() != origin {
		t.Error("Origin() should return the stashed origin error")
	}
	if err.Kind() != KindNotFoundError {
		t.Errorf("Kind() = %q, want KindNotFoundError", err.Kind().Name)
	}
}

func TestMirror_usesKindDescription(t *testing.T) {
	err := Mirror(KindNotFoundError)

	if err.Error() != KindNotFoundError.Description {
		t.Errorf("Error() = %q, want the kind description %q", err.Error(), KindNotFoundError.Description)
	}
	if err.HasOrigin() {
		t.Error("Mirror should not attach an origin")
	}
}

func TestMirrorOverride_descriptionAndOrigin(t *testing.T) {
	origin := New(KindSystemError, "secret backend detail")
	generic := KindForStatus(http.StatusServiceUnavailable)
	err := MirrorOverride(generic, origin)

	if err.Error() != generic.Description {
		t.Errorf("Error() = %q, want the generic description %q", err.Error(), generic.Description)
	}
	if err.Origin() != origin {
		t.Error("MirrorOverride should keep the origin reference")
	}
}

func TestErrorDTO_carriesOriginCodeOnly(t *testing.T) {
	origin := New(KindSystemError, "dial tcp 10.0.0.5:7233: connection refused")
	generic := KindForStatus(http.StatusInternalServerError)
	dto := MirrorOverride(generic, origin).ErrorDTO()

	if dto.Code != generic.Code {
		t.Errorf("Code = %q, want the generic status code %q", dto.Code, generic.Code)
	}
	if dto.Origin != KindSystemError.Code {
		t.Errorf("Origin = %q, want the origin kind code %q", dto.Origin, KindSystemError.Code)
	}
	if dto.Error != generic.Description {
		t.Errorf("Error = %q, want the generic description", dto.Error)
	}
	if strings.Contains(dto.Error, "10.0.0.5") {
		t.Error("the origin's sensitive message leaked into the wire body")
	}
}

func TestErrorDTO_noOrigin_omitsField(t *testing.T) {
	dto := New(KindNotFoundError, "user not found").ErrorDTO()
	if dto.Origin != "" {
		t.Errorf("Origin = %q, want empty when no origin is attached", dto.Origin)
	}

	raw, err := json.Marshal(dto)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var body map[string]any
	if e := json.Unmarshal(raw, &body); e != nil {
		t.Fatalf("unmarshal: %v", e)
	}
	if _, present := body["origin"]; present {
		t.Errorf("an empty origin must be omitted from the wire body, got %s", raw)
	}
}

func TestHeadlessError_isCodeOnly(t *testing.T) {
	h, ok := NewHeadlessError("S00503").(*HeadlessErrorT)
	if !ok {
		t.Fatalf("NewHeadlessError returned %T, want *HeadlessErrorT", h)
	}

	if h.Code() != "S00503" {
		t.Errorf("Code() = %q, want S00503", h.Code())
	}
	if h.Error() != "S00503" {
		t.Errorf("Error() = %q, want the code", h.Error())
	}
	if NewHeadlessError("") != nil {
		t.Error(`NewHeadlessError("") should return nil so an absent origin stays absent`)
	}
}

func TestCodeOf(t *testing.T) {
	if got := CodeOf(NewHeadlessError("S00503")); got != "S00503" {
		t.Errorf("CodeOf(headless) = %q, want S00503", got)
	}
	// *ErrorT satisfies KindCoder via Code(), so CodeOf returns its kind code.
	if got := CodeOf(New(KindSystemError, "x")); got != KindSystemError.Code {
		t.Errorf("CodeOf(*ErrorT) = %q, want %q", got, KindSystemError.Code)
	}
	if got := CodeOf(fmt.Errorf("plain")); got != "" {
		t.Errorf("CodeOf(plain error) = %q, want empty", got)
	}
}

// The origin survives a full encode/decode round-trip as a code-only reference:
// the sender's sensitive body never travels, but the receiver can still read the
// original kind code for troubleshooting.
func TestDTO_originRoundTrip(t *testing.T) {
	origin := New(KindSystemError, "sensitive backend detail")
	err := Override(KindNotFoundError, origin, "not found")

	raw := err.ErrorDTO().JSON()

	var wire DTO
	if e := json.Unmarshal(raw, &wire); e != nil {
		t.Fatalf("unmarshal: %v", e)
	}
	if strings.Contains(string(raw), "sensitive backend detail") {
		t.Error("the origin's message must not cross the wire")
	}

	decoded, ok := wire.Decode()
	if !ok {
		t.Fatal("Decode() ok = false, want the known code to decode")
	}
	if !decoded.HasOrigin() {
		t.Fatal("decoded error lost its origin reference")
	}
	if _, isHeadless := decoded.Origin().(*HeadlessErrorT); !isHeadless {
		t.Errorf("decoded origin = %T, want a code-only *HeadlessErrorT", decoded.Origin())
	}
	if got := CodeOf(decoded.Origin()); got != KindSystemError.Code {
		t.Errorf("origin code after round-trip = %q, want %q", got, KindSystemError.Code)
	}
}

func TestMustDecode_rebuildsOrigin(t *testing.T) {
	dto := &DTO{
		Error:  "not found",
		Code:   KindNotFoundError.Code,
		Origin: KindSystemError.Code,
	}

	decoded := dto.MustDecode()
	if got := CodeOf(decoded.Origin()); got != KindSystemError.Code {
		t.Errorf("origin code = %q, want %q", got, KindSystemError.Code)
	}
}

// Decoding a DTO with no origin must leave the origin genuinely absent. A typed
// nil stored in the error field would make HasOrigin lie and Origin panic when
// read.
func TestDecode_emptyOrigin_staysAbsent(t *testing.T) {
	for _, tc := range []struct {
		name   string
		decode func() *ErrorT
	}{
		{"Decode", func() *ErrorT { e, _ := (&DTO{Error: "boom", Code: KindNotFoundError.Code}).Decode(); return e }},
		{"MustDecode", func() *ErrorT { return (&DTO{Error: "boom", Code: KindNotFoundError.Code}).MustDecode() }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := tc.decode()
			if e.HasOrigin() {
				t.Error("HasOrigin() = true for a DTO with no origin, want false")
			}
			if e.Origin() != nil {
				t.Errorf("Origin() = %v, want nil", e.Origin())
			}
			if got := CodeOf(e.Origin()); got != "" {
				t.Errorf("CodeOf(Origin()) = %q, want empty", got)
			}
		})
	}
}

// The origin code must survive a second serialization. After decode the origin
// is a code-only HeadlessErrorT, and re-encoding it must carry the same code
// rather than degrading to the generic KindError code.
func TestErrorDTO_reencodePreservesOriginCode(t *testing.T) {
	origin := New(KindSystemError, "sensitive backend detail")
	first := Override(KindNotFoundError, origin, "not found").ErrorDTO()

	decoded, ok := first.Decode()
	if !ok {
		t.Fatal("decode failed")
	}

	second := decoded.ErrorDTO()
	if second.Origin != KindSystemError.Code {
		t.Errorf("re-encoded origin = %q, want %q preserved verbatim", second.Origin, KindSystemError.Code)
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
