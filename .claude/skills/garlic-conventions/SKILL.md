---
name: garlic-conventions
description: >
  Garlic Go framework conventions for building Web APIs. TRIGGER when: code
  imports "github.com/luanguimaraesla/garlic" or any garlic sub-package
  (errors, rest, middleware, request, database, logging, validator, etc.),
  or user asks about garlic framework patterns. DO NOT TRIGGER when: project
  does not depend on garlic.
---

# Garlic Framework Conventions

Garlic is a Go library for building Web APIs. This skill encodes the "always
do X, never do Y" rules that are not obvious from reading the code alone.

## 1. Error Propagation

This is the most important convention. Garlic errors carry reverse traces and
scoped context that enable structured debugging. Breaking the propagation chain
loses this data silently.

### Rules

- **ALWAYS** use `errors.Propagate(err, message, opts...)` at every function
  boundary where an error is returned. This preserves the original error's kind
  and appends the caller to the reverse trace.
- **ALWAYS** use `errors.PropagateAs(kind, err, message, opts...)` when you need
  to reclassify the error kind at a layer boundary (e.g., a database error that
  should become a user-facing not-found error, or a generic error that should
  become a reconcile error at the controller boundary). Use `Propagate` when the
  original kind should be preserved.
- **ALWAYS** add `errors.Hint(message, args...)` to user-level errors
  (`KindUserError` and its subkinds). Hints are surfaced in API responses and
  help users understand what they did wrong and how to fix it. Hints support
  format strings: `errors.Hint("Check resource %q in namespace %q", name, ns)`.
- **NEVER** use bare `return err` -- it creates a gap in the reverse trace.
  This includes errors that are already garlic errors (e.g. from
  `request.ParseResourceUUID`, `request.Decode*`, or another garlic-aware
  function): propagation appends the current caller to the reverse trace, so
  "it is already a garlic error" is never a reason to return bare.
- **NEVER** return a helper's error result bare, even when the helper
  propagates internally. `return foo{}, classifyX(err)` is wrong; the caller
  wraps it: `return foo{}, errors.Propagate(classifyX(err), "msg", ectx)`.
  The helper's internal propagation covers the helper; the call-site
  `Propagate` is what stitches the calling function into the reverse trace.
  Corollary: classification helpers classify only, they do not take an
  `ectx` parameter (ectx is function-scoped); the caller owns the context.
- **NEVER** wrap errors with `fmt.Errorf("...: %w", err)` -- it bypasses the
  reverse trace and scoped context entirely.
- **NEVER** use `errors.New()` to wrap an existing error -- use `Propagate`.
  `New` is only for creating fresh errors with no cause.
- **NEVER** import the stdlib `errors` package in garlic code. No stdlib
  sentinels: internal discriminator errors are garlic templates/errors
  recognized by kind. All comparisons go through garlic: `errors.Is`,
  `errors.As`, `errors.IsKind`, `errors.AsKind` (garlic's `Is`/`As` delegate
  to the stdlib, so they also match foreign errors like `io.EOF` and
  `*http.MaxBytesError`).

### The only exception: foreign interface contracts

When a type implements an interface owned by the stdlib or a third party
(`io.Reader`, `http.RoundTripper`, `sql.Scanner`, ...), the methods must
honor that contract's error semantics and return errors raw: consumers match
on concrete identity (`io.ReadAll` compares `err == io.EOF` by pointer;
`errors.As` targets like `*http.MaxBytesError` must stay recognizable).
Wrapping inside the implementation breaks the foreign protocol. The garlic
boundary is the first garlic-aware caller of that implementation, and THAT
function must propagate. Record the recorded/terminal error on the type if
the caller needs it for classification.

### Example: 3-layer propagation chain

