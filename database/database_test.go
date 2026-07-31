//go:build unit
// +build unit

package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"testing"

	"github.com/jmoiron/sqlx"

	"github.com/luanguimaraesla/garlic/errors"
)

// The fake driver below answers every query with an empty result set, which is
// the only database behaviour this regression needs. It runs entirely in
// process: no socket, no credentials, no external SQL.

type noRowsConnector struct{}

func (noRowsConnector) Connect(context.Context) (driver.Conn, error) { return noRowsConn{}, nil }

func (noRowsConnector) Driver() driver.Driver { return noRowsDriver{} }

type noRowsDriver struct{}

func (noRowsDriver) Open(string) (driver.Conn, error) { return noRowsConn{}, nil }

type noRowsConn struct{}

func (noRowsConn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }

func (noRowsConn) Close() error { return nil }

func (noRowsConn) Begin() (driver.Tx, error) { return nil, driver.ErrSkip }

func (noRowsConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	return emptyRows{}, nil
}

type emptyRows struct{}

func (emptyRows) Columns() []string { return []string{"id"} }

func (emptyRows) Close() error { return nil }

func (emptyRows) Next([]driver.Value) error { return io.EOF }

type createResource struct {
	ID string `db:"id"`
}

// An INSERT that reports success but returns no row leaves the resource unscanned,
// so Create must surface it as a failure instead of a silent success.
func TestDatabase_Create_noReturnedRowsReturnsError(t *testing.T) {
	db := New(&Config{})
	db.DB = sqlx.NewDb(sql.OpenDB(noRowsConnector{}), "postgres")
	t.Cleanup(func() { _ = db.Close() })

	const query = "INSERT INTO resources DEFAULT VALUES RETURNING id"

	err := db.Create(context.Background(), query, &createResource{})
	if err == nil {
		t.Fatal("Create returned nil for an INSERT that returned no rows")
	}

	e, ok := errors.AsKind(err, errors.KindSystemError)
	if !ok {
		t.Fatalf("Create error = %v, want a KindSystemError", err)
	}
	if e.Error() != "no rows returned while scanning resource during creation" {
		t.Errorf("message = %q, want the no-row message", e.Error())
	}

	found := false
	for _, ctx := range e.Troubleshooting.Context {
		if fields, ok := ctx.(map[string]any); ok && fields["query"] == query {
			found = true
		}
	}
	if !found {
		t.Errorf("the query context was lost, got %v", e.Troubleshooting.Context)
	}
}
