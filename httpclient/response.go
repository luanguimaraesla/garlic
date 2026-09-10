package httpclient

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/luanguimaraesla/garlic/errors"
)

// maxDiagnosticBytes bounds what DecodeError keeps from a body it could not
// decode as a garlic DTO, and the identifiers of a DTO whose kind is unknown.
// Those values travel in the private troubleshooting context; a registered DTO
// is not bounded here, it is kept as the peer sent it.
const maxDiagnosticBytes = 4096

// maxMediaTypeComponentBytes bounds each half of the media type kept from a
// Content-Type header. RFC 6838 allows 127 characters for a type and 127 for a
// subtype.
const maxMediaTypeComponentBytes = 127

// Response wraps the raw HTTP response returned by the upstream service. The
// body stays open unless a helper such as Decode or DecodeError consumes it.
type Response struct {
	*http.Response
}

// IsSuccess reports whether the status is in the 2xx range.
func (r *Response) IsSuccess() bool { return r.StatusCode >= 200 && r.StatusCode < 300 }

// IsError reports whether the status is 400 or greater.
func (r *Response) IsError() bool { return r.StatusCode >= 400 }

// RetryAfter returns the Retry-After header parsed as a duration. It reports
// false when the header is missing or invalid. A valid header can return a zero
// duration, such as Retry-After: 0 or a past HTTP date.
func (r *Response) RetryAfter() (time.Duration, bool) {
	if r == nil || r.Response == nil {
		return 0, false
	}

	return parseRetryAfter(r.Header.Get("Retry-After"))
}

// Decode JSON-decodes the response body into v and closes it.
func (r *Response) Decode(v any) error {
	if r == nil || r.Response == nil || r.Body == nil {
		return errors.New(KindResponseDecodeError, "response body is empty")
	}
	defer r.drainAndClose()

	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(v); err != nil {
		return errors.PropagateAs(KindResponseDecodeError, err, "failed to decode response body")
	}

	return nil
}

// DecodeError decodes an upstream error with its HTTP status and closes the body.
// Unknown kinds and non-Garlic bodies become local errors with private diagnostics.
func (r *Response) DecodeError() error {
	if r == nil || r.Response == nil {
		return errors.New(KindResponseDecodeError, "response body is empty")
	}
	defer r.drainAndClose()

	if !r.IsError() {
		return errors.New(KindResponseDecodeError, "failed to decode a non-error response into error")
	}

	status := r.StatusCode

	var (
		body    []byte
		readErr error
	)
	if r.Body != nil {
		body, readErr = io.ReadAll(r.Body)
	}

	// Only a body that arrived in full is worth reading as a DTO: a truncated one
	// would decode into an error the peer never sent.
	if readErr == nil {
		if dto, raw, ok := parseErrorDTO(body); ok {
			if gerr, registered := dto.DecodeFor(status); registered {
				return gerr
			}

			// parseErrorDTO already required kind to arrive as a non-empty string.
			code, _ := capturePeerString(raw.Code)

			// The generic kind for the upstream status sits underneath as the cause,
			// so errors.IsKind still matches the status class while the outer kind
			// marks the mismatch.
			return errors.From(
				kindWithStatus(KindUnknownResponseError, status),
				errors.Raw(statusKind(status), "upstream returned an error status"),
				fmt.Sprintf("upstream returned error kind %s, which is not registered here", renderReceived(code)),
				errors.Context(unknownKindFields(status, raw, code)...),
			)
		}
	}

	snippet, message := describeBody(body, readErr)

	opts := []errors.Opt{
		errors.Details(responseMetadata(status, safeMediaType(r.Header.Get("Content-Type")), snippet)),
		errors.Context(capturedFields("body", snippet)...),
	}

	if readErr != nil {
		// Keep the prefix that did arrive: a truncated diagnostic is still a
		// diagnostic, and the read failure stays visible as the local cause.
		return errors.From(statusKind(status), readErr, message, opts...)
	}

	return errors.New(statusKind(status), message, opts...)
}