```go
// Repository layer
func (r *UserRepo) FindByID(ctx context.Context, id uuid.UUID) (*User, error) {
    ectx := errors.Context(errors.Field("user_id", id))
    var user User
    err := r.db.Read(ctx, query, &user, id)
    if err != nil {
        return nil, errors.Propagate(err, "failed to read user", ectx)
    }
    return &user, nil
}

// Service layer
func (s *UserService) Get(ctx context.Context, id uuid.UUID) (*User, error) {
    ectx := errors.Context(errors.Field("user_id", id))
    user, err := s.repo.FindByID(ctx, id)
    if err != nil {
        return nil, errors.Propagate(err, "failed to get user", ectx)
    }
    return user, nil
}

// Handler layer
func (a *UserAPI) Read(w http.ResponseWriter, r *http.Request) error {
    ectx := errors.Context(errors.Field("route", "users.read"))
    id, err := request.ParseResourceUUID(r, "user_id")
    if err != nil {
        // Even though request helpers return garlic errors, the
        // handler still propagates: propagation appends THIS caller
        // to the reverse trace. "Already a garlic error" is never a
        // reason to return bare.
        return errMissingUserID.Propagate(err, ectx)
    }
    user, err := a.service.Get(r.Context(), id)
    if err != nil {
        return errors.Propagate(err, "failed to read user", ectx)
    }
    rest.WriteResponse(http.StatusOK, user).Must(w)
    return nil
}
```

### Error templates

Use templates for reusable error configurations with predefined kind, message,
and hints:

```go
var errNotFound = errors.Template(
    errors.KindNotFoundError,
    "resource not found",
    errors.Hint("Check if the resource exists or the ID is correct."),
)

// Create a fresh error from the template
return errNotFound.New()

// Wrap an existing error with the template's kind and message
return errNotFound.Propagate(cause, ectx)
```

### Errors as flow control signals

**Use with caution.** Prefer standard Go patterns first: returning `(result, bool)`
for existence checks, `nil` for no-op, or multiple return values. Only use
`errors.New` as a signal when standard patterns would create awkward APIs (e.g.,
a method that performs side effects and needs to distinguish "did nothing" from
"failed").

```go
// Helper returns a skip signal when there's nothing to do.
func (r *Repo) ensureRecord(ctx context.Context, id string) error {
    if exists {
        return errors.New(KindSkipError, "record already exists")
    }
    return r.create(ctx, id)
}

// Caller distinguishes signal from real error.
if err := r.repo.ensureRecord(ctx, id); err != nil {
    if !errors.IsKind(err, KindSkipError) {
        return errors.Propagate(err, "failed to ensure record", ectx)
    }
    log.Debug("Record already existed, skipping creation")
}
```

### Error inspection and logging

- Use `errors.IsKind(err, kind)` for hierarchical kind checking. Checking
  `KindUserError` matches all subkinds (Validation, NotFound, Auth, etc.).
- Use `errors.AsKind(err, kind)` to extract the first matching `*ErrorT`.
- **ALWAYS** propagate before logging. Call `errors.Propagate` (or
  `errors.PropagateAs`) first, then log the propagated error with
  `errors.Zap`. This ensures the log entry carries the full reverse trace
  including the current call site.
- **ALWAYS** use `errors.Zap(err)` when logging errors, never `zap.Error(err)`.
  `errors.Zap` preserves the full troubleshooting data (reverse trace, context,
  stack trace) in structured log output.
- **NEVER** log an error and then return it unpropagated. If you log, propagate
  first. If you return, propagate at the return site. The propagation chain
  must never have gaps.

```go
// CORRECT: propagate first, then log
gerr := errors.Propagate(err, "operation failed", ectx)
l.Error("operation failed", errors.Zap(gerr))

// CORRECT: propagate and return (caller handles logging)
return errors.Propagate(err, "operation failed", ectx)

// WRONG: logging raw error without propagation
l.Error("operation failed", errors.Zap(err))  // gap in trace
```

### Error context

Use `errors.Context` to attach scoped debugging fields. Each propagation layer
adds its own scope, keyed by the caller function name:

