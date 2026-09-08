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
// Those values travel in the troubleshooting context, so they reach structured
// logs rather than the error DTO a service writes back to its own callers. A
// registered DTO is not bounded here: it is kept as the peer sent it.
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

// DecodeError turns an error response into a garlic error and closes the body.
// Callers can use errors.IsKind on the returned error.
//
// The body is classified in three steps. A garlic DTO whose kind is registered
// in this program decodes faithfully, keeping the peer's kind, message, details,
// and origin, and tolerating fields added by a newer garlic. A DTO carrying a
// kind this program does not know becomes a [KindUnknownResponseError] naming
// the received code, which is how a registration or version mismatch between
// services surfaces. Everything else, from a proxy's HTML page to malformed
// JSON to an empty body, becomes an error of the kind for the upstream status.
//
// Every error built here reports the exact upstream status through
// [errors.ErrorT.StatusCode], including a non-standard code such as 499 or 599
// that its kind can only classify as a 4xx or 5xx class. A registered DTO is the
// exception: its kind is authoritative and it reports that kind's status, the
// one the peer named rather than the one its response travelled under. The
// transport status stays on [Response.StatusCode] either way.
//
// What the peer wrote into a body that is not a DTO is kept only in the
// troubleshooting context, bounded at maxDiagnosticBytes and only while it is
// valid UTF-8, so it reaches logs without crossing the wire again. The
// identifiers of an unknown kind obey the same bound, and the received code is
// also named, quoted and bounded, in the error message. A registered DTO instead
// keeps its message and details as the peer sent them, under no bound of its
// own: whether those may be forwarded is the ordinary error-class decision that
// rest.WriteError makes for any garlic error.
//
// The body is drained and closed on every path, including the guard that rejects
// a response without an error status. That guard reports a local
// [KindResponseDecodeError] rather than the response status, because a caller
// reaching it asked the wrong question of a response that did not fail.
func (r *Response) DecodeError() error {
	if r == nil || r.Response == nil {
		return errors.New(KindResponseDecodeError, "response body is empty")
	}
	defer r.drainAndClose()

	if !r.IsError() {
		return errors.New(KindResponseDecodeError, "failed to decode a non-error response into error")
	}

	status := r.StatusCode
	mediaType := safeMediaType(r.Header.Get("Content-Type"))

	var (
		body    []byte
		readErr error
	)
	if r.Body != nil {
		body, readErr = io.ReadAll(r.Body)
	}

	switch {
	case readErr != nil:
		// Keep the prefix that did arrive: a truncated diagnostic is still a
		// diagnostic, and the read failure stays visible as the local cause.
		return errors.From(errors.KindForStatus(status), readErr,
			"failed reading the upstream error response",
			fallbackOpts(status, mediaType, captureDiagnostic(body))...)

	case len(body) == 0:
		return errors.New(errors.KindForStatus(status),
			"upstream returned an empty error response",
			fallbackOpts(status, mediaType, capture{})...)
	}

	dto, raw, ok := parseErrorDTO(body)
	if !ok {
		return errors.New(errors.KindForStatus(status),
			"upstream returned a non-garlic error response",
			fallbackOpts(status, mediaType, captureDiagnostic(body))...)
	}

	if gerr, registered := dto.Decode(); registered {
		return gerr
	}

	// parseErrorDTO already required kind to arrive as a non-empty string.
	code, _ := capturePeerString(raw.Code)

	// The generic kind for the upstream status sits underneath as the cause, so
	// errors.IsKind still matches the status class while the outer kind marks
	// the mismatch.
	return errors.From(
		KindUnknownResponseError,
		errors.Raw(errors.KindForStatus(status), "upstream returned an error status", errors.Status(status)),
		fmt.Sprintf("upstream returned error kind %s, which is not registered here", renderReceived(code)),
		errors.Status(status),
		errors.Context(unknownKindEntries(status, raw, code)...),
	)
}

// rawFields holds the identifiers of an error DTO as they arrived on the wire.
// A kind mismatch reports them, and encoding/json rewrites invalid UTF-8 into
// U+FFFD while decoding, so the decoded strings can no longer tell a readable
// identifier from a laundered binary one.
type rawFields struct {
	Code    json.RawMessage `json:"kind"`
	Name    json.RawMessage `json:"name"`
	Message json.RawMessage `json:"error"`
}

