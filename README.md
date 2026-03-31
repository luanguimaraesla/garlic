# Garlic

Garlic is a Go framework that provides the essential building blocks for developing microservices. It covers the most common concerns -- structured logging, HTTP routing, request parsing, error handling, database access, metrics, tracing, and background workers -- so that teams can focus on business logic instead of reinventing infrastructure plumbing for every new service.

Garlic is consumed as a library (no main entry point). Import only the packages you need.

## Installation

```bash
go get github.com/luanguimaraesla/garlic
```

## Quick Start

```go
package main

import (
    "context"
    "net/http"

    chi "github.com/go-chi/chi/v5"
    "github.com/prometheus/client_golang/prometheus/promhttp"

    "github.com/luanguimaraesla/garlic/logging"
    "github.com/luanguimaraesla/garlic/middleware"
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

    server := rest.GetServer("api")
    r := server.Router()

    // Public routes (no auth, no logging)
    r.Group(func(r chi.Router) {
        r.Use(middleware.ContentTypeJson)
        r.Handle("/metrics", promhttp.Handler())
    })

    // Protected routes (full middleware stack)
    r.Group(func(r chi.Router) {
        r.Use(
            middleware.Logging,
            middleware.PropagateTracing,
            middleware.MetricsMonitor,
            middleware.ContentTypeJson,
        )
        rest.RegisterApp(r, &HealthApp{})
    })

    errc := server.Listen(context.Background(), ":8080")
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

| Package | Description |
|---------|-------------|
| [`errors`](./errors) | Rich error type with kind hierarchy, context propagation, and HTTP status mapping |
| [`rest`](./rest) | Chi-based HTTP server with error-aware handlers and JSON response helpers |
| [`middleware`](./middleware) | HTTP middleware: logging, tracing, Prometheus metrics, CORS, content-type |
| [`request`](./request) | Request parsing helpers for path params, query strings, and JSON bodies |
| [`database`](./database) | PostgreSQL abstraction with CRUD, transactions, filtering, and mocking |
| [`database/utils`](./database/utils) | Named query helpers, patch bindings, and PostgreSQL type converters |
| [`logging`](./logging) | Singleton Zap-based structured logger with context integration |
| [`validator`](./validator) | Singleton go-playground/validator with custom field validators |
| [`monitoring`](./monitoring) | Prometheus metrics for HTTP request tracking |
| [`tracing`](./tracing) | Request and session ID context propagation |
| [`httpclient`](./httpclient) | HTTP client with exponential backoff retry and distributed tracing |
| [`crypto`](./crypto) | AES-256-GCM authenticated encryption and SHA-256 hashing |
| [`worker`](./worker) | Goroutine pool for background task execution |
| [`toolkit`](./toolkit) | Generic pointer and nil-checking utilities |
| [`test`](./test) | Builder-pattern HTTP test case utilities |
| [`global`](./global) | Application-level metadata (version) |
| [`utils`](./utils) | Struct flattening with mapstructure tags |
| [`debug`](./debug) | Pretty-print and debugger breakpoint utilities |

## Architecture

```
                          ┌────────────┐
                          │   rest     │
                          │  (server)  │
                          └─────┬──────┘
                                │
               ┌────────────────┼────────────────┐
               │                │                │
        ┌──────▼──────┐  ┌─────▼──────┐  ┌──────▼──────┐
        │  middleware  │  │  request   │  │  monitoring │
        └──────┬──────┘  └─────┬──────┘  └─────────────┘
               │               │
     ┌─────────┼─────────┐     │
     │         │         │     │
┌────▼───┐ ┌──▼────┐ ┌──▼─────▼──┐
│logging │ │tracing│ │ validator │
└────────┘ └───────┘ └───────────┘

        ┌──────────┐    ┌────────────┐
        │ database │    │ httpclient │
        └────┬─────┘    └──────┬─────┘
             │                 │
             └────────┬────────┘
                      │
                ┌─────▼─────┐
                │  errors   │
                └───────────┘