```go
ectx := errors.Context(
    errors.Field("order_id", orderID),
    errors.Field("quantity", qty),
)
return errors.Propagate(err, "failed to process order", ectx)
```

For sensitive values, use `errors.RedactedString` which shows only a portion of
the value in logs.

#### Error contexts are function-scoped

- **NEVER** pass `*errors.ContextT` as a function parameter. Garlic keys each
  context scope by the function that attached it, so a helper stamping the
  caller's ectx misattributes where the data came from and couples the helper's
  signature to whatever the caller happened to collect.
- **ALWAYS** build `errors.Context(...)` inline, in the function that creates
  or propagates the error, from values in its own scope. Reusing one ectx at
  several error sites within the same function is fine.
- Helpers that need identifying fields take plain data parameters (an ID, a
  name) and build their own context from them. Callers attach their own scope
  when they propagate; duplicate fields across scopes are normal and by design.
- Keep identity fields (e.g. a tenant or resource ID) in the error context even
  when middleware already enriched the request logger with them. Error contexts
  must be self-contained; a duplicated field on log lines is accepted noise.
- Do not echo caller-controlled input (malformed digests, raw headers) into
  context fields; create those errors with `.New()` and no fields.

```go
// WRONG: ectx travels across the function boundary
func validateOrder(form OrderForm, ectx *errors.ContextT) error {
    if form.Quantity <= 0 {
        return errInvalidQuantity.New(ectx) // caller's scope, misattributed
    }
    return nil
}

// CORRECT: each function owns its scope
func validateOrder(form OrderForm) error {
    if form.Quantity <= 0 {
        return errInvalidQuantity.New(errors.Context(
            errors.Field("quantity", form.Quantity),
        ))
    }
    return nil
}

func (s *OrderService) Create(ctx context.Context, form OrderForm) error {
    ectx := errors.Context(errors.Field("order_id", form.OrderID))
    if err := validateOrder(form); err != nil {
        return errors.Propagate(err, "invalid order", ectx) // caller adds its scope here
    }
    // ...
}
```

## 2. Context-Based Logging

### Rules

- **ALWAYS** use `logging.GetLoggerFromContext(ctx)` or `request.GetLogger(r)`
  in request-scoped code (handlers, services, repositories called during a
  request). The context logger carries request ID, session ID, and other tracing
  fields injected by middleware.
- **ONLY** use `logging.Global()` in non-request code: application startup,
  background workers, init functions.
- **NEVER** create logger instances manually (e.g., `zap.NewProduction()`).
- **ALWAYS** log errors with `errors.Zap(err)`, not `zap.Error(err)`.

### Lifecycle

```go
// Call once at application startup. Panics if called twice.
logging.Init(&logging.Config{
    Level:    "info",
    Encoding: "json",
    InitialFields: map[string]any{"service": "my-api"},
})

// Non-request code: use Global()
logging.Global().Info("application started")

// Request-scoped code: use context logger
func (s *Service) Do(ctx context.Context) error {
    l := logging.GetLoggerFromContext(ctx)
    l.Info("processing request")
    // ...
}

// Or from an http.Request directly:
func (a *API) Handle(w http.ResponseWriter, r *http.Request) error {
    l := request.GetLogger(r)
    l.Info("handling request")
    // ...
}
```

### Enriching the logger

Add fields for the current scope, then reuse the enriched logger:

```go
func (s *OrderService) Create(ctx context.Context, form OrderForm) (*Order, error) {
    ectx := errors.Context(
        errors.Field("user_id", form.UserID),
        errors.Field("product_id", form.ProductID),
    )
    l := logging.GetLoggerFromContext(ctx).With(ectx.Zap())
    l.Info("creating order")
    // ...
}
```

### Pushing enriched logger back to context

