//go:build unit

package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/luanguimaraesla/garlic/errors"
	"github.com/luanguimaraesla/garlic/rest"
)

// asErrorT returns the outermost *errors.ErrorT in the chain.
func asErrorT(t *testing.T, err error) *errors.ErrorT {
	t.Helper()

	var e *errors.ErrorT
	if !errors.As(err, &e) {
		t.Fatalf("error is not an *errors.ErrorT: %v", err)
	}

	return e
}

// troubleshooting flattens the per-caller troubleshooting context into a single
// map, which is all a test needs to assert on.
func troubleshooting(t *testing.T, err error) map[string]any {
	t.Helper()

	flat := map[string]any{}
	for _, entries := range asErrorT(t, err).Troubleshooting.Context {
		fields, ok := entries.(map[string]any)
		if !ok {
			t.Fatalf("troubleshooting context entry is %T, want map[string]any", entries)
		}
		for key, value := range fields {
			flat[key] = value
		}
	}

	return flat
}

// assertNoLeak proves the peer's body never reaches the message or the
// wire-visible details, and that no encoding/json wording leaks with it.
func assertNoLeak(t *testing.T, err error, secret string) {
	t.Helper()

	message := err.Error()
	if strings.Contains(message, secret) {
		t.Errorf("message leaks the response body: %q", message)
	}
	for _, parser := range []string{"invalid character", "cannot unmarshal", "unexpected end of JSON"} {
		if strings.Contains(message, parser) {
			t.Errorf("message leaks parser wording %q: %q", parser, message)
		}
	}

	for key, value := range asErrorT(t, err).Details {
		if text, ok := value.(string); ok && strings.Contains(text, secret) {
			t.Errorf("details[%q] leaks the response body: %q", key, text)
		}
	}
}

func TestResponse_DecodeErrorReturnsGarlicError(t *testing.T) {
	raw, _ := json.Marshal(errors.New(errors.KindNotFoundError, "missing").ErrorDTO())
	c := newClientWithTransport(t, RoundTripperFunc(func(*http.Request) (*http.Response, error) {
		return textResponse(http.StatusNotFound, string(raw), nil), nil
	}), &Config{Retry: RetryConfig{}})

	resp, err := c.R(context.Background()).Get("/x")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	gerr := resp.DecodeError()
	if !errors.IsKind(gerr, errors.KindNotFoundError) {
		t.Fatalf("decoded error kind = %v, want KindNotFoundError", gerr)
	}
	if gerr.Error() != "missing" {
		t.Errorf("message = %q, want missing", gerr.Error())
	}
}

func TestResponse_DecodeErrorRejectsUnknownKind(t *testing.T) {
	c := newClientWithTransport(t, RoundTripperFunc(func(*http.Request) (*http.Response, error) {
		return textResponse(http.StatusBadGateway, `{"error":"weird","kind":"ZZZ999"}`, nil), nil
	}), &Config{Retry: RetryConfig{}})

	resp, err := c.R(context.Background()).Get("/x")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	gerr := resp.DecodeError()
	if !errors.IsKind(gerr, KindUnknownResponseError) {
		t.Fatalf("decoded error kind = %v, want KindUnknownResponseError", gerr)
	}
}

func TestResponse_DecodeErrorRejectsNonErrorResponse(t *testing.T) {
	resp := &Response{Response: textResponse(http.StatusOK, `{}`, nil)}

	gerr := resp.DecodeError()
	if !errors.IsKind(gerr, KindResponseDecodeError) {
		t.Fatalf("decoded error kind = %v, want KindResponseDecodeError", gerr)
	}
}

func TestRequest_SendDoesNotErrorOnHTTPStatus(t *testing.T) {
	raw, _ := json.Marshal(errors.New(errors.KindNotFoundError, "missing").ErrorDTO())
	c := newClientWithTransport(t, RoundTripperFunc(func(*http.Request) (*http.Response, error) {
		return textResponse(http.StatusNotFound, string(raw), nil), nil
	}), &Config{Retry: RetryConfig{}})

	resp, err := c.R(context.Background()).Get("/x")
	if err != nil {
		t.Fatalf("Get returned error for HTTP status: %v", err)
	}
	if !resp.IsError() {
		t.Fatal("expected response to report an error status")
	}
}

func TestResponse_DecodeErrorPreservesRegisteredDTO(t *testing.T) {
	raw, err := json.Marshal(errors.DTO{
		Name:    errors.KindNotFoundError.FQN(),
		Error:   "user 42 does not exist",
		Code:    errors.KindNotFoundError.Code,
		Origin:  errors.KindSystemError.Code,
		Details: map[string]any{"hint": "check the identifier"},
	})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	gerr := errorResponse(http.StatusNotFound, io.NopCloser(strings.NewReader(string(raw))), "application/json").DecodeError()

	if !errors.IsKind(gerr, errors.KindNotFoundError) {
		t.Fatalf("kind = %v, want KindNotFoundError", gerr)
	}
	if !errors.IsKind(gerr, errors.KindUserError) {
		t.Error("a 404 DTO should still match the user class")
	}
	if gerr.Error() != "user 42 does not exist" {
		t.Errorf("message = %q", gerr.Error())
	}
	if got := asErrorT(t, gerr).Details["hint"]; got != "check the identifier" {
		t.Errorf("details hint = %v", got)
	}
	if code, ok := errors.OriginCodeOf(gerr); !ok || code != errors.KindSystemError.Code {
		t.Errorf("origin = %q, %v, want %q", code, ok, errors.KindSystemError.Code)
	}
	if got := asErrorT(t, gerr).StatusCode(); got != http.StatusNotFound {
		t.Errorf("StatusCode() = %d, want 404", got)
	}
}

func TestResponse_DecodeErrorToleratesAdditiveFields(t *testing.T) {
	body := fmt.Sprintf(
		`{"error":"missing","kind":%q,"severity":"warning","retry":{"after":3}}`,
		errors.KindNotFoundError.Code,
	)

	gerr := errorResponse(http.StatusNotFound, io.NopCloser(strings.NewReader(body)), "application/json").DecodeError()

	if !errors.IsKind(gerr, errors.KindNotFoundError) {
		t.Fatalf("kind = %v, want KindNotFoundError", gerr)
	}
	if gerr.Error() != "missing" {
		t.Errorf("message = %q, want missing", gerr.Error())
	}
}