// describeBody bounds what may be retained from a body that is no garlic DTO,
// and names why it is not one. An empty body keeps no snippet, since an empty
// one says nothing a length does not.
func describeBody(body []byte, readErr error) (capture, string) {
	switch {
	case readErr != nil:
		return captureDiagnostic(body), "failed reading the upstream error response"
	case len(body) == 0:
		return capture{}, "upstream returned an empty error response"
	}

	return captureDiagnostic(body), "upstream returned a non-garlic error response"
}

// kindWithStatus pins kind to the exact status a response arrived with. A kind
// that already reports that status is left alone, so a later caller can still
// retype the error.
func kindWithStatus(kind *errors.Kind, status int) *errors.Kind {
	if kind.StatusCode() == status {
		return kind
	}

	return kind.CustomizeStatusCode(status)
}

// statusKind is the generic kind for an upstream status, pinned to that exact
// status so a non-standard 499 or 599 survives a 4xx or 5xx classification.
func statusKind(status int) *errors.Kind {
	return kindWithStatus(errors.KindForStatus(status), status)
}

// rawFields holds the identifiers of an error DTO as they arrived on the wire.
// encoding/json rewrites invalid UTF-8 into U+FFFD while decoding, so a decoded
// string can no longer tell a readable identifier from a laundered binary one.
type rawFields struct {
	Code    json.RawMessage `json:"kind"`
	Name    json.RawMessage `json:"name"`
	Message json.RawMessage `json:"error"`
}

// parseErrorDTO decodes body as a garlic error DTO, reporting false for anything
// that is not one. Requiring both core fields, error and a non-empty kind, keeps
// foreign JSON carrying a kind discriminator out of the garlic branch, so its
// diagnostics survive instead of decoding into an empty garlic error.
func parseErrorDTO(body []byte) (*errors.DTO, rawFields, bool) {
	var envelope struct {
		Error *string `json:"error"`
		Kind  *string `json:"kind"`
	}

	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, rawFields{}, false
	}
	if envelope.Error == nil || envelope.Kind == nil || *envelope.Kind == "" {
		return nil, rawFields{}, false
	}

	// A plain Unmarshal, so fields a newer garlic added are tolerated.
	var dto errors.DTO
	if err := json.Unmarshal(body, &dto); err != nil {
		return nil, rawFields{}, false
	}

	var raw rawFields
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, rawFields{}, false
	}

	return &dto, raw, true
}

// safeMediaType reduces a Content-Type header to its bare media type. The header
// is peer-controlled and the details of a fallback error are wire-visible, so
// anything that does not parse within the length its grammar allows is dropped.
func safeMediaType(header string) string {
	mediaType, _, err := mime.ParseMediaType(header)
	if err != nil {
		return ""
	}

	// The same parser reads a Content-Disposition, so it also accepts a bare
	// token such as attachment. Only a type/subtype pair is a media type.
	mainType, subType, ok := strings.Cut(mediaType, "/")
	if !ok || !mediaTypeComponent(mainType) || !mediaTypeComponent(subType) {
		return ""
	}

	return mediaType
}

func mediaTypeComponent(s string) bool {
	return s != "" && len(s) <= maxMediaTypeComponentBytes
}

// responseMetadata is what an error DecodeError built itself may say about the
// response on the wire again: status, media type when one survived
// safeMediaType, and how much body arrived. The body itself goes to the
// troubleshooting context.
func responseMetadata(status int, mediaType string, body capture) map[string]any {
	metadata := map[string]any{
		"http_status": status,
		"body_bytes":  body.length,
		"truncated":   body.truncated,
	}

	if mediaType != "" {
		metadata["content_type"] = mediaType
	}

	return metadata
}