**Use with caution.** When a function enriches the logger and then calls
subfunctions that also use `logging.GetLoggerFromContext`, you can push the
enriched logger back into the context with `logging.SetContextLogger`. But
this should only be done when the enriched fields are truly meaningful for
every downstream log line (e.g., resource name/namespace at the top of a
reconciler). Garlic errors already carry the full context chain via
`errors.Context`, so on error paths you get all the scoped fields
automatically without enriching the logger. Prefer `errors.Context` for
scoped debugging data and reserve logger enrichment for top-level identifiers
that are useful on non-error log lines too.

```go
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    ectx := errors.Context(
        errors.Field("resource", req.Name),
        errors.Field("namespace", req.Namespace),
    )
    log := logging.GetLoggerFromContext(ctx).With(ectx.Zap())
    ctx = logging.SetContextLogger(ctx, log)

    // All downstream calls now get the enriched logger automatically.
    // Only do this at the top level where the fields matter for every log line.
    return r.doWork(ctx, req)
}
```

## 3. REST Handlers and Responses

### Handler signature

Garlic handlers return `error`. The route wrapper (in `rest/route.go`)
automatically logs the error and writes the appropriate HTTP response. Never
write error responses manually in handlers.

```go
// Signature: func(http.ResponseWriter, *http.Request) error
func (a *API) Create(w http.ResponseWriter, r *http.Request) error {
    // ...
    if err != nil {
        return errors.Propagate(err, "failed to create resource")
        // The route wrapper handles logging + WriteError
    }
    rest.WriteResponse(http.StatusCreated, resource).Must(w)
    return nil
}
```

### Route wrapper behavior

When a handler returns a non-nil error, the wrapper:
1. Logs with `errors.Zap(err)` (Warn for user errors, Error for system errors)
2. Calls `rest.WriteError(err).Must(w)`, the one canonical error writer, which
   projects the error through `errors.ErrorT.PublicDTO`:
   - User errors (4xx) are exposed in full — name, message, code, details/hints
     — with `Kind.StatusCode()` as the status.
   - System errors (5xx) keep their real HTTP status, but the body is redacted
     to a reference: the kind's code (so a client can quote it to support) plus
     the standard HTTP status text. The kind name, the static `Description`, the
     dynamic message, and details are all stripped, so the code identifies the
     failure without describing it.
   - `rest.WriteMessage` is for non-error informational responses only.

### Response helpers

```go
rest.WriteResponse(http.StatusOK, payload).Must(w)   // JSON with arbitrary payload
rest.WriteMessage(http.StatusCreated, "created").Must(w)  // {"message": "created"}
rest.WriteError(err).Must(w)  // auto status code from error kind
```

### App interface

Group related routes by implementing the `App` interface:

```go
type OrderAPI struct{ service *OrderService }

func (a *OrderAPI) Routes() rest.Routes {
    return rest.Routes{
        rest.Get("/v1/orders", a.List),
        rest.Post("/v1/orders", a.Create),
        rest.Get("/v1/orders/{order_id}", a.Read),
        rest.Delete("/v1/orders/{order_id}", a.Delete),
    }
}

// Register with the router
rest.RegisterApp(router, &OrderAPI{service})
```

### Server lifecycle

The server uses a multiton pattern. Options only apply on the first call:

```go
srv := rest.GetServer("api", rest.WithShutdownTimeout(10*time.Second))
// Subsequent calls to GetServer("api") return the same instance,
// ignoring any new options.
```

## 4. Middleware Stack

### Ordering

Middleware must be applied in this order. Tracing depends on Logging (it
retrieves the logger from context to enrich it with request/session IDs):

```go
router.Use(
    middleware.ContextCancel,    // 1. cancellable context
    middleware.Logging,          // 2. logger injected into context
    middleware.Tracing,          // 3. request/session IDs (needs logger)
    middleware.MetricsMonitor,   // 4. OpenTelemetry HTTP metrics
    middleware.ContentTypeJson,  // 5. JSON content type
    middleware.Cors(cfg),        // 6. CORS headers
)
```