```

All packages depend on `errors` for structured error handling. The `logging` and `tracing` packages provide context values consumed by middleware and propagated through request handlers.

## Error Handling

The `errors` package is the foundation of the framework. Every error carries a **kind** that classifies it. Kinds form a hierarchy that maps automatically to HTTP status codes, so handlers never need to set status codes manually. When a handler returns an error, the `rest` package logs it, converts it to a JSON DTO, and responds with the correct status code. System errors are sanitized so internal details are never leaked to clients.

### Kind Hierarchy

```
KindError (base, 500)
├── KindUserError (400)
│   ├── KindInvalidRequestError (400)
│   │   └── KindValidationError (400)
│   ├── KindNotFoundError (404)
│   │   └── KindDatabaseRecordNotFoundError (404)
│   ├── KindAuthError (401)
│   └── KindForbiddenError (403)
└── KindSystemError (500)
    ├── KindContextError
    │   └── KindContextValueNotFoundError
    └── KindDatabaseTransactionError (500)
```

### Custom Error Kinds

Define domain-specific kinds by setting a parent in the hierarchy. Register them in an `init()` function and use a blank import (`_ "myapp/errors"`) in `main.go` to ensure registration:

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

Other packages retrieve registered kinds by name:

```go
var KindPaymentDeclinedError = errors.Get("PaymentDeclinedError")
```

### Error Propagation

The typical pattern is: handlers call services, services call repositories, and each layer wraps errors with `Propagate` as they bubble up. The original error kind is preserved through the chain.

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

// Handler layer -- errors are returned, never written directly
func (api *UserAPI) Read(w http.ResponseWriter, r *http.Request) error {
    id, err := request.ParseResourceUUID(r, "user_id")
    if err != nil {
        return err // KindInvalidRequestError -> 400
    }

    user, err := api.service.Get(r.Context(), id)
    if err != nil {
        return errors.Propagate(err, "failed to read user")
        // if original was KindNotFoundError -> 404
        // if original was KindSystemError -> 500 (sanitized)
    }

    rest.WriteResponse(http.StatusOK, user).Must(w)
    return nil
}
```

### Error Context

Build an error context once at the beginning of a service method and reuse it for both structured logging and error wrapping. Context fields are included in logs but never exposed to API consumers.

```go
func (s *OrderService) Create(ctx context.Context, form OrderForm) (*Order, error) {
    ectx := errors.Context(
        errors.Field("user_id", form.UserID),
        errors.Field("product_id", form.ProductID),
        errors.Field("quantity", form.Quantity),
    )
    l := logging.GetLoggerFromContext(ctx).With(ectx.Zap())

    l.Info("Creating order")

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

### Error Templates

Templates define reusable error factories with a predefined kind, message, and options. Use `New` to create a fresh error or `Propagate` to wrap an existing one:

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
        // Wrap an existing error with template kind and message
        return nil, notFoundTemplate.Propagate(err, ectx)
    }

    return resource, nil
}
```

### Inspecting Errors

`IsKind` walks the entire error chain and traverses the kind hierarchy, so checking for a parent kind matches any of its children:

```go
if errors.IsKind(err, errors.KindUserError) {
    // matches ValidationError, NotFoundError, AuthError, ForbiddenError, etc.
}
```

`AsKind` retrieves the first matching error in the chain for inspection:

```go
if e, ok := errors.AsKind(err, errors.KindNotFoundError); ok {
    log.Println(e.Details)
}
```

### Structured Logging

`errors.Zap(err)` produces a `zap.Field` that includes the full error context, troubleshooting data, and stack traces:

```go
l.Error("Failed to process payment", errors.Zap(err))
```

### JSON Serialization

Errors convert to JSON DTOs for API responses. System errors are sanitized automatically:

```go
dto := err.ErrorDTO()
// {
//   "name": "ValidationError::InvalidRequestError::UserError::Error",
//   "kind": "E00004",
//   "error": "email is invalid",
//   "details": {"hint": "provide a valid email address"}
// }
```

