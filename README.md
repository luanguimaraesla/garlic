# Garlic

[![CI](https://github.com/luanguimaraesla/garlic/actions/workflows/ci.yml/badge.svg)](https://github.com/luanguimaraesla/garlic/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/luanguimaraesla/garlic.svg)](https://pkg.go.dev/github.com/luanguimaraesla/garlic)
[![Go Report Card](https://goreportcard.com/badge/github.com/luanguimaraesla/garlic)](https://goreportcard.com/report/github.com/luanguimaraesla/garlic)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](./LICENSE)

Garlic is a small Go framework for building HTTP services. It handles the
boring parts so each service can focus on business logic: routing, structured
errors, logging, metrics, tracing, request parsing, database helpers, HTTP
clients, and workers.

Garlic is a library, not an application. Import only the packages you need.

## Installation

```bash
go get github.com/luanguimaraesla/garlic
```

## Quick start

```go
package main

import (
    "context"
    "net/http"
    "os/signal"
    "syscall"

    chi "github.com/go-chi/chi/v5"

    "github.com/luanguimaraesla/garlic/logging"
    "github.com/luanguimaraesla/garlic/middleware"
    "github.com/luanguimaraesla/garlic/observability"
    "github.com/luanguimaraesla/garlic/rest"
)

func main() {
    logging.Init(&logging.Config{
        Level:    "info",
        Encoding: "json",
        InitialFields: map[string]any{
            "service": "my-api",
        },
    })

    observability.Init(&observability.Config{ServiceName: "my-api"})

    server := rest.GetServer("api", rest.WithOnShutdown(func(ctx context.Context) {
        _ = observability.Shutdown(ctx)
    }))

    r := server.Router()

    r.Group(func(r chi.Router) {
        r.Use(
            middleware.Logging,
            middleware.PropagateTracing,
            middleware.MetricsMonitor,
            middleware.ContentTypeJson,
        )

        rest.RegisterApp(r, &HealthApp{})
    })

    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer stop()

    errc := server.Listen(ctx, ":8080")
    if err := <-errc; err != nil {
        logging.Global().Fatal(err.Error())
    }
}

type HealthApp struct{}

func (a *HealthApp) Routes() rest.Routes {
    return rest.Routes{
        rest.Get("/health", func(w http.ResponseWriter, r *http.Request) error {
            rest.WriteMessage(http.StatusOK, "ok").Must(w)
            return nil
        }),
    }
}
```

## Packages

| Package | What it gives you |
|---------|-------------------|
| [`errors`](./errors) | Rich errors with kind hierarchy, context propagation, and HTTP status mapping |
| [`rest`](./rest) | Chi-based HTTP server with error-aware handlers and JSON response helpers |
| [`middleware`](./middleware) | HTTP middleware for logging, tracing, OpenTelemetry metrics, CORS, and content type |
| [`request`](./request) | Helpers for path params, query strings, JSON bodies, validation, and model conversion |
| [`database`](./database) | PostgreSQL helpers for CRUD, transactions, filtering, and mocks |
| [`database/utils`](./database/utils) | Named query helpers, patch bindings, and PostgreSQL type converters |
| [`logging`](./logging) | Singleton Zap logger with context integration |
| [`logging/keyvals`](./logging/keyvals) | Adapter for keyvals-style logger interfaces, such as Temporal SDK, go-kit log, and log15 |
| [`validator`](./validator) | Singleton go-playground/validator setup with custom field validators |
| [`monitoring`](./monitoring) | OpenTelemetry HTTP request metrics |
| [`observability`](./observability) | OpenTelemetry MeterProvider setup that exports metrics over OTLP/gRPC |
| [`tracing`](./tracing) | Request and session ID propagation through context and headers |
| [`httpclient`](./httpclient) | Pooled HTTP client with auth, streaming, typed errors, tracing, and idempotency-aware retry |
| [`crypto`](./crypto) | AES-256-GCM authenticated encryption and SHA-256 hashing |
| [`worker`](./worker) | Goroutine pool for background tasks |
| [`toolkit`](./toolkit) | Generic pointer and nil-checking helpers |
| [`test`](./test) | Builder-pattern HTTP test helpers |
| [`global`](./global) | Application metadata, such as version information |
| [`utils`](./utils) | Struct flattening with `mapstructure` tags |
| [`debug`](./debug) | Pretty-printing and debugger helpers |

## How the pieces fit together

```text
HTTP request
    │
    ▼
  rest ─────────────┐
    │               │
    ▼               ▼
middleware       request
    │               │
    ▼               ▼
 logging         validator
 tracing
 monitoring

database ─┐
httpclient├── errors
worker ───┘
```

Most packages use `errors` so failures carry a kind, context fields, stack
traces, and an HTTP status. `logging` and `tracing` store request-scoped values
in `context.Context`, and middleware makes those values available to handlers.

## Error handling

The `errors` package is the center of Garlic. Every Garlic error has a **kind**.
Kinds form a hierarchy, and that hierarchy maps errors to HTTP status codes.

That means handlers can return errors instead of choosing status codes by hand.
The `rest` package logs the error, serializes it to JSON, and writes the right
response. User errors are returned to clients. System errors are sanitized so
internal details stay out of the API response.

### Kind hierarchy

Kinds use three levels:

- **Primitive kinds (`P`)** are the root categories.
- **Secondary kinds (`S`)** map to standard HTTP status codes. They are named
  `HTTP<status>Error`, such as `HTTP404Error`, with codes like `S00404`.
- **Tertiary kinds (`C`)** are framework or application-specific kinds. Garlic
  packages register their own tertiary kinds under the right parent.

```text
KindError (P00000)
├── KindUserError (P00001, 400)
│   └── HTTP4xxError (S004xx)            // one per 4xx status
│       ├── KindInvalidRequestError (C00001, 400 ← S00400)
│       ├── KindAuthError (C00003, 401 ← S00401)
│       ├── KindForbiddenError (C00004, 403 ← S00403)
│       └── KindNotFoundError (C00005, 404 ← S00404)
└── KindSystemError (P00002, 500)
    └── HTTP3xxError / HTTP5xxError (S00xxx)  // one per non-4xx status
```

The `errors` package owns the primitive kinds, the HTTP secondary kinds, and a
small set of generic tertiary kinds. Domain packages add their own. For example,
`validator` registers `ValidationError`, `tracing` registers context-related
errors, and `database` registers record-not-found and transaction errors.

Use `errors.IsKind(err, errors.KindUserError)` to match any client-side error.
User errors are exposed in full. System errors are genericized by
`ErrorT.PublicDTO` to their HTTP status: only a per-status code and the standard
status text cross the wire, so the specific kind, its dynamic message, and its
details never leak.

`errors.KindForStatus(status)` returns the secondary kind for a status code.
Garlic registers one for every standard HTTP status during initialization. A
non-standard status falls back to its status class. The `P`, `S`, and `C` code
prefixes are reserved by Garlic, so custom kinds should use another prefix.

### Custom error kinds

Define application-specific kinds by setting a parent in the hierarchy. Register
them in an `init()` function, then add a blank import in `main.go` so
registration always runs.

```go
// myapp/errors/errors.go
package apperrors

import (
    "net/http"

    "github.com/luanguimaraesla/garlic/errors"
)

var KindPaymentDeclinedError = &errors.Kind{
    Name:           "PaymentDeclinedError",
    Code:           "PAY001",
    Description:    "The payment provider declined the transaction",
    HTTPStatusCode: http.StatusConflict,
    Parent:         errors.KindUserError,
}

func init() {
    errors.Register(KindPaymentDeclinedError)
}
```

Other packages can fetch registered kinds by name:

```go
var KindPaymentDeclinedError = errors.Get("PaymentDeclinedError")
```

### Error propagation

Wrap errors with `errors.Propagate` as they move up the stack. Garlic keeps the
original kind, so a repository `KindNotFoundError` still becomes a `404` when it
reaches the HTTP handler.

```go
// Repository layer
func (r *UserRepo) FindByID(ctx context.Context, id uuid.UUID) (*User, error) {
    var user User
    err := r.db.Read(ctx, "SELECT * FROM users WHERE id = $1", &user, id)
    if err != nil {
        return nil, errors.Propagate(err, "failed to read user from database")
    }

    return &user, nil
}

// Service layer
func (s *UserService) Get(ctx context.Context, id uuid.UUID) (*User, error) {
    user, err := s.repo.FindByID(ctx, id)
    if err != nil {
        return nil, errors.Propagate(err, "failed to get user")
    }

    return user, nil
}

// Handler layer: return errors, do not write failure responses directly.
func (api *UserAPI) Read(w http.ResponseWriter, r *http.Request) error {
    id, err := request.ParseResourceUUID(r, "user_id")
    if err != nil {
        return err
    }

    user, err := api.service.Get(r.Context(), id)
    if err != nil {
        return errors.Propagate(err, "failed to read user")
    }

    rest.WriteResponse(http.StatusOK, user).Must(w)
    return nil
}
```

### Error context

Build an error context once at the start of a service method. Reuse it for
structured logs and error propagation. Context fields appear in logs, but they
are never returned to API clients.

```go
func (s *OrderService) Create(ctx context.Context, form OrderForm) (*Order, error) {
    ectx := errors.Context(
        errors.Field("user_id", form.UserID),
        errors.Field("product_id", form.ProductID),
        errors.Field("quantity", form.Quantity),
    )

    logger := logging.GetLoggerFromContext(ctx).With(ectx.Zap())
    logger.Info("Creating order")

    order, err := s.repo.Create(ctx, form.ToModel())
    if err != nil {
        return nil, errors.Propagate(err, "failed to create order", ectx)
    }

    return order, nil
}
```

Sensitive values can be partially redacted:

```go
ectx := errors.Context(
    errors.RedactedString("api_key", "sk_live_abc123def456"),
)
// logs as: "sk****f456"
```

### Error templates

Templates are reusable error factories. They define the kind, message, and
options once. Use `New` to create a fresh error or `Propagate` to wrap an
existing one.

```go
var notFoundTemplate = errors.Template(
    errors.KindNotFoundError,
    "resource not found",
    errors.Hint("Check if this resource exists or the ID is correct."),
)

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Resource, error) {
    ectx := errors.Context(errors.Field("resource_id", id))

    resource, err := s.repo.FindByID(ctx, id)
    if err != nil {
        return nil, notFoundTemplate.Propagate(err, ectx)
    }

    return resource, nil
}
```

### Inspecting errors

`IsKind` walks the whole error chain and follows the kind hierarchy. Checking a
parent kind also matches its children.

```go
if errors.IsKind(err, errors.KindUserError) {
    // Matches validation, not-found, auth, forbidden, and other user errors.
}
```

`AsKind` returns the first matching Garlic error in the chain.

```go
if e, ok := errors.AsKind(err, errors.KindNotFoundError); ok {
    log.Println(e.Details)
}
```

### Structured logging

`errors.Zap(err)` returns a `zap.Field` with the error context, troubleshooting
data, and stack trace.

```go
logger.Error("Failed to process payment", errors.Zap(err))
```

### JSON serialization

Errors convert to response DTOs. System errors are sanitized automatically.

```go
dto := err.ErrorDTO()
// {
//   "name": "ValidationError::InvalidRequestError::HTTP400Error::UserError::Error",
//   "kind": "C00002",
//   "error": "email is invalid",
//   "details": {"hint": "provide a valid email address"}
// }
```

## Middleware

The `middleware` package provides Chi-compatible middleware.

| Middleware | Purpose |
|------------|---------|
| `ContextCancel` | Creates a cancellable child context for each request |
| `Logging` | Adds a Zap logger to the request context and logs method, URL, status, size, and duration |
| `Tracing` | Generates a request ID, sets `X-Request-ID`, and stores it in context |
| `PropagateTracing` | Reads `X-Request-ID` and `X-Session-ID` from incoming headers |
| `MetricsMonitor` | Records OpenTelemetry request count, active request count, and duration metrics |
| `ContentTypeJson` | Sets `Content-Type: application/json` on every response |
| `Cors` | Sets CORS headers and handles `OPTIONS` preflight requests |

The `/health` endpoint is excluded from logging and metrics by default.

### Middleware stack patterns

Apply middleware per route group. This keeps public routes simple and gives API
routes the full observability stack.

```go
server := rest.GetServer("api")
r := server.Router()

// Public routes: health checks and docs.
r.Group(func(r chi.Router) {
    r.Use(middleware.ContentTypeJson)
    rest.RegisterApp(r, healthAPI)
})

// API routes: logging, tracing, metrics, and JSON responses.
r.Group(func(r chi.Router) {
    r.Use(
        middleware.Logging,
        middleware.PropagateTracing,
        middleware.MetricsMonitor,
        middleware.ContentTypeJson,
    )

    rest.RegisterApp(r, usersAPI)
    rest.RegisterApp(r, ordersAPI)
})
```

Use `Tracing` at the edge of the system to create request IDs. Use
`PropagateTracing` in downstream services that receive IDs through headers.

### CORS

```go
cfg := &middleware.Config{
    Cors: &middleware.CorsConfig{
        AllowedHosts:   []string{"https://app.example.com"},
        AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
        AllowedHeaders: []string{"Content-Type", "Authorization"},
        ExposedHeaders: []string{"X-Request-ID", "X-Session-ID"},
    },
}

r.Use(middleware.Cors(cfg))
```

## Metrics and observability

`middleware.MetricsMonitor` records HTTP metrics through the global OpenTelemetry
`MeterProvider`. The default provider is a no-op, so nothing is exported until
the application installs one.

The easiest setup is `observability.Init`, which exports metrics to an
OpenTelemetry collector over OTLP/gRPC.

```go
observability.Init(&observability.Config{ServiceName: "my-api"})

server := rest.GetServer("api", rest.WithOnShutdown(func(ctx context.Context) {
    _ = observability.Shutdown(ctx)
}))
```

The exporter reads the standard OpenTelemetry environment variables. In a
collector-sidecar deployment, the service name may be the only code-level
setting you need.

```sh
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4317
```

Non-zero `observability.Config` fields (`Endpoint`, `Insecure`, `Interval`)
override their matching environment variables. If your application already
installs its own `MeterProvider`, skip `observability.Init` and call
`otel.SetMeterProvider` yourself. Garlic records into whichever provider is
installed.

### Instruments

The middleware records metrics under the meter
`github.com/luanguimaraesla/garlic` and uses OpenTelemetry HTTP semantic
convention attributes.

| Instrument | Type | Attributes |
|------------|------|------------|
| `http.server.requests` | counter | `http.request.method`, `http.route`, `http.response.status_code` |
| `http.server.active_requests` | up/down counter | `http.request.method`, `http.route` |
| `http.server.request.duration` | histogram in seconds | `http.request.method`, `http.route`, `http.response.status_code` |

`http.server.active_requests` and `http.server.request.duration` are standard
semantic-convention metrics. `http.server.requests` is Garlic's request counter;
its total can also be derived from the histogram count.

### Migrating from the Prometheus backend

Garlic no longer uses `prometheus/client_golang` directly. This is a breaking
change.

- `monitoring.TrafficMetric`, `monitoring.ActiveRequests`, and
  `monitoring.LatencyMetric` were removed.
- Garlic no longer registers metrics in the default Prometheus registry.
- Garlic no longer exposes a `/metrics` endpoint. Metrics are pushed to an OTLP
  collector instead.
- Replace `promhttp.Handler()` routes with `observability.Init(...)`, or install
  your own Prometheus-exporting `MeterProvider` if you still need a scrape
  endpoint.

## Routes and handlers

Garlic handlers return `error`. They write successful responses directly, but
they return failures and let `rest` handle logging and status codes.

```go
type OrderAPI struct {
    service *OrderService
}

func (api *OrderAPI) Routes() rest.Routes {
    return rest.Routes{
        rest.Get("/v1/orders", api.List),
        rest.Post("/v1/orders", api.Create),
        rest.Get("/v1/orders/{order_id}", api.Read),
        rest.Delete("/v1/orders/{order_id}", api.Delete),
    }
}

func (api *OrderAPI) Create(w http.ResponseWriter, r *http.Request) error {
    var form CreateOrderForm
    model, err := request.ParseForm[Order](r, &form)
    if err != nil {
        return err
    }

    order, err := api.service.Create(r.Context(), model)
    if err != nil {
        return errors.Propagate(err, "failed to create order")
    }

    rest.WriteResponse(http.StatusCreated, order).Must(w)
    return nil
}
```

## Request parsing

```go
// Path parameters
id, err := request.ParseResourceUUID(r, "order_id")
page, err := request.ParseResourceInt(r, "page")
slug, err := request.ParseResourceString(r, "slug")

// Query parameters
limit, start := request.ParseParamPagination(r)
active, err := request.ParseOptionalParamBool(r, "active")
name, err := request.ParseParamString(r, "name")

// Body decoding, validation, and model conversion
var form CreateOrderForm
model, err := request.ParseForm[Order](r, &form)
```

## Database transactions

```go
storer := database.NewStorer(db)

err := storer.Transaction(ctx, func(txCtx context.Context) error {
    if err := db.Create(txCtx, insertQuery, &order); err != nil {
        return err // rolls back
    }

    return db.Update(txCtx, updateInventoryQuery, order.ProductID, order.Quantity)
    // commits on success
})
```

## Custom validation

Register custom validators at startup, then use them in struct tags.

```go
validator.Init(
    validator.NewValidation("is_git_url", func(fl validator.Field) bool {
        value := fl.Field().String()
        return strings.HasPrefix(value, "https://") && strings.HasSuffix(value, ".git")
    }),
)

type CreateRepoForm struct {
    URL    string `json:"url" validate:"required,is_git_url"`
    Branch string `json:"branch" validate:"required"`
}
```

## Inter-service communication

The `httpclient` package gives you a pooled HTTP client. Create one client per
upstream service at startup, then fork per-call requests with `conn.R(ctx)`. Each
request inherits the client defaults and propagates `X-Request-ID` and
`X-Session-ID` from the context.

```go
conn, err := httpclient.New(&httpclient.Config{
    BaseURL:     "http://order-service:8080",
    TokenSource: httpclient.FileTokenSource("/var/run/secrets/token"),
})

var order OrderDTO
resp, err := conn.R(ctx).
    SetQueryParam("include", "items").
    SetResult(&order).
    Get("/v1/orders/" + orderID)
```

Non-2xx responses return a typed `*httpclient.ResponseError`. It preserves the
HTTP status, the `Retry-After` hint, and selected headers. It is safe for any
response body shape, still matches `errors.IsKind`, and can be inspected with
the standard library's `errors.As`.

```go
var responseErr *httpclient.ResponseError
if errors.As(err, &responseErr) && responseErr.StatusCode() == http.StatusTooManyRequests {
    if delay, ok := responseErr.RetryAfter(); ok {
        time.Sleep(delay)
    }
}
```

The client also supports streaming uploads with explicit `Content-Length`, raw
streaming downloads through `SetDoNotParseResponse`, pluggable auth through a
`TokenSource`, custom transports, `http.RoundTripper`, before/after hooks, and
idempotency-aware retry. Retry is enabled for `GET`, `HEAD`, `OPTIONS`, `PUT`,
and `DELETE` by default. Use `EnableRetry` to opt a `POST` request in.

To add OpenTelemetry, wrap `Config.Transport`. The connector does not import
`otelhttp` directly.

In unit tests, inject `httpclient.NewRequesterMock()` anywhere your code depends
on `httpclient.Requester`. You do not need to start a test HTTP server.

> **Migration from the old `Connector` API:** `NewConnector`,
> `Connector.Request`, the `Request{Method, URI, Data, QueryParams}` struct, and
> the package-level `Get`/`Post`/`Put`/`Patch`/`Delete` functions were removed.
> Replace `conn.Request(ctx, &Request{Method: http.MethodGet, URI: path}, &out)`
> with `conn.R(ctx).SetResult(&out).Get(path)`. Construct the client with
> `httpclient.New(&Config{BaseURL: ...})`. The old `Config.URL` field is now
> `Config.BaseURL`, still mapped from the `url` config key. Request contexts now
> reach the transport, so deadlines and cancellation work as expected. Retry is
> also limited to idempotent methods unless you opt in.

## Third-party logger interfaces

Some Go libraries expect a keyvals-style logger with methods like
`Debug/Info/Warn/Error(msg string, kv ...any)` and `With(kv ...any)`. Temporal
SDK, go-kit log, and log15 all use this shape. The `logging/keyvals` package
adapts a Garlic logger to that interface so each service does not need its own
shim.

```go
import (
    "go.temporal.io/sdk/client"

    "github.com/luanguimaraesla/garlic/logging"
    "github.com/luanguimaraesla/garlic/logging/keyvals"
)

c, err := client.Dial(client.Options{
    HostPort: "temporal:7233",
    Logger:   keyvals.NewLogger(logging.Global()),
})
```

The package is named after the interface shape, not after a specific library.
Garlic does not import those target SDKs, so adding `logging/keyvals` does not
pull Temporal or any other SDK into your dependency graph. The compile-time
interface check happens in your project when you pass the adapter to the target
library.

`NewLogger(nil)` falls back to `logging.Global()`. `With` returns a new adapter,
so chained calls keep satisfying the same interface.

## AI agent guidance

Garlic ships a [Claude Code](https://claude.ai/code) skill that teaches agents
the framework conventions: error propagation, context-based logging, middleware
ordering, and the rest of Garlic's patterns. Projects that depend on Garlic can
install it with [skills](https://github.com/vercel-labs/skills):

```bash
npx skills add luanguimaraesla/garlic -s garlic-conventions
```

The skill activates when Claude Code detects Garlic imports in a project. Run
`npx skills check` after updating Garlic so agents pick up the latest guidance.

## Development

```bash
make test                        # Run unit tests with coverage
make GOTESTRUN=TestName test     # Run a specific unit test
make lint                        # Run golangci-lint
make fix                         # Run goimports, go mod tidy, and vendoring
make cover                       # Show the text coverage report
make cover/html                  # Open the HTML coverage report
```