- Use `middleware.Tracing` for edge services that generate request IDs.
- Use `middleware.PropagateTracing` for downstream services that receive IDs
  via `X-Request-ID` and `X-Session-ID` headers.
- **ALWAYS** register middleware BEFORE routes (`router.Use()` before
  `rest.RegisterApp()`).

### Route groups

Use Chi route groups to apply different middleware stacks:

```go
// Public: no logging, no auth
r.Group(func(r chi.Router) {
    r.Use(middleware.ContentTypeJson)
    rest.RegisterApp(r, healthAPI)
})

// API: full observability
r.Group(func(r chi.Router) {
    r.Use(
        middleware.Logging,
        middleware.PropagateTracing,
        middleware.MetricsMonitor,
        middleware.ContentTypeJson,
    )
    rest.RegisterApp(r, usersAPI)
})
```

### Metrics and observability

`middleware.MetricsMonitor` records HTTP metrics through OpenTelemetry, against
the global `MeterProvider`. It records three instruments under the meter
`github.com/luanguimaraesla/garlic`, using the OTEL HTTP semantic-convention
attribute keys:

| Instrument | Type | Attributes |
|------------|------|------------|
| `http.server.requests` | counter | `http.request.method`, `http.route`, `http.response.status_code` |
| `http.server.active_requests` | up/down counter | `http.request.method`, `http.route` |
| `http.server.request.duration` | histogram (seconds) | `http.request.method`, `http.route`, `http.response.status_code` |

`http.server.active_requests` and `http.server.request.duration` are HTTP
semantic-convention metrics; `http.server.requests` is a garlic-specific request
counter.

### Rules

- **ALWAYS** install a `MeterProvider` at startup, otherwise the global provider
  is a no-op and nothing is exported. The `observability` package is the
  recommended one-liner: it pushes metrics to an OTLP/gRPC collector.
- **ALWAYS** call `observability.Init` once, early, before serving traffic
  (it is fatal if called twice), and `observability.Shutdown(ctx)` on graceful
  shutdown so the final interval of metrics is flushed.
- **NEVER** expose a `/metrics` endpoint or wire `promhttp` / a Prometheus
  registry directly. Metrics are pushed to a collector, not scraped. There is
  no `observability.Handler`.
- **PREFER** configuring the exporter with the standard OTEL environment
  variables (`OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_INSECURE`,
  `OTEL_METRIC_EXPORT_INTERVAL`). Non-zero `observability.Config` fields
  (`Endpoint`, `Insecure`, `Interval`) override the matching variable.
- An application that already runs its own `MeterProvider` can skip
  `observability.Init` and install it via `otel.SetMeterProvider`; the
  middleware records into whatever provider is installed.

```go
// Startup: push garlic's HTTP metrics to the collector. With
// OTEL_EXPORTER_OTLP_ENDPOINT set in the environment, ServiceName is the only
// code-level configuration most deployments need.
observability.Init(&observability.Config{ServiceName: "my-api"})

// Graceful shutdown: flush the last export interval.
rest.GetServer("api", rest.WithOnShutdown(func(ctx context.Context) {
    _ = observability.Shutdown(ctx)
}))
```

The `/health` route is automatically excluded from both logging and metrics.

## 5. Database Transactions

### Rules

- **ALWAYS** use `storer.Transaction(ctx, fn)` for transactions. It handles
  commit and rollback automatically with rollback-first semantics (deferred
  rollback runs before error handling).
- **NEVER** manually call `Begin`/`Commit`/`Rollback`.
- **ALWAYS** use the `txCtx` returned by the transaction function for all
  queries inside the transaction. `Database.Executor(ctx)` automatically routes
  queries through the active transaction when one exists in context.
- `BeginContext` returns the existing transaction if one is already in context
  (no nesting, inner calls reuse the outer transaction).