## Middleware

The `middleware` package provides HTTP middleware compatible with Chi's `Use` method.

### Available Middleware

| Middleware | Purpose |
|------------|---------|
| `ContextCancel` | Creates a cancellable child context for each request, ensuring resource cleanup |
| `Logging` | Injects a structured Zap logger into the request context and logs method, URL, status code, response size, and duration |
| `Tracing` | Generates a UUID request ID, sets `X-Request-ID` in the response, and stores it in context |
| `PropagateTracing` | Reads `X-Request-ID` and `X-Session-ID` from incoming headers for downstream services |
| `MetricsMonitor` | Records Prometheus metrics: `http_request_total` (counter), `http_active_requests` (gauge), `http_request_duration_seconds` (histogram) |
| `ContentTypeJson` | Sets `Content-Type: application/json` on every response |
| `Cors` | Sets CORS headers from a config struct and handles `OPTIONS` preflight requests |

The `/health` endpoint is automatically excluded from logging and metrics.

### Middleware Stack Patterns

Apply middleware per route group to control which routes get logging, auth, or metrics:

```go
server := rest.GetServer("api")
r := server.Router()

// Public routes: health checks, metrics, docs
r.Group(func(r chi.Router) {
    r.Use(middleware.ContentTypeJson)
    r.Handle("/metrics", promhttp.Handler())
    rest.RegisterApp(r, healthAPI)
})

// API routes: full observability stack
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

Use `Tracing` for edge services that generate request IDs, and `PropagateTracing` for downstream services that receive them via headers.

### CORS Configuration

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

## Routes and Handlers

Handlers return `error` instead of writing failure responses directly. The `rest` package catches returned errors, logs them, and responds with the appropriate HTTP status code based on the error kind.

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

## Request Parsing

```go
// Path parameters
id, err := request.ParseResourceUUID(r, "order_id")
page, err := request.ParseResourceInt(r, "page")
slug, err := request.ParseResourceString(r, "slug")

// Query parameters
limit, start := request.ParseParamPagination(r)
active, err := request.ParseOptionalParamBool(r, "active")
name, err := request.ParseParamString(r, "name")

// Body decoding + validation + model conversion
var form CreateOrderForm
model, err := request.ParseForm[Order](r, &form)
```

## Database Transactions

```go
storer := database.NewStorer(db)

err := storer.Transaction(ctx, func(txCtx context.Context) error {
    if err := db.Create(txCtx, insertQuery, &order); err != nil {
        return err // triggers automatic rollback
    }
    return db.Update(txCtx, updateInventoryQuery, order.ProductID, order.Quantity)
    // committed on success
})
```

## Custom Validation

```go
// Register custom validators at startup
validator.Init(
    validator.NewValidation("is_git_url", func(fl validator.Field) bool {
        value := fl.Field().String()
        return strings.HasPrefix(value, "https://") && strings.HasSuffix(value, ".git")
    }),
)

// Use in struct tags
type CreateRepoForm struct {
    URL    string `json:"url" validate:"required,is_git_url"`
    Branch string `json:"branch" validate:"required"`
}
```

## Inter-Service Communication

The `httpclient` package provides an HTTP client with exponential backoff retry and automatic propagation of `X-Request-ID` and `X-Session-ID` headers for distributed tracing.

```go
conn := httpclient.NewConnector(&httpclient.Config{
    URL: "http://order-service:8080",
})

var order OrderDTO
err := conn.Request(ctx, &httpclient.Request{
    Method: http.MethodGet,
    URI:    fmt.Sprintf("/v1/orders/%s", orderID),
    QueryParams: map[string]string{
        "include": "items",
    },
}, &order)
```

## Development

```bash
make test                        # Run all unit tests with coverage
make GOTESTRUN=TestName test     # Run a specific test by name
make lint                        # Run golangci-lint
make fix                         # Format code (goimports) + tidy/vendor modules
make cover                       # Show text coverage report
make cover/html                  # Open HTML coverage report
```