// unknownKindFields records what the peer called its kind, alongside the name
// and message it sent with it.
func unknownKindFields(status int, raw rawFields, code capture) []errors.Entry {
	fields := []errors.Entry{errors.Field("http_status", status)}
	fields = append(fields, capturedFields("received_kind", code)...)

	if name, ok := capturePeerString(raw.Name); ok {
		fields = append(fields, capturedFields("received_name", name)...)
	}
	if message, ok := capturePeerString(raw.Message); ok {
		fields = append(fields, capturedFields("received_message", message)...)
	}

	return fields
}

// capturedFields reports one bounded peer value under name, name_bytes and
// name_truncated. A value the retention policy dropped keeps its length alone.
func capturedFields(name string, c capture) []errors.Entry {
	fields := []errors.Entry{
		errors.Field(name+"_bytes", c.length),
		errors.Field(name+"_truncated", c.truncated),
	}

	if c.readable {
		fields = append(fields, errors.Field(name, c.text))
	}

	return fields
}

// capturePeerString bounds one identifier of an error DTO, judging and keeping
// the same raw JSON member: encoding/json launders invalid UTF-8 and resolves
// duplicate members, so a decoded string can describe another occurrence. It
// reports false for a member the peer did not send as a non-empty string.
func capturePeerString(raw json.RawMessage) (capture, bool) {
	if len(raw) < 3 || raw[0] != '"' || raw[len(raw)-1] != '"' {
		return capture{}, false
	}

	if !utf8.Valid(raw) {
		// Everything but the quotes around the literal is the value the peer sent,
		// which is the length worth reporting, not the length of the replacements.
		length := len(raw) - 2

		return capture{length: length, truncated: length > maxDiagnosticBytes}, true
	}

	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return capture{}, false
	}

	return captureDiagnostic([]byte(text)), true
}

// renderReceived spells a captured identifier into an error message. Quoting
// expands control characters, so the bound applies to the escaped form; one the
// retention policy dropped is named by its length alone.
func renderReceived(c capture) string {
	if !c.readable {
		return fmt.Sprintf("<%d unreadable bytes>", c.length)
	}

	return captureDiagnostic([]byte(strconv.Quote(c.text))).text
}

// capture is a bounded view of a peer-controlled value.
type capture struct {
	text      string
	length    int
	truncated bool
	readable  bool
}

// captureDiagnostic bounds a peer-controlled value at maxDiagnosticBytes,
// cutting on a rune boundary. The text is kept only when the bounded prefix is
// valid UTF-8, so a binary body is reduced to its length alone.
func captureDiagnostic(value []byte) capture {
	c := capture{length: len(value), truncated: len(value) > maxDiagnosticBytes}

	prefix, ok := boundedPrefix(value)
	if !ok {
		return c
	}

	c.text = string(prefix)
	c.truncated = len(prefix) < len(value)
	c.readable = true

	return c
}

// boundedPrefix returns a whole-rune UTF-8 prefix within maxDiagnosticBytes.
// It rejects invalid runes, checking continuation bytes beyond the limit when
// a rune starts inside it.
func boundedPrefix(value []byte) ([]byte, bool) {
	end := 0
	for end < len(value) && end < maxDiagnosticBytes {
		r, size := utf8.DecodeRune(value[end:])
		if r == utf8.RuneError && size <= 1 {
			return nil, false
		}
		if end+size > maxDiagnosticBytes {
			break
		}

		end += size
	}

	return value[:end], true
}

// Close closes the response body. It is safe to call more than once.
func (r *Response) Close() error {
	if r == nil || r.Response == nil || r.Body == nil {
		return nil
	}

	body := r.Body
	r.Body = nil
	_, _ = io.Copy(io.Discard, io.LimitReader(body, 4<<10))
	return body.Close()
}

func (r *Response) drainAndClose() {
	drainAndClose(r.Body)
	r.Body = nil
}

// drainAndClose reads a bounded tail of the body before closing it so idle
// connections can usually return to the pool.
func drainAndClose(rc io.ReadCloser) {
	if rc == nil {
		return
	}

	_, _ = io.Copy(io.Discard, io.LimitReader(rc, 4<<10))
	_ = rc.Close()
}
