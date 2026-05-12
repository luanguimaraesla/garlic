// Package keyvals adapts a garlic logger to the keyvals-style logger
// interface used by several Go libraries, including Temporal Go SDK
// (go.temporal.io/sdk/log.Logger), go-kit log, and log15. The shape is:
//
//	Debug(msg string, kv ...any)
//	Info(msg string, kv ...any)
//	Warn(msg string, kv ...any)
//	Error(msg string, kv ...any)
//	With(kv ...any) Logger
//
// Use [NewLogger] to wrap a [logging.Logger] and pass the result to
// any library expecting that interface:
//
//	client.Dial(client.Options{
//	    Logger: keyvals.NewLogger(logging.Global()),
//	})
//
// The returned [Logger] is a concrete type. Go's structural typing
// lets it satisfy the target interface at the call site without
// garlic taking a build-time dependency on those third-party SDKs.
package keyvals