func TestResponse_DecodeErrorKeepsOversizedRegisteredDTO(t *testing.T) {
	message := strings.Repeat("m", 3*maxDiagnosticBytes)
	body := fmt.Sprintf(
		`{"error":%q,"kind":%q,"details":{"blob":%q},"future":%q}`,
		message, errors.KindInvalidRequestError.Code, message, message,
	)

	gerr := errorResponse(http.StatusBadRequest, io.NopCloser(strings.NewReader(body)), "application/json").DecodeError()

	if !errors.IsKind(gerr, errors.KindInvalidRequestError) {
		t.Fatalf("kind = %v, want KindInvalidRequestError", gerr)
	}
	if gerr.Error() != message {
		t.Errorf("an oversized DTO message should survive intact, got %d bytes", len(gerr.Error()))
	}
	if got := asErrorT(t, gerr).Details["blob"]; got != message {
		t.Error("an oversized DTO detail should survive intact")
	}
}

func TestResponse_DecodeErrorReportsUnknownKind(t *testing.T) {
	cases := []struct {
		status int
		class  *errors.Kind
	}{
		{http.StatusBadGateway, errors.KindForStatus(http.StatusBadGateway)},
		{499, errors.KindUserError},
	}

	for _, tc := range cases {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			body := `{"error":"backend exploded","kind":"Z99999","name":"FutureError"}`
			gerr := errorResponse(tc.status, io.NopCloser(strings.NewReader(body)), "application/json").DecodeError()

			if !errors.IsKind(gerr, KindUnknownResponseError) {
				t.Fatalf("kind = %v, want KindUnknownResponseError", gerr)
			}
			if !errors.IsKind(gerr, tc.class) {
				t.Errorf("the cause chain should also match %s", tc.class.Name)
			}
			if !strings.Contains(gerr.Error(), "Z99999") {
				t.Errorf("message should name the received kind: %q", gerr.Error())
			}
			if got := asErrorT(t, gerr).StatusCode(); got != tc.status {
				t.Errorf("StatusCode() = %d, want %d", got, tc.status)
			}

			context := troubleshooting(t, gerr)
			if context["received_kind"] != "Z99999" {
				t.Errorf("received_kind = %v", context["received_kind"])
			}
			if context["received_name"] != "FutureError" {
				t.Errorf("received_name = %v", context["received_name"])
			}
			if context["received_message"] != "backend exploded" {
				t.Errorf("received_message = %v", context["received_message"])
			}
			if context["http_status"] != tc.status {
				t.Errorf("http_status = %v", context["http_status"])
			}
		})
	}
}

func TestResponse_DecodeErrorBoundsUnknownKindDiagnostics(t *testing.T) {
	oversized := strings.Repeat("K", 3*maxDiagnosticBytes)
	body := fmt.Sprintf(`{"error":%q,"kind":%q,"name":%q}`, oversized, oversized, oversized)

	gerr := errorResponse(http.StatusBadGateway, io.NopCloser(strings.NewReader(body)), "application/json").DecodeError()

	if !errors.IsKind(gerr, KindUnknownResponseError) {
		t.Fatalf("an oversized unknown DTO should still classify as a mismatch: %v", gerr)
	}

	context := troubleshooting(t, gerr)
	for _, key := range []string{"received_kind", "received_name", "received_message"} {
		text, ok := context[key].(string)
		if !ok {
			t.Fatalf("%s = %v, want a string", key, context[key])
		}
		if len(text) != maxDiagnosticBytes {
			t.Errorf("%s is %d bytes, want the %d-byte bound", key, len(text), maxDiagnosticBytes)
		}
		if context[key+"_bytes"] != len(oversized) {
			t.Errorf("%s_bytes = %v, want %d", key, context[key+"_bytes"], len(oversized))
		}
		if context[key+"_truncated"] != true {
			t.Errorf("%s_truncated = %v, want true", key, context[key+"_truncated"])
		}
	}

	if rendered := renderedKind(t, gerr); len(rendered) > maxDiagnosticBytes {
		t.Errorf("the rendered kind is %d bytes, want at most %d", len(rendered), maxDiagnosticBytes)
	}
}

// renderedKind isolates the peer-derived part of a kind mismatch message from
// the fixed wording around it.
func renderedKind(t *testing.T, err error) string {
	t.Helper()

	const (
		prefix = "upstream returned error kind "
		suffix = ", which is not registered here"
	)

	message := err.Error()
	rendered, _, found := strings.Cut(strings.TrimPrefix(message, prefix), suffix)
	if !strings.HasPrefix(message, prefix) || !found {
		t.Fatalf("message does not read like a kind mismatch: %q", message)
	}

	return rendered
}