```go
storer := database.NewStorer(db)

err := storer.Transaction(ctx, func(txCtx context.Context) error {
    if err := db.Create(txCtx, insertQuery, &order); err != nil {
        return err // automatic rollback
    }
    return db.Update(txCtx, updateQuery, order.ProductID, order.Quantity)
    // automatic commit on nil return
})
```

### Constraint handling

PostgreSQL constraint violations are automatically mapped:
- `23505` (UNIQUE violation) becomes `KindUserError`, "resource already exists"
- `23502` (NOT NULL violation) becomes `KindUserError`, "missing required field"
- Other DB errors become `KindSystemError`

## 6. Request Parsing

Use garlic's parsing helpers instead of manually reading Chi URL params or
decoding JSON. They return garlic errors with proper kinds and user-facing hints.

```go
// Path parameters
id, err := request.ParseResourceUUID(r, "order_id")
page, err := request.ParseResourceInt(r, "page")
slug, err := request.ParseResourceString(r, "slug")

// Query parameters
limit, start := request.ParseParamPagination(r)
userID, err := request.ParseParamUUID(r, "user_id")
active, err := request.ParseOptionalParamBool(r, "active")

// Body: decode + validate + convert to model in one call
var form CreateOrderForm
model, err := request.ParseForm[Order](r, &form)
```

The `Form` interface requires a `ToModel()` method:

```go
type CreateOrderForm struct {
    ProductID string `json:"product_id" validate:"required,uuid"`
    Quantity  int    `json:"quantity" validate:"required,gt=0"`
}

func (f *CreateOrderForm) ToModel() (Order, error) {
    return Order{
        ProductID: uuid.MustParse(f.ProductID),
        Quantity:  f.Quantity,
    }, nil
}
```

## 7. Validator

### Lifecycle

```go
// Call once at startup. Panics if called twice.
validator.Init(
    validator.NewValidation("is_positive", func(fl validator.Field) bool {
        return fl.Field().Int() > 0
    }),
)

// Access via singleton
v := validator.Global()
```

### Validation errors

**ALWAYS** convert go-playground/validator errors with `ParseValidationErrors`.
It returns `KindValidationError` with per-field hints in `Details["validation"]`,
using JSON tag names (not Go field names):

```go
if err := validator.Global().Struct(form); err != nil {
    return validator.ParseValidationErrors(err)
}
```

## 8. Testing and Build Tags

- **ALL** unit tests must have the `//go:build unit` tag. Without it, `make
  test` (which passes `-tags=unit`) will skip them.
- Use the `test` package's builder pattern for HTTP handler tests:

```go
//go:build unit

package mypackage

func TestCreateOrder(t *testing.T) {
    tc := test.New(t).
        WithMethod(http.MethodPost).
        WithURL("/v1/orders").
        WithBody(createOrderForm).
        WithURLParams(map[string]string{"org_id": orgID.String()})

    tc.Run(api.Create, func(res *http.Response) {
        assert.Equal(t, http.StatusCreated, res.StatusCode)
    })
}
```

## 9. Outbound HTTP (httpclient)

### Rules

- **ALWAYS** build one shared `httpclient.New(config)` client per upstream at
  startup and reuse it — the transport is pooled. **NEVER** build a client per
  call.
- **ALWAYS** fork per call with `conn.R(ctx)`, passing the request-scoped `ctx`
  so tracing headers and cancellation propagate. The context reaches the
  transport, so a `context.WithTimeout` actually applies.
- **ALWAYS** depend on the `httpclient.Requester` interface in services, and
  inject `httpclient.NewRequesterMock()` in `//go:build unit` tests. **NEVER**
  stand up an `httptest` server in a service test just to fake one call.
- **ALWAYS** treat outbound errors with `errors.IsKind`; the typed
  `*httpclient.ResponseError` (via `errors.As`) carries `StatusCode()`,
  `RetryAfter()`, and selected headers, and is panic-free for any body shape.
