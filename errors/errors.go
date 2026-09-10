package errors

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Opt interface {
	Opt(e *ErrorT)
}

type Troubleshooting struct {
	ReverseTrace []string
	StackTrace   string
	Context      map[string]any
}

type ErrorT struct {
	kind            *Kind
	message         string
	cause           error
	origin          error
	Details         map[string]any
	Troubleshooting Troubleshooting
}

// newErrorT builds an ErrorT with its fields set and an empty details map. Every
// exported builder funnels through it so origin and details are set the same way;
// a cause is attached separately through wrap.
func newErrorT(kind *Kind, message string, origin error) *ErrorT {
	return &ErrorT{
		kind:    kind,
		message: message,
		origin:  origin,
		Details: map[string]any{},
	}
}

// Propagate wraps an existing error with a new message, inheriting the kind of
// the wrapped error when it is an ErrorT and falling back to KindError
// otherwise. It is the standard way to carry an error across a boundary while
// keeping its context and metadata.
//
// When err is a nil error interface, Propagate returns nil without applying
// options or capturing any propagation metadata, so a failure-free result stays
// failure-free. A non-nil result is always backed by *ErrorT. A typed nil stored
// inside an error interface is not nil and is not normalized.
func Propagate(err error, message string, opts ...Opt) error {
	if err == nil {
		return nil
	}

	kind := KindError
	if e, ok := err.(*ErrorT); ok {
		kind = e.Kind()
	}

	return PropagateAs(kind, err, message, opts...)
}

// PropagateAs wraps an existing error with an explicit kind and message,
// appending a reverse trace entry so the propagation path stays visible.
//
// When err is a nil error interface, PropagateAs returns nil without applying
// options or capturing a reverse trace. A non-nil result is always backed by
// *ErrorT. A typed nil stored inside an error interface is not nil and is not
// normalized.
func PropagateAs(kind *Kind, err error, message string, opts ...Opt) error {
	if err == nil {
		return nil
	}

	opts = append(opts, RevTrace())
	return From(kind, err, message, opts...)
}

// New builds an error of the given kind and message, appending a reverse trace
// entry so the origin of the failure stays visible.
func New(kind *Kind, message string, opts ...Opt) *ErrorT {
	opts = append(opts, RevTrace())
	return Raw(kind, message, opts...)
}

// Raw is New without the automatic reverse trace entry, for an error whose
// metadata the caller wants to control entirely.
func Raw(kind *Kind, message string, opts ...Opt) *ErrorT {
	return newErrorT(kind, message, nil).With(opts...)
}

// From builds an error of the given kind that wraps err, carrying over the
// details and troubleshooting data of an ErrorT cause. Unlike PropagateAs it
// adds no reverse trace entry, so the caller controls the metadata through opts.
//
// When err is a nil error interface, From returns nil without allocating or
// applying options. A non-nil result is always backed by *ErrorT. A typed nil
// stored inside an error interface is not nil and is not normalized.
func From(kind *Kind, err error, message string, opts ...Opt) error {
	if err == nil {
		return nil
	}

	return newErrorT(kind, message, nil).wrap(err).With(opts...)
}

// Override builds an error that presents kind and message to the outside world
// while keeping origin as a private reference to the failure that really
// happened. The origin never shows up in Error or in the wire message; only its
// kind code travels, through ErrorDTO, so support can trace the real cause
// without the sensitive error leaking to the client. Use it when a raw failure
// must be surfaced as a safer, generic error but you still want the original
// code available for troubleshooting.
func Override(kind *Kind, origin error, message string, opts ...Opt) *ErrorT {
	return newErrorT(kind, message, origin).With(opts...)
}

// Mirror builds an error whose message is the kind's static Description, so it
// captures nothing dynamic or sensitive. The result says only what the kind
// already documents, which makes it safe to hand straight to a client.
func Mirror(kind *Kind, opts ...Opt) *ErrorT {
	return newErrorT(kind, kind.Description, nil).With(opts...)
}

// MirrorOverride combines Mirror and Override: the visible message is the kind's
// static Description, and the failure that really happened is kept as a private
// origin whose kind code still crosses the wire for troubleshooting. It is the
// projection used to sanitize a system error while preserving a reference to its
// cause.
func MirrorOverride(kind *Kind, origin error, opts ...Opt) *ErrorT {
	return newErrorT(kind, kind.Description, origin).With(opts...)
}