// Quoting expands what it escapes, so an identifier that is within the bound as
// received can still render several times over it.
func TestResponse_DecodeErrorBoundsTheRenderedUnknownKind(t *testing.T) {
	cases := map[string]string{
		"control characters":     strings.Repeat("\x00", 3*maxDiagnosticBytes),
		"quotes and backslashes": strings.Repeat(`"\`, maxDiagnosticBytes),
		"multibyte text":         strings.Repeat("€", maxDiagnosticBytes),
		"ordinary code":          "Z99999",
	}

	for name, code := range cases {
		t.Run(name, func(t *testing.T) {
			body, err := json.Marshal(map[string]string{"error": "boom", "kind": code})
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}

			gerr := errorResponse(http.StatusBadGateway, io.NopCloser(bytes.NewReader(body)), "application/json").DecodeError()

			if !errors.IsKind(gerr, KindUnknownResponseError) {
				t.Fatalf("kind = %v, want KindUnknownResponseError", gerr)
			}
			if !errors.IsKind(gerr, errors.KindForStatus(http.StatusBadGateway)) {
				t.Error("the status class should stay reachable through the cause")
			}

			rendered := renderedKind(t, gerr)
			if len(rendered) > maxDiagnosticBytes {
				t.Errorf("the rendered kind is %d bytes, want at most %d", len(rendered), maxDiagnosticBytes)
			}
			if !utf8.ValidString(rendered) {
				t.Error("the rendered kind should stay valid UTF-8")
			}
			if strings.ContainsRune(rendered, 0) {
				t.Error("quoting should keep control characters out of the message")
			}
		})
	}

	if got := renderedKind(t, unknownKindError(t, "Z99999")); got != `"Z99999"` {
		t.Errorf("an ordinary code renders as %s, want it quoted and intact", got)
	}
}

func unknownKindError(t *testing.T, code string) error {
	t.Helper()

	body, err := json.Marshal(map[string]string{"error": "boom", "kind": code})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	return errorResponse(http.StatusBadGateway, io.NopCloser(bytes.NewReader(body)), "application/json").DecodeError()
}

// encoding/json rewrites invalid UTF-8 into U+FFFD while decoding, so what the
// peer sent can only be judged on the raw literal. An identifier that arrived as
// binary keeps its length and nothing else.
func TestResponse_DecodeErrorDropsABinaryUnknownKind(t *testing.T) {
	body := []byte("{\"error\":\"boom\",\"kind\":\"Z\xff\xfe\"}")

	gerr := errorResponse(http.StatusBadGateway, io.NopCloser(bytes.NewReader(body)), "application/json").DecodeError()

	if !errors.IsKind(gerr, KindUnknownResponseError) {
		t.Fatalf("kind = %v, want KindUnknownResponseError", gerr)
	}

	context := troubleshooting(t, gerr)
	if got, ok := context["received_kind"]; ok {
		t.Errorf("a binary kind should not be retained, got %v", got)
	}
	if context["received_kind_bytes"] != 3 {
		t.Errorf("received_kind_bytes = %v, want the 3 bytes the peer sent", context["received_kind_bytes"])
	}
	if context["received_kind_truncated"] != false {
		t.Errorf("received_kind_truncated = %v, want false", context["received_kind_truncated"])
	}

	if got := renderedKind(t, gerr); got != "<3 unreadable bytes>" {
		t.Errorf("the rendered kind is %q, want the length placeholder", got)
	}
	if strings.ContainsRune(gerr.Error(), utf8.RuneError) {
		t.Error("the message should not carry the replacement characters json produced")
	}
}

// A duplicate member decides what the peer finally sent, and encoding/json
// leaves a decoded field on the earlier occurrence when the later one is null.
// The diagnostic follows the raw member instead, so an earlier value cannot
// reach the logs on the validity of a later one.
func TestResponse_DecodeErrorJudgesTheRawNameOccurrence(t *testing.T) {
	cases := map[string]struct {
		body string
		// wantBytes is nil when the peer named no identifier to report.
		wantBytes any
	}{
		"binary name nulled by a duplicate": {
			body: "{\"error\":\"boom\",\"kind\":\"Z99999\",\"name\":\"s\xff\",\"name\":null}",
		},
		"binary name nulled by a duplicate in another case": {
			body: "{\"error\":\"boom\",\"kind\":\"Z99999\",\"name\":\"s\xff\",\"NAME\":null}",
		},
		"binary name after a nulled duplicate": {
			body:      "{\"error\":\"boom\",\"kind\":\"Z99999\",\"name\":null,\"name\":\"s\xff\"}",
			wantBytes: 2,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			gerr := errorResponse(http.StatusBadGateway, io.NopCloser(strings.NewReader(tc.body)), "application/json").DecodeError()

			if !errors.IsKind(gerr, KindUnknownResponseError) {
				t.Fatalf("kind = %v, want KindUnknownResponseError", gerr)
			}

			context := troubleshooting(t, gerr)
			if got, ok := context["received_name"]; ok {
				t.Errorf("received_name = %q, want a raw-invalid name reduced to its length", got)
			}
			if got := context["received_name_bytes"]; got != tc.wantBytes {
				t.Errorf("received_name_bytes = %v, want %v", got, tc.wantBytes)
			}
			if context["received_kind"] != "Z99999" {
				t.Errorf("received_kind = %v, want the kind still reported", context["received_kind"])
			}
			if strings.ContainsRune(fmt.Sprint(context), utf8.RuneError) {
				t.Errorf("a diagnostic carries the replacements json produced: %v", context)
			}
		})
	}
}

// A valid identifier is kept as encoding/json decoded it, which is the text the
// peer meant, with the lengths of that decoded text.
func TestResponse_DecodeErrorKeepsAMultibyteUnknownKind(t *testing.T) {
	const code = "Zé99€"

	context := troubleshooting(t, unknownKindError(t, code))

	if context["received_kind"] != code {
		t.Errorf("received_kind = %v, want %q", context["received_kind"], code)
	}
	if context["received_kind_bytes"] != len(code) {
		t.Errorf("received_kind_bytes = %v, want %d", context["received_kind_bytes"], len(code))
	}
}

func TestCaptureDiagnostic_dropsUnreadableValues(t *testing.T) {
	binary := []byte{0x00, 0xff, 0xfe, 0x80, 0x81}

	c := captureDiagnostic(binary)
	if c.readable || c.text != "" {
		t.Errorf("a binary value should not be retained, got %q", c.text)
	}
	if c.length != len(binary) {
		t.Errorf("length = %d, want %d", c.length, len(binary))
	}
	if c.truncated {
		t.Error("a short value should not be marked truncated")
	}
}

func TestCaptureDiagnostic_cutsOnARuneBoundary(t *testing.T) {
	cases := map[string]struct {
		value []byte
		want  int
	}{
		"three-byte rune": {
			value: append(bytes.Repeat([]byte("a"), maxDiagnosticBytes-1), []byte("€€")...),
			want:  maxDiagnosticBytes - 1,
		},
		"four-byte rune": {
			value: append(bytes.Repeat([]byte("a"), maxDiagnosticBytes-2), []byte("😀😀")...),
			want:  maxDiagnosticBytes - 2,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := captureDiagnostic(tc.value)
			if !c.readable {
				t.Fatal("a half-cut rune should be left out, not turn the value unreadable")
			}
			if !c.truncated {
				t.Error("truncated should be set")
			}
			if c.length != len(tc.value) {
				t.Errorf("length = %d, want the original %d", c.length, len(tc.value))
			}
			if len(c.text) != tc.want {
				t.Errorf("text is %d bytes, want %d with the partial rune left out", len(c.text), tc.want)
			}
		})
	}
}

// Trimming back to something that decodes would turn a binary prefix into a
// readable-looking snippet, which is exactly what the retention policy keeps out
// of the logs. Only a rune the bound itself cut in half may be dropped.
func TestCaptureDiagnostic_dropsInvalidBytesAroundTheBound(t *testing.T) {
	fill := func(n int) []byte { return bytes.Repeat([]byte("a"), n) }

	cases := map[string][]byte{
		"invalid byte at the bound":      append(fill(maxDiagnosticBytes-1), 0xff, 'x'),
		"lone continuation byte":         append(fill(maxDiagnosticBytes-1), 0x80, 'x'),
		"invalid byte before the bound":  append(append(fill(maxDiagnosticBytes-8), 0xff), fill(16)...),
		"broken multibyte sequence":      append(fill(maxDiagnosticBytes-2), 0xe2, 0x28, 0xa1),
		"surrogate half at the bound":    append(fill(maxDiagnosticBytes-2), 0xed, 0xa0, 0x80),
		"overlong encoding at the bound": append(fill(maxDiagnosticBytes-2), 0xc0, 0x80, 'x'),
	}

	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			c := captureDiagnostic(value)

			if c.readable || c.text != "" {
				t.Errorf("an invalid value should keep no text, got %d bytes", len(c.text))
			}
			if c.length != len(value) {
				t.Errorf("length = %d, want %d", c.length, len(value))
			}
			if !c.truncated {
				t.Error("a value past the bound should still be marked truncated")
			}
		})
	}
}

// A byte the bound already excluded is never retained, so it has no say in
// whether the prefix before it is.
func TestCaptureDiagnostic_keepsAValidPrefixPastTheBound(t *testing.T) {
	for name, value := range map[string][]byte{
		"invalid byte at the bound":   invalidAt(maxDiagnosticBytes),
		"invalid byte past the bound": invalidAt(maxDiagnosticBytes + 1),
	} {
		t.Run(name, func(t *testing.T) {
			c := captureDiagnostic(value)

			if !c.readable {
				t.Fatal("the bounded prefix is valid, it should be retained")
			}
			if len(c.text) != maxDiagnosticBytes {
				t.Errorf("text is %d bytes, want the %d-byte bound", len(c.text), maxDiagnosticBytes)
			}
			if c.length != len(value) {
				t.Errorf("length = %d, want the original %d", c.length, len(value))
			}
			if !c.truncated {
				t.Error("truncated should be set")
			}
		})
	}
}

// invalidAt builds a value whose first invalid byte sits at offset, followed by
// one more byte so the value always runs past the retention bound.
func invalidAt(offset int) []byte {
	return append(bytes.Repeat([]byte("a"), offset), 0xff, 'x')
}

// The retention budget decides how much of a body is kept, not whether it is
// kept at all. Only an invalid byte the budget would have retained drops the
// snippet.
func TestResponse_DecodeErrorJudgesInvalidBytesWithinTheBudget(t *testing.T) {
	cases := map[string]struct {
		offset int
		// wantText is -1 when the value should keep nothing but its length.
		wantText int
	}{
		"inside the budget": {offset: maxDiagnosticBytes - 1, wantText: -1},
		"at the budget":     {offset: maxDiagnosticBytes, wantText: maxDiagnosticBytes},
		"past the budget":   {offset: maxDiagnosticBytes + 1, wantText: maxDiagnosticBytes},
	}

	for name, tc := range cases {
		value := invalidAt(tc.offset)

		bodies := map[string]io.ReadCloser{
			"complete body":      io.NopCloser(bytes.NewReader(value)),
			"failed read prefix": &failingBody{prefix: value},
		}

		for read, body := range bodies {
			t.Run(name+", "+read, func(t *testing.T) {
				gerr := errorResponse(http.StatusBadGateway, body, "application/octet-stream").DecodeError()

				context := troubleshooting(t, gerr)
				snippet, kept := context["body"].(string)

				switch {
				case tc.wantText < 0 && kept:
					t.Errorf("body kept %d bytes, want an invalid prefix reduced to its length", len(snippet))
				case tc.wantText >= 0 && !kept:
					t.Error("the valid bounded prefix should be retained")
				case tc.wantText >= 0 && len(snippet) != tc.wantText:
					t.Errorf("body is %d bytes, want %d", len(snippet), tc.wantText)
				}

				if context["body_bytes"] != len(value) {
					t.Errorf("body_bytes = %v, want the %d bytes received", context["body_bytes"], len(value))
				}
				if context["body_truncated"] != true {
					t.Errorf("body_truncated = %v, want true", context["body_truncated"])
				}
			})
		}
	}
}

func TestResponse_DecodeErrorClassifiesEnvelopes(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		garlic  bool
		message string
	}{
		{"foreign kind discriminator", `{"kind":"P00002","message":"backend exploded"}`, false, ""},
		{"error without kind", `{"error":"boom"}`, false, ""},
		{"empty kind", `{"error":"boom","kind":""}`, false, ""},
		{"numeric kind", `{"error":"boom","kind":7}`, false, ""},
		{"json null", `null`, false, ""},
		{"json array", `[{"error":"boom","kind":"Z1"}]`, false, ""},
		{"mistyped details", `{"error":"boom","kind":"Z1","details":"nope"}`, false, ""},
		{"unregistered kind", `{"error":"boom","kind":"Z1"}`, true, "Z1"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gerr := errorResponse(http.StatusBadGateway, io.NopCloser(strings.NewReader(tc.body)), "application/json").DecodeError()

			if got := errors.IsKind(gerr, KindUnknownResponseError); got != tc.garlic {
				t.Fatalf("KindUnknownResponseError = %v, want %v (%v)", got, tc.garlic, gerr)
			}
			if !errors.IsKind(gerr, errors.KindForStatus(http.StatusBadGateway)) {
				t.Error("every classification should keep the status kind reachable")
			}
			if tc.message != "" && !strings.Contains(gerr.Error(), tc.message) {
				t.Errorf("message = %q, want it to name %q", gerr.Error(), tc.message)
			}
			if !tc.garlic {
				if got := troubleshooting(t, gerr)["body"]; got != tc.body {
					t.Errorf("body snippet = %v, want the received body", got)
				}
			}
		})
	}
}

func TestResponse_DecodeErrorPreservesNonGarlicDiagnostics(t *testing.T) {
	const page = "<html><body>502 Bad Gateway</body></html>"

	gerr := errorResponse(http.StatusBadGateway, io.NopCloser(strings.NewReader(page)), "text/html").DecodeError()

	if !errors.IsKind(gerr, errors.KindForStatus(http.StatusBadGateway)) {
		t.Fatalf("kind = %v, want the 502 kind", gerr)
	}
	if errors.IsKind(gerr, KindUnknownResponseError) {
		t.Error("a non-garlic body must stay distinguishable from a kind mismatch")
	}
	if got := asErrorT(t, gerr).StatusCode(); got != http.StatusBadGateway {
		t.Errorf("StatusCode() = %d, want 502", got)
	}

	details := asErrorT(t, gerr).Details
	if details["http_status"] != http.StatusBadGateway {
		t.Errorf("http_status = %v", details["http_status"])
	}
	if details["content_type"] != "text/html" {
		t.Errorf("content_type = %v", details["content_type"])
	}
	if details["body_bytes"] != len(page) {
		t.Errorf("body_bytes = %v, want %d", details["body_bytes"], len(page))
	}
	if details["truncated"] != false {
		t.Errorf("truncated = %v, want false", details["truncated"])
	}

	if got := troubleshooting(t, gerr)["body"]; got != page {
		t.Errorf("body snippet = %v, want the received page", got)
	}
	assertNoLeak(t, gerr, "502 Bad Gateway</body>")
}

func TestResponse_DecodeErrorKeepsMalformedJSONDiagnostics(t *testing.T) {
	const body = `{"error": "truncated`

	gerr := errorResponse(http.StatusInternalServerError, io.NopCloser(strings.NewReader(body)), "application/json").DecodeError()

	if !errors.IsKind(gerr, errors.KindForStatus(http.StatusInternalServerError)) {
		t.Fatalf("kind = %v, want the 500 kind", gerr)
	}
	if errors.IsKind(gerr, KindResponseDecodeError) {
		t.Error("a malformed peer body is no longer a local decode failure")
	}
	if got := troubleshooting(t, gerr)["body"]; got != body {
		t.Errorf("body snippet = %v, want the received body", got)
	}
	assertNoLeak(t, gerr, "truncated")
}

func TestResponse_DecodeErrorBoundsOversizedBodies(t *testing.T) {
	body := strings.Repeat("x", 5*maxDiagnosticBytes)

	gerr := errorResponse(http.StatusServiceUnavailable, io.NopCloser(strings.NewReader(body)), "text/plain").DecodeError()

	details := asErrorT(t, gerr).Details
	if details["body_bytes"] != len(body) {
		t.Errorf("body_bytes = %v, want the full %d", details["body_bytes"], len(body))
	}
	if details["truncated"] != true {
		t.Errorf("truncated = %v, want true", details["truncated"])
	}

	snippet, ok := troubleshooting(t, gerr)["body"].(string)
	if !ok {
		t.Fatal("the bounded snippet should be a string")
	}
	if len(snippet) != maxDiagnosticBytes {
		t.Errorf("snippet is %d bytes, want the %d-byte bound", len(snippet), maxDiagnosticBytes)
	}
}

func TestResponse_DecodeErrorRecordsBinaryBodiesByLengthOnly(t *testing.T) {
	body := []byte{0x89, 0x50, 0x4e, 0x47, 0x00, 0xff, 0xfe}

	gerr := errorResponse(http.StatusInternalServerError, io.NopCloser(bytes.NewReader(body)), "image/png").DecodeError()

	context := troubleshooting(t, gerr)
	if _, ok := context["body"]; ok {
		t.Errorf("a binary body should not be retained, got %v", context["body"])
	}
	if context["body_bytes"] != len(body) {
		t.Errorf("body_bytes = %v, want %d", context["body_bytes"], len(body))
	}
	if asErrorT(t, gerr).Details["content_type"] != "image/png" {
		t.Error("the content type should still be recorded")
	}
}

func TestResponse_DecodeErrorRefusesBinaryBodiesAtTheBound(t *testing.T) {
	body := append(bytes.Repeat([]byte("a"), maxDiagnosticBytes-1), 0xff, 'x')

	gerr := errorResponse(http.StatusInternalServerError, io.NopCloser(bytes.NewReader(body)), "application/octet-stream").DecodeError()

	context := troubleshooting(t, gerr)
	if got, ok := context["body"]; ok {
		t.Errorf("a body that is binary within the bound should not be retained, got %d bytes", len(got.(string)))
	}
	if context["body_bytes"] != len(body) {
		t.Errorf("body_bytes = %v, want %d", context["body_bytes"], len(body))
	}
	if context["body_truncated"] != true {
		t.Errorf("body_truncated = %v, want true", context["body_truncated"])
	}
	if got := asErrorT(t, gerr).Details["truncated"]; got != true {
		t.Errorf("details truncated = %v, want true", got)
	}
}

func TestResponse_DecodeErrorRefusesAnUnreadableFailedReadPrefix(t *testing.T) {
	body := &failingBody{prefix: append(bytes.Repeat([]byte("a"), maxDiagnosticBytes-1), 0xff, 'x')}

	gerr := errorResponse(http.StatusServiceUnavailable, body, "application/octet-stream").DecodeError()

	if !errors.Is(gerr, io.ErrUnexpectedEOF) {
		t.Error("the read failure should stay visible as the local cause")
	}

	context := troubleshooting(t, gerr)
	if _, ok := context["body"]; ok {
		t.Error("an unreadable prefix should not be retained")
	}
	if context["body_bytes"] != len(body.prefix) {
		t.Errorf("body_bytes = %v, want the %d bytes received", context["body_bytes"], len(body.prefix))
	}
}

// The Content-Type header is peer-controlled and its details are wire-visible,
// so only a media type that parses, and nothing it carries beside it, gets that
// far.
func TestResponse_DecodeErrorKeepsOnlyASafeMediaType(t *testing.T) {
	cases := map[string]struct {
		header string
		want   any
	}{
		"parameters are dropped":          {header: `text/plain; charset=utf-8; secret="s3cr3t"`, want: "text/plain"},
		"case is canonical":               {header: "TEXT/HTML", want: "text/html"},
		"extension types are kept":        {header: "application/vnd.acme.thing+json", want: "application/vnd.acme.thing+json"},
		"malformed is dropped":            {header: `text/plain; charset="unterminated`, want: nil},
		"binary is dropped":               {header: "text/\xff\xfe", want: nil},
		"bare token is dropped":           {header: "attachment", want: nil},
		"token-shaped data is dropped":    {header: "customer-s3cr3t-123", want: nil},
		"a disposition is dropped":        {header: `inline; filename="s3cr3t.pdf"`, want: nil},
		"an oversized type is dropped":    {header: strings.Repeat("x", maxMediaTypeComponentBytes+1) + "/plain", want: nil},
		"an oversized subtype is dropped": {header: "text/" + strings.Repeat("x", maxMediaTypeComponentBytes+1), want: nil},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			gerr := errorResponse(http.StatusBadRequest, io.NopCloser(strings.NewReader("nope")), tc.header).DecodeError()

			if got := asErrorT(t, gerr).Details["content_type"]; got != tc.want {
				t.Errorf("content_type = %v, want %v", got, tc.want)
			}
			assertNoLeak(t, gerr, "s3cr3t")
		})
	}
}

// The details of a fallback error are wire-visible: a service that writes the
// decoded error back to its own caller publishes whatever safeMediaType kept.
func TestResponse_DecodeErrorKeepsUnsafeMediaTypesOffTheWire(t *testing.T) {
	cases := map[string]struct {
		header string
		want   any
	}{
		"bare token":  {header: "customer-s3cr3t-123", want: nil},
		"disposition": {header: `attachment; filename="s3cr3t.pdf"`, want: nil},
		"media type":  {header: "text/html; charset=utf-8", want: "text/html"},
	}

	for name, tc := range cases {
		for _, status := range []int{http.StatusBadRequest, 499} {
			t.Run(fmt.Sprintf("%s under %d", name, status), func(t *testing.T) {
				gerr := errorResponse(status, io.NopCloser(strings.NewReader("nope")), tc.header).DecodeError()

				rec := httptest.NewRecorder()
				rest.WriteError(gerr).Must(rec)

				var dto errors.DTO
				if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
					t.Fatalf("decoding the wire body %q: %v", rec.Body.String(), err)
				}

				if got := dto.Details["content_type"]; got != tc.want {
					t.Errorf("wire content_type = %v, want %v", got, tc.want)
				}
				if dto.Details["http_status"] == nil {
					t.Error("the safe metadata should still reach the wire")
				}
				if strings.Contains(rec.Body.String(), "s3cr3t") {
					t.Errorf("the wire DTO carries the rejected header: %s", rec.Body.String())
				}
			})
		}
	}
}

