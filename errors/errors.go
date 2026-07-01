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

// Propagate creates a new ErrorT instance with a default error kind (KindError),
// a specified message, and additional options. It wraps an existing error with
// this new instance, allowing for enhanced error tracking and debugging. This
// function is useful for propagating errors while maintaining comprehensive
// error context and metadata.
func Propagate(err error, message string, opts ...Opt) *ErrorT {
	kind := KindError
	if e, ok := err.(*ErrorT); ok {
		kind = e.Kind()
	}

	return PropagateAs(kind, err, message, opts...)
}

// PropagateAs creates a new ErrorT instance with a specified error kind, message, and options,
// and wraps an existing error with this new instance. It appends additional options for
// reverse trace and stack trace to the provided options, ensuring that the error context
// is enriched with detailed tracing information. This function is useful for propagating
// errors with a specific kind while maintaining comprehensive error tracking and debugging capabilities.
func PropagateAs(kind *Kind, err error, message string, opts ...Opt) *ErrorT {
	opts = append(opts, RevTrace())
	return From(kind, err, message, opts...)
}

// New creates a new instance of ErrorT with the specified kind, message, and options.
// It initializes the ErrorT structure, sets the kind and message, and processes the provided
// options by inserting them into the opts map using the insert method. This function is
// essential for constructing error objects with additional context and metadata, which can
// be used for detailed error reporting and handling.
func New(kind *Kind, message string, opts ...Opt) *ErrorT {
	opts = append(opts, RevTrace())
	return Raw(kind, message, opts...)
}

// Raw creates a new instance of ErrorT with the specified kind, message, and options.
// Unlike the New function, it does not append additional options for stack trace or reverse trace.
// This function is useful when you want to create an error object without automatically
// adding tracing information, allowing for more control over the error's metadata and context.
func Raw(kind *Kind, message string, opts ...Opt) *ErrorT {
	return newErrorT(kind, message, nil).With(opts...)
}

// From creates a new ErrorT instance from an existing error, using the error's
// message and kind (if available) as the basis for the new instance. It allows
// for additional options to be specified, which are inserted into the new ErrorT
// instance. This function is useful for converting standard errors into ErrorT
// instances, enabling enhanced error tracking and handling with additional context
// and metadata.
func From(kind *Kind, err error, message string, opts ...Opt) *ErrorT {
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

// Kind returns the kind of the ErrorT instance.
// This method provides access to the error kind, which is used to
// categorize and identify the nature of the error. It is useful for
// error handling and reporting, allowing developers to determine the
// specific type of error encountered.
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

// With applies opts to the error in place and returns it, so options can be
// attached fluently after construction.
func (e *ErrorT) With(opts ...Opt) *ErrorT {
	for _, opt := range opts {
		opt.Opt(e)
	}

	return e
}

// wrap takes an existing error and wraps it with the current ErrorT instance,
// incorporating any options from the existing error into the current instance.
// If the existing error is of type ErrorT, its options are merged into the current
// instance using the insert method. This allows for the aggregation of error
// context and metadata, facilitating enhanced error tracking and debugging.
func (e *ErrorT) wrap(other error) *ErrorT {
	if other == nil {
		return e
	}

	if o, ok := other.(*ErrorT); ok {
		e.Details = o.Details
		e.Troubleshooting = o.Troubleshooting
	}

	e.cause = other
	return e
}

// Unwrap returns the wrapped error from the ErrorT instance.
// This method is used to retrieve the original error that was wrapped
// by the ErrorT instance, enabling error unwrapping and inspection
// in error handling workflows.
func (e *ErrorT) Unwrap() error {
	return e.cause
}

// Error returns the error message for the ErrorT instance.
// If the ErrorT instance wraps another error, this method
// appends the wrapped error's message to the current error
// message, providing a complete error description. This is
// useful for error reporting and logging, as it gives a
// comprehensive view of the error chain.
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

// MarshalLogObject encodes the ErrorT instance into a zapcore.ObjectEncoder for structured logging.
// This method adds the error message, kind, and any additional details from the options map to the
// encoder. It ensures that all relevant error information is captured in the log, facilitating
// comprehensive error tracking and debugging when using the zap logging library.
func (e *ErrorT) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("message", e.message)
	enc.AddString("error", e.Error())
	enc.AddString("code", e.kind.Code)
	enc.AddString("kind", e.kind.FQN())
	enc.AddInt("error_status_code", e.kind.StatusCode())
	_ = enc.AddReflected("details", e.Details)
	_ = enc.AddReflected("troubleshooting", e.Troubleshooting)

	return nil
}

// Zap creates a zap.Field for logging an error using the zap logging library.
// If the provided error is of type ErrorT, it logs the error as a zap object,
// which includes detailed error information such as the message, kind, and options.
// Otherwise, it logs the error using zap.Error, which captures the error message
// and stack trace. This function is useful for integrating structured error logging
// into applications using the zap logging framework.
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