// parseErrorDTO decodes body as a garlic error DTO, reporting false for anything
// that is not one. A JSON object qualifies only when it carries both core
// fields, error and a non-empty kind. Requiring the pair is what keeps foreign
// JSON that happens to have a kind discriminator out of the garlic branch, so
// its diagnostics survive instead of decoding into an empty garlic error or a
// fake registration mismatch.
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
// is peer-controlled and its parameters can carry anything, while the details of
// a fallback error are wire-visible, so only a media type that parses and stays
// within the length its grammar allows gets that far. Anything else is dropped.
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

// fallbackOpts is the shape shared by every error DecodeError builds itself: the
// exact upstream status, wire-visible metadata about the response, and the
// bounded body snippet, which stays in the troubleshooting context.
func fallbackOpts(status int, mediaType string, body capture) []errors.Opt {
	details := diagnostics{
		"http_status": status,
		"body_bytes":  body.length,
		"truncated":   body.truncated,
	}

	if mediaType != "" {
		details["content_type"] = mediaType
	}

	return []errors.Opt{
		errors.Status(status),
		details,
		errors.Context(body.entries("body")...),
	}
}

// unknownKindEntries records what the peer called its kind. These identifiers
// are peer-controlled and can be arbitrarily long, so each goes through the same
// bound as a response body.
func unknownKindEntries(status int, raw rawFields, code capture) []errors.Entry {
	entries := []errors.Entry{errors.Field("http_status", status)}
	entries = append(entries, code.entries("received_kind")...)

	if name, ok := capturePeerString(raw.Name); ok {
		entries = append(entries, name.entries("received_name")...)
	}
	if message, ok := capturePeerString(raw.Message); ok {
		entries = append(entries, message.entries("received_message")...)
	}

	return entries
}

// capturePeerString bounds one identifier of an error DTO, judging and keeping
// the same raw JSON member. encoding/json rewrites invalid UTF-8 into U+FFFD
// while decoding, and leaves a field as an earlier duplicate member left it when
// a later one is null, so a decoded string can describe an occurrence other than
// the one judged here.
//
// It reports false for a member the peer did not send as a non-empty string,
// which names no identifier to report.
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
// expands control characters, so the bound applies to the escaped form: quoting
// an already bounded value would let an identifier made of escapes render
// several times over the budget. One the retention policy dropped is named by
// its length alone.
func renderReceived(c capture) string {
	if !c.readable {
		return fmt.Sprintf("<%d unreadable bytes>", c.length)
	}

	return captureDiagnostic([]byte(strconv.Quote(c.text))).text
}

// diagnostics copies safe response metadata into the details of an error.
type diagnostics map[string]any

func (d diagnostics) Opt(e *errors.ErrorT) {
	for key, value := range d {
		e.Details[key] = value
	}
}

// capture is a bounded, log-safe view of a peer-controlled value.
type capture struct {
	text      string
	length    int
	truncated bool
	readable  bool
}

// captureDiagnostic bounds a peer-controlled value at maxDiagnosticBytes,
// cutting on a rune boundary. The text is kept only when the bounded prefix is
// valid UTF-8, so a binary body is reduced to its length and nothing unprintable
// reaches a log line.
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

// boundedPrefix returns the longest run of whole runes that fits in
// maxDiagnosticBytes. It reports false as soon as the bounded region holds a
// byte that begins no valid rune: deleting those until the rest decodes would
// launder binary into text that merely looks readable, so such a value keeps
// nothing but its length. A rune the bound itself cuts in half is a different
// matter, and simply stays out of the prefix. Bytes beyond the budget are never
// read: they are not retained either, so they have no say in whether the prefix
// before them is.
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

// entries renders the capture as troubleshooting fields. The value itself is
// omitted when it was not readable, leaving the length behind as the only
// evidence.
func (c capture) entries(key string) []errors.Entry {
	entries := []errors.Entry{
		errors.Field(key+"_bytes", c.length),
		errors.Field(key+"_truncated", c.truncated),
	}

	if c.readable {
		entries = append(entries, errors.Field(key, c.text))
	}

	return entries
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