// A registered kind stays authoritative for classification and for what the DTO
// says, while the status the error reports is the one its response arrived with.
func TestResponse_DecodeErrorRestoresTheTransportStatusOfARegisteredDTO(t *testing.T) {
	raw, err := json.Marshal(errors.DTO{
		Name:    errors.KindNotFoundError.FQN(),
		Error:   "user 42 does not exist",
		Code:    errors.KindNotFoundError.Code,
		Origin:  errors.KindSystemError.Code,
		Details: map[string]any{"hint": "check the identifier"},
	})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	for _, status := range []int{http.StatusBadGateway, 599} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			resp := errorResponse(status, io.NopCloser(bytes.NewReader(raw)), "application/json")

			gerr := resp.DecodeError()

			if !errors.IsKind(gerr, errors.KindNotFoundError) {
				t.Fatalf("kind = %v, want KindNotFoundError", gerr)
			}
			if !errors.IsKind(gerr, errors.KindUserError) {
				t.Error("semantic class matching should survive a 5xx transport status")
			}

			e := asErrorT(t, gerr)
			if got := e.StatusCode(); got != status {
				t.Errorf("StatusCode() = %d, want the transport %d", got, status)
			}
			if got := e.Kind().StatusCode(); got != status {
				t.Errorf("Kind().StatusCode() = %d, want the transport %d", got, status)
			}

			if gerr.Error() != "user 42 does not exist" {
				t.Errorf("message = %q, want the peer's, with no frame added", gerr.Error())
			}
			if e.Unwrap() != nil {
				t.Errorf("cause = %v, want none added", e.Unwrap())
			}
			if got := e.Details["hint"]; got != "check the identifier" {
				t.Errorf("details hint = %v", got)
			}
			if code, ok := errors.OriginCodeOf(gerr); !ok || code != errors.KindSystemError.Code {
				t.Errorf("origin = %q, %v, want %q", code, ok, errors.KindSystemError.Code)
			}

			dto := e.ErrorDTO()
			if dto.Code != errors.KindNotFoundError.Code || dto.Name != errors.KindNotFoundError.FQN() {
				t.Errorf("wire DTO = %+v, want the peer's kind", dto)
			}

			if resp.StatusCode != status {
				t.Errorf("Response.StatusCode = %d, want the transport %d", resp.StatusCode, status)
			}
		})
	}

	if got := errors.KindNotFoundError.StatusCode(); got != http.StatusNotFound {
		t.Errorf("the registered kind now reports %d after two responses, want 404", got)
	}
}