// Kind returns the kind that classifies the error.
func (e *ErrorT) Kind() *Kind {
	return e.kind
}

// Origin returns the sanitized origin reference from this error or one it wraps,
// or nil when none. The origin is troubleshooting metadata and never part of Error.
func (e *ErrorT) Origin() error {
	return Origin(e)
}

// HasOrigin reports whether this error or one it wraps carries an origin.
func (e *ErrorT) HasOrigin() bool {
	return Origin(e) != nil
}

// Code is shorthand for the code of the error's kind.
func (e *ErrorT) Code() string {
	return e.Kind().Code
}

// StatusCode returns the HTTP status this error reports, which is the status of
// its kind. Build the error with [Kind.CustomizeStatusCode] when a status
// received from elsewhere must survive verbatim, such as a non-standard 499 or
// 599 that [KindForStatus] can only classify as its 4xx or 5xx class.
func (e *ErrorT) StatusCode() int {
	return e.Kind().StatusCode()
}

// With applies opts to the error in place and returns it, so options can be
// attached fluently after construction.
func (e *ErrorT) With(opts ...Opt) *ErrorT {
	for _, opt := range opts {
		opt.Opt(e)
	}

	return e
}

// wrap attaches other as the cause and carries an ErrorT cause's details,
// troubleshooting data, and pinned status outward, so the metadata gathered
// deeper down survives the boundary.
func (e *ErrorT) wrap(other error) *ErrorT {
	if other == nil {
		return e
	}

	if o, ok := other.(*ErrorT); ok {
		e.Details = o.Details
		e.Troubleshooting = o.Troubleshooting
		e.inheritStatusCustomization(o.kind)
	}

	e.cause = other
	return e
}

// inheritStatusCustomization carries a status pinned on the cause's kind onto
// the kind this error was built with, so an exact status stays readable after
// the error crosses a boundary. A kind the caller customized itself wins and is
// left alone. The inheriting kind is copied rather than written to, so a
// registered kind or a template's kind is never changed by a propagation.
func (e *ErrorT) inheritStatusCustomization(cause *Kind) {
	if cause == nil || !cause.customStatus || e.kind.customStatus {
		return
	}

	e.kind = e.kind.CustomizeStatusCode(cause.HTTPStatusCode)
}

// Unwrap returns the cause, so the error works with the standard errors package.
func (e *ErrorT) Unwrap() error {
	return e.cause
}

// Error returns the message, followed by the cause chain when there is one. The
// origin is troubleshooting metadata and never appears here.
func (e *ErrorT) Error() string {
	message := e.message
	if e.cause != nil {
		message = fmt.Sprintf("%s: %s", message, e.cause.Error())
	}

	return message
}

// ErrorDTO converts the error into its transferable DTO: the kind's name and
// code, the dynamic message, and the details. When an origin is attached, its
// kind code is copied into DTO.Origin so a sanitized error can still point the
// receiver at the underlying failure. Only the origin's code travels; its
// message, name, and details stay behind.
func (e *ErrorT) ErrorDTO() *DTO {
	return &DTO{
		Name:    e.kind.FQN(),
		Error:   e.message,
		Code:    e.kind.Code,
		Origin:  CodeOf(e.origin),
		Details: e.Details,
	}
}

// Description returns the static, human-readable description of the error's
// kind. Unlike the dynamic message, it is authored at compile time and is safe
// to expose to clients.
func (e *ErrorT) Description() string {
	return e.kind.Description
}

// MarshalLogObject encodes the error for structured logging, including its
// details and troubleshooting data.
func (e *ErrorT) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("message", e.message)
	enc.AddString("error", e.Error())
	enc.AddString("code", e.kind.Code)
	enc.AddString("kind", e.kind.FQN())
	enc.AddInt("error_status_code", e.StatusCode())
	_ = enc.AddReflected("details", e.Details)
	_ = enc.AddReflected("troubleshooting", e.Troubleshooting)

	return nil
}

// Zap returns a zap.Field for err, logging a garlic error as a structured object
// and anything else through zap.Error.
func Zap(err error) zap.Field {
	if e, ok := err.(*ErrorT); ok {
		return zap.Object("error", e)
	}

	// Fall back to the chain so wrapped errors (for example an *ErrorT embedded
	// in a transport-level error) still log with their full structured context.
	var e *ErrorT
	if As(err, &e) {
		return zap.Object("error", e)
	}

	return zap.Error(err)
}