- **ALWAYS** reclassify an upstream error with `errors.PropagateAs(...)` before
  re-emitting it if its kind should not surface to your own client (a decoded
  upstream 404 is a user-class error by default and would otherwise be exposed).
- **NEVER** rely on a `POST` being retried — retry is idempotency-gated by
  default. Use `EnableRetry()` for a `POST` you know is safe to replay, or pass a
  custom `RetryPolicy`.
- **NEVER** log tokens or set `Authorization` from a logged value; pass auth via
  a `TokenSource` (`StaticToken`, `FileTokenSource`), which is read fresh per
  attempt.

### Example

```go
conn, err := httpclient.New(&httpclient.Config{
    BaseURL:     "http://orders:8080",
    TokenSource: httpclient.FileTokenSource("/var/run/secrets/token"),
})

var order OrderDTO
resp, err := conn.R(ctx).SetResult(&order).Get("/v1/orders/" + id)
if err != nil {
    var re *httpclient.ResponseError
    if errors.As(err, &re) && re.StatusCode() == http.StatusTooManyRequests {
        if d, ok := re.RetryAfter(); ok { time.Sleep(d) }
    }
    return errors.Propagate(err, "failed to fetch order")
}
```

Compose OpenTelemetry by wrapping `Config.Transport` with
`otelhttp.NewTransport(base)`; the connector does not import `otelhttp`.

## Quick Reference

| Area | ALWAYS | NEVER |
|------|--------|-------|
| Errors | `errors.Propagate(err, msg)` at every boundary | Bare `return err` or `fmt.Errorf` wrapping |
| Errors | Propagate before logging | Log raw unpropagated errors |
| Errors | `errors.Hint(msg)` on user-level errors | Omit hints on `KindUserError` and subkinds |
| Errors | `errors.Zap(err)` for logging | `zap.Error(err)` |
| Errors | `errors.IsKind(err, kind)` for checks | Type assertions on `*ErrorT` |
| Errors | Build `errors.Context(...)` at function scope | Pass `*errors.ContextT` as a parameter |
| Logging | `logging.GetLoggerFromContext(ctx)` in requests | `logging.Global()` in request-scoped code |
| Logging | `logging.Init()` once at startup | Calling `Init()` twice (panics) |
| Logging | Logger enrichment only for top-level identifiers | Enriching logger for every scope (use `errors.Context` instead) |
| Handlers | Return `error` from handlers | Write error responses manually |
| Handlers | `rest.WriteResponse(status, payload).Must(w)` | Forget `.Must(w)` |
| Middleware | Apply in order: Cancel, Logging, Tracing, ... | Tracing before Logging |
| Middleware | Register before routes | `router.Use()` after `RegisterApp()` |
| Metrics | `observability.Init(...)` once at startup, `Shutdown` on exit | A `/metrics` endpoint or `promhttp` / Prometheus registry |
| Transactions | `storer.Transaction(ctx, fn)` | Manual `Begin`/`Commit`/`Rollback` |
| Transactions | Use `txCtx` for all queries in transaction | Base context inside transaction |
| Parsing | `request.ParseResourceUUID(r, param)` | Manual `chi.URLParam()` parsing |
| Parsing | `request.ParseForm[T](r, &form)` | Manual JSON decode without validation |
| Validator | `validator.ParseValidationErrors(err)` | Raw go-playground errors in responses |
| HTTP client | One shared `httpclient.New(config)`, fork with `conn.R(ctx)` | A new client per call |
| HTTP client | Depend on `httpclient.Requester`, inject `RequesterMock` in tests | An `httptest` server in service tests |
| HTTP client | `errors.As(err, &re)` for status/Retry-After, `errors.IsKind` to classify | Assuming a body shape or that `POST` retries |
| HTTP client | Auth via a `TokenSource` | Logging tokens or hardcoding `Authorization` |
| Tests | `//go:build unit` on all unit tests | Omitting the build tag |