// The status a registered DTO reports has to reach the wire when a service
// writes the decoded error back to its own caller.
func TestResponse_DecodeErrorRegisteredDTOReachesTheWireUnderTheTransportStatus(t *testing.T) {
	raw, err := json.Marshal(errors.New(errors.KindNotFoundError, "user 42 does not exist").ErrorDTO())
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	gerr := errorResponse(599, io.NopCloser(bytes.NewReader(raw)), "application/json").DecodeError()

	rec := httptest.NewRecorder()
	rest.WriteError(gerr).Must(rec)

	if rec.Code != 599 {
		t.Errorf("wire status = %d, want 599", rec.Code)
	}

	var dto errors.DTO
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatalf("decoding the wire body %q: %v", rec.Body.String(), err)
	}
	if dto.Code != errors.KindNotFoundError.Code {
		t.Errorf("wire kind = %q, want %q", dto.Code, errors.KindNotFoundError.Code)
	}
	if dto.Error != "user 42 does not exist" {
		t.Errorf("wire message = %q, want the peer's", dto.Error)
	}
}

// Restoring the transport status must not reclassify the peer's kind. A
// registered system kind that arrives under a 4xx status is still a system
// error, so serializing the decoded error back to this service's own caller
// sanitizes it as usual.
func TestResponse_DecodeErrorRegisteredSystemDTOUnder4xxStaysSanitized(t *testing.T) {
	const secret = "dial tcp 10.0.0.5:7233: connection refused"

	raw, err := json.Marshal(errors.DTO{
		Name:    errors.KindSystemError.FQN(),
		Error:   secret,
		Code:    errors.KindSystemError.Code,
		Details: map[string]any{"hint": "internal detail"},
	})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	for _, status := range []int{http.StatusBadRequest, http.StatusNotFound, 499} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			gerr := errorResponse(status, io.NopCloser(bytes.NewReader(raw)), "application/json").DecodeError()

			if !errors.IsKind(gerr, errors.KindSystemError) {
				t.Fatalf("kind = %v, want KindSystemError", gerr)
			}
			if errors.IsKind(gerr, errors.KindUserError) {
				t.Error("a 4xx transport status must not reclassify a system kind")
			}
			if got := asErrorT(t, gerr).StatusCode(); got != status {
				t.Errorf("StatusCode() = %d, want the transport %d", got, status)
			}

			rec := httptest.NewRecorder()
			rest.WriteError(gerr).Must(rec)

			if rec.Code != status {
				t.Errorf("wire status = %d, want %d", rec.Code, status)
			}

			var dto errors.DTO
			if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
				t.Fatalf("decoding the wire body %q: %v", rec.Body.String(), err)
			}

			generic := errors.KindForStatus(status)
			if dto.Code != generic.Code || dto.Error != generic.Description {
				t.Errorf("body: want the generic kind for %d, got %+v", status, dto)
			}
			if strings.Contains(rec.Body.String(), "10.0.0.5") {
				t.Errorf("the peer's system message reached the wire: %s", rec.Body.String())
			}
			if dto.Details["hint"] == "internal detail" {
				t.Error("the peer's own hint crossed the wire")
			}
			if dto.Origin != errors.KindSystemError.Code {
				t.Errorf("origin = %q, want the system kind code %q", dto.Origin, errors.KindSystemError.Code)
			}
		})
	}

	if got := errors.KindSystemError.StatusCode(); got != http.StatusInternalServerError {
		t.Errorf("the registered kind now reports %d after three responses, want 500", got)
	}
}

