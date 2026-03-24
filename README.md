# Garlic

Garlic is an internal Go framework providing standardized utilities for building Web APIs, libraries, workers, and service integrations. It is consumed as a library — there is no main entry point.

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

    "github.com/luanguimaraesla/garlic/logging"
    "github.com/luanguimaraesla/garlic/middleware"
    "github.com/luanguimaraesla/garlic/rest"
)

func main() {
    logging.Init(&logging.Config{Level: "info", Encoding: "json"})

    srv := rest.GetServer("api")
    r := srv.Router()

    r.Use(
        middleware.ContextCancel,
        middleware.Logging,
        middleware.Tracing,
        middleware.MetricsMonitor,
        middleware.ContentTypeJson,
    )

    rest.RegisterApp(r, &MyApp{})

    errc := srv.Listen(context.Background(), ":8080")
    if err := <-errc; err != nil {
        logging.Global().Fatal(err.Error())
    }
}

type MyApp struct{}

func (a *MyApp) Routes() rest.Routes {
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
| [`crypto`](./crypto) | AES-CBC encryption/decryption and SHA-256 hashing |
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

Errors are classified by **kind** in a hierarchy that maps automatically to HTTP status codes:

```
KindError (base)
├── KindUserError (400)
│   ├── KindInvalidRequestError (400)
│   │   └── KindValidationError (400)
│   ├── KindNotFoundError (404)
│   ├── KindAuthError (401)
│   └── KindForbiddenError (403)
└── KindSystemError (500)
    └── KindDatabaseTransactionError (500)
```

```go
// Create errors with a kind
err := errors.New(errors.KindValidationError, "email is invalid",
    errors.Hint("provide a valid email address"),
)

// Wrap existing errors, preserving the kind
err = errors.Propagate(dbErr, "failed to fetch user")

// Wrap with an explicit kind
err = errors.PropagateAs(errors.KindNotFoundError, dbErr, "user not found")

// Check error kinds (traverses the hierarchy)
if errors.IsKind(err, errors.KindUserError) {
    // handles ValidationError, NotFoundError, AuthError, etc.
}
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
func main() {
    validator.Init(
        validator.NewValidation("is_positive", func(fl validator.Field) bool {
            return fl.Field().Int() > 0
        }),
    )
}

type CreateOrder struct {
    Quantity int `json:"quantity" validate:"required,is_positive"`
}

func handler(w http.ResponseWriter, r *http.Request) error {
    order, err := request.ParseForm[Order](r, &CreateOrder{})
    if err != nil {
        return err // returns KindValidationError with per-field hints
    }
    // ...
}
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
