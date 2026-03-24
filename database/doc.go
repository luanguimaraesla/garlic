// Package database provides a PostgreSQL abstraction over sqlx and pgx with
// connection management, CRUD operations, transactions, filtering, and mocking.
//
// # Connection
//
// Create a [Database] with [New] and connect:
//
//	db := database.New(&database.Config{
//	    Host:     "localhost",
//	    Port:     5432,
//	    Database: "myapp",
//	    Username: "postgres",
//	    Password: "secret",
//	    SSLMode:  database.SSLModeDisable,
//	})
//	if err := db.Connect(); err != nil { ... }
//
// # CRUD Operations
//
// All operations accept a context and use named SQL parameters via sqlx:
//
//	err := db.Create(ctx, "INSERT INTO users (name) VALUES (:name) RETURNING *", &user)
//	err := db.Read(ctx, "SELECT * FROM users WHERE id = $1", &user, id)
//	err := db.List(ctx, "SELECT * FROM users", &users)
//	err := db.Update(ctx, "UPDATE users SET name = $1 WHERE id = $2", name, id)
//	err := db.Delete(ctx, "DELETE FROM users WHERE id = $1", id)
//
// [Create] detects PostgreSQL unique and not-null constraint violations and
// returns user-friendly errors. [Read], [Update], and [Delete] return
// [KindNotFoundError] when no rows are affected.
//
// For arbitrary queries, use [Database.RawExec] (positional params) or
// [Database.NamedRawExec] (named params), both returning [sql.Result].
//
// # Transactions
//
// Use [Storer.Transaction] for automatic commit/rollback lifecycle:
//
//	storer := database.NewStorer(db)
//	err := storer.Transaction(ctx, func(txCtx context.Context) error {
//	    if err := db.Create(txCtx, insertQuery, &resource); err != nil {
//	        return err
//	    }
//	    return db.Update(txCtx, updateQuery, args...)
//	})
//
// The transaction is committed on success and rolled back on error or panic.
// Nested calls to [BeginContext] reuse the existing transaction.
//
// # Filtering and Patching
//
// [ExtractFilters] reads filter-tagged struct fields to build WHERE clauses.
// The database/utils sub-package provides [utils.JoinedPatchResourceBindings]
// for generating SET clauses from partially-filled structs (nil pointer fields
// are skipped).
//
// # Mocking
//
// [StoreMock] implements the executor interface for unit tests. Configure it
// with WithResult, WithError, or WithModel, then assert queries with
// [AssertQueryExecution].
package database