// A full round trip through a garlic service: the status the peer pinned on the
// kind reaches the wire, and the client reads it back as the exact code.
func TestResponse_DecodeErrorRoundTripsCustomizedStatuses(t *testing.T) {
	cases := map[string]struct {
		served      error
		status      int
		kind        *errors.Kind
		wantMessage string
	}{
		"user error under 499": {
			served:      errors.New(errors.KindNotFoundError.CustomizeStatusCode(499), "user 42 does not exist"),
			status:      499,
			kind:        errors.KindNotFoundError,
			wantMessage: "user 42 does not exist",
		},
		"sanitized system error under 599": {
			served:      errors.New(errors.KindSystemError.CustomizeStatusCode(599), "dial tcp 10.0.0.5:7233: connection refused"),
			status:      599,
			kind:        errors.KindSystemError,
			wantMessage: errors.KindSystemError.Description,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			client := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
				rest.WriteError(tc.served).Must(w)
			})

			resp, err := client.R(context.Background()).Get("/users/42")
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			if resp.StatusCode != tc.status {
				t.Fatalf("Response.StatusCode = %d, want %d", resp.StatusCode, tc.status)
			}

			gerr := resp.DecodeError()
			if !errors.IsKind(gerr, tc.kind) {
				t.Fatalf("kind = %v, want %s", gerr, tc.kind.Name)
			}
			if got := asErrorT(t, gerr).StatusCode(); got != tc.status {
				t.Errorf("StatusCode() = %d, want %d", got, tc.status)
			}
			if gerr.Error() != tc.wantMessage {
				t.Errorf("message = %q, want %q", gerr.Error(), tc.wantMessage)
			}
			assertNoLeak(t, gerr, "10.0.0.5")
		})
	}
}

