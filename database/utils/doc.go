// Package utils provides query-building and type-conversion utilities for the
// database package.
//
// # Named Queries
//
// [Named] converts a query with named parameters (e.g., :name) to positional
// PostgreSQL parameters ($1, $2, ...) using sqlx, including IN-clause expansion:
//
//	query, args := utils.Named(
//	    "SELECT * FROM users WHERE id IN (:ids)",
//	    map[string]any{"ids": []int{1, 2, 3}},
//	)
//
// # Patch Bindings
//
// [JoinedPatchResourceBindings] generates comma-separated "column = :column"
// pairs for PATCH-style updates, skipping nil pointer fields:
//
//	type PatchUser struct {
//	    Name  *string `db:"name"`
//	    Email *string `db:"email"`
//	}
//	// With Name set and Email nil:
//	// JoinedPatchResourceBindings(&patch) => "name = :name"
//
// [NamedResourceBindings] returns the individual binding strings as a slice.
//
// # ResourceIter
//
// [ResourceIter] returns an iterator over non-nil struct fields tagged with db,
// yielding (column name, value) pairs. It uses the range-over-func pattern.
//
// # StringSlice
//
// [StringSlice] wraps []string and implements [database/sql.Scanner] and
// [database/sql/driver.Valuer] for PostgreSQL array columns.
package utils