func TestResponse_DecodeErrorPreservesNonStandardStatuses(t *testing.T) {
	cases := []struct {
		status int
		class  *errors.Kind
	}{
		{499, errors.KindUserError},
		{599, errors.KindSystemError},
	}

	for _, tc := range cases {
		t.Run(fmt.Sprint(tc.status), func(t *testing.T) {
			gerr := errorResponse(tc.status, io.NopCloser(strings.NewReader("upstream gave up")), "text/plain").DecodeError()

			if !errors.IsKind(gerr, tc.class) {
				t.Fatalf("kind = %v, want %s", gerr, tc.class.Name)
			}
			if got := asErrorT(t, gerr).StatusCode(); got != tc.status {
				t.Errorf("StatusCode() = %d, want %d", got, tc.status)
			}
		})
	}
}

func TestResponse_DecodeErrorFallsBackOnEmptyBodies(t *testing.T) {
	cases := map[string]*Response{
		"nil body":     errorResponse(499, nil, ""),
		"http.NoBody":  errorResponse(http.StatusInternalServerError, http.NoBody, ""),
		"zero bytes":   errorResponse(http.StatusInternalServerError, io.NopCloser(strings.NewReader("")), "text/html"),
		"head request": errorResponse(499, http.NoBody, "text/html"),
	}

	for name, resp := range cases {
		t.Run(name, func(t *testing.T) {
			status := resp.StatusCode
			gerr := resp.DecodeError()

			if !errors.IsKind(gerr, errors.KindForStatus(status)) {
				t.Fatalf("kind = %v, want the kind for %d", gerr, status)
			}
			if got := asErrorT(t, gerr).StatusCode(); got != status {
				t.Errorf("StatusCode() = %d, want %d", got, status)
			}
			if got := asErrorT(t, gerr).Details["body_bytes"]; got != 0 {
				t.Errorf("body_bytes = %v, want 0", got)
			}
		})
	}
}

func TestResponse_DecodeErrorKeepsThePrefixOfAFailedRead(t *testing.T) {
	body := &failingBody{prefix: []byte("upstream said: quota exceeded")}

	gerr := errorResponse(599, body, "text/plain").DecodeError()

	if !errors.IsKind(gerr, errors.KindSystemError) {
		t.Fatalf("kind = %v, want the 5xx class", gerr)
	}
	if got := asErrorT(t, gerr).StatusCode(); got != 599 {
		t.Errorf("StatusCode() = %d, want 599", got)
	}
	if !errors.Is(gerr, io.ErrUnexpectedEOF) {
		t.Error("the read failure should stay visible as the local cause")
	}

	context := troubleshooting(t, gerr)
	if context["body"] != string(body.prefix) {
		t.Errorf("body snippet = %v, want the received prefix", context["body"])
	}
	if got := asErrorT(t, gerr).Details["body_bytes"]; got != len(body.prefix) {
		t.Errorf("body_bytes = %v, want %d", got, len(body.prefix))
	}
	assertNoLeak(t, gerr, "quota exceeded")
}

func TestResponse_DecodeErrorAlwaysClosesTheBody(t *testing.T) {
	bodies := map[string]string{
		"registered":   fmt.Sprintf(`{"error":"missing","kind":%q}`, errors.KindNotFoundError.Code),
		"unknown kind": `{"error":"boom","kind":"Z1"}`,
		"non garlic":   "<html></html>",
		"empty":        "",
	}

	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			recorder := &recordingBody{Reader: strings.NewReader(body)}
			_ = errorResponse(http.StatusInternalServerError, recorder, "").DecodeError()

			if recorder.closed != 1 {
				t.Errorf("Close called %d times, want 1", recorder.closed)
			}
		})
	}

	t.Run("misuse guard", func(t *testing.T) {
		const body = `{}`
		recorder := &recordingBody{Reader: strings.NewReader(body)}
		gerr := errorResponse(http.StatusOK, recorder, "").DecodeError()

		if !errors.IsKind(gerr, KindResponseDecodeError) {
			t.Errorf("kind = %v, want KindResponseDecodeError", gerr)
		}
		if recorder.read != len(body) {
			t.Errorf("read %d bytes, want the body drained before closing", recorder.read)
		}
		if recorder.closed != 1 {
			t.Errorf("Close called %d times, want 1", recorder.closed)
		}
	})

	t.Run("head request", func(t *testing.T) {
		recorder := &recordingBody{Reader: strings.NewReader("")}
		c := newClientWithTransport(t, RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusServiceUnavailable,
				Status:     http.StatusText(http.StatusServiceUnavailable),
				Header:     http.Header{},
				Body:       recorder,
				Request:    req,
			}, nil
		}), &Config{Retry: RetryConfig{}})

		resp, err := c.R(context.Background()).Head("/x")
		if err != nil {
			t.Fatalf("Head: %v", err)
		}

		gerr := resp.DecodeError()
		if got := asErrorT(t, gerr).StatusCode(); got != http.StatusServiceUnavailable {
			t.Errorf("StatusCode() = %d, want 503", got)
		}
		if got := asErrorT(t, gerr).Details["body_bytes"]; got != 0 {
			t.Errorf("body_bytes = %v, want 0", got)
		}
		if recorder.closed != 1 {
			t.Errorf("Close called %d times, want 1", recorder.closed)
		}

		if err := resp.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
		if recorder.closed != 1 {
			t.Errorf("Close called %d times after an explicit Close, want 1", recorder.closed)
		}
	})

	t.Run("partial read", func(t *testing.T) {
		body := &failingBody{prefix: []byte("half")}
		_ = errorResponse(http.StatusInternalServerError, body, "").DecodeError()

		if body.closed != 1 {
			t.Errorf("Close called %d times, want 1", body.closed)
		}
	})
}

func TestParseRetryAfter(t *testing.T) {
	if d, ok := parseRetryAfter("5"); !ok || d != 5*time.Second {
		t.Errorf("delta-seconds: got %v, %v", d, ok)
	}
	if _, ok := parseRetryAfter(""); ok {
		t.Error("empty should be (0, false)")
	}
	if _, ok := parseRetryAfter("garbage"); ok {
		t.Error("garbage should be (0, false)")
	}
	if _, ok := parseRetryAfter("-3"); ok {
		t.Error("negative should be (0, false)")
	}
	future := time.Now().Add(10 * time.Second).UTC().Format(http.TimeFormat)
	if d, ok := parseRetryAfter(future); !ok || d <= 0 {
		t.Errorf("HTTP-date: got %v, %v", d, ok)
	}
}
