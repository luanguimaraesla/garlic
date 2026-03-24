//go:build unit
// +build unit

package database

import (
	"fmt"
	"testing"
)

func TestNewStoreMock(t *testing.T) {
	mock := NewStoreMock()
	if mock.err != nil {
		t.Error("expected nil error in new mock")
	}
}

func TestStoreMock_WithError(t *testing.T) {
	err := fmt.Errorf("test error")
	mock := NewStoreMock().WithError(err)
	if mock.err != err {
		t.Error("WithError should set the error")
	}
}

func TestStoreMock_WithResult(t *testing.T) {
	result := NewSqlResult(1, 2)
	mock := NewStoreMock().WithResult(result)
	if mock.result.lastInsertId != 1 || mock.result.rowsAffected != 2 {
		t.Error("WithResult should set the result")
	}
}

func TestStoreMock_Exec(t *testing.T) {
	mock := NewStoreMock()
	result, err := mock.Exec("SELECT 1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rows, _ := result.RowsAffected()
	if rows != 0 {
		t.Errorf("want 0 rows, got %d", rows)
	}

	calls, query := mock.getExecResults()
	if calls != 1 {
		t.Errorf("want 1 call, got %d", calls)
	}
	if query != "SELECT 1" {
		t.Errorf("want 'SELECT 1', got %q", query)
	}
}

func TestStoreMock_Exec_withError(t *testing.T) {
	expectedErr := fmt.Errorf("exec failed")
	mock := NewStoreMock().WithError(expectedErr)

	_, err := mock.Exec("SELECT 1")
	if err != expectedErr {
		t.Errorf("want %v, got %v", expectedErr, err)
	}
}

func TestStoreMock_NamedExec(t *testing.T) {
	mock := NewStoreMock()
	_, err := mock.NamedExec("INSERT INTO users (name) VALUES (:name)", map[string]any{"name": "Alice"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	calls, _ := mock.getExecResults()
	if calls != 1 {
		t.Errorf("want 1 call, got %d", calls)
	}
}

func TestStoreMock_ResetExecResults(t *testing.T) {
	mock := NewStoreMock()
	_, _ = mock.Exec("SELECT 1")
	mock.ResetExecResults()

	calls, query := mock.getExecResults()
	if calls != 0 {
		t.Errorf("want 0 calls after reset, got %d", calls)
	}
	if query != "" {
		t.Errorf("want empty query after reset, got %q", query)
	}
}

func TestSqlResult(t *testing.T) {
	r := NewSqlResult(10, 20)

	lastID, err := r.LastInsertId()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lastID != 10 {
		t.Errorf("LastInsertId: want 10, got %d", lastID)
	}

	rows, err := r.RowsAffected()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rows != 20 {
		t.Errorf("RowsAffected: want 20, got %d", rows)
	}
}

func TestDefaultSqlResult(t *testing.T) {
	r := DefaultSqlResult()

	lastID, _ := r.LastInsertId()
	if lastID != 0 {
		t.Errorf("LastInsertId: want 0, got %d", lastID)
	}

	rows, _ := r.RowsAffected()
	if rows != 0 {
		t.Errorf("RowsAffected: want 0, got %d", rows)
	}
}

func TestTxMock_Commit(t *testing.T) {
	mock := NewTxMock()
	if err := mock.Commit(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTxMock_Commit_withError(t *testing.T) {
	expectedErr := fmt.Errorf("commit failed")
	mock := NewTxMock().WithError(expectedErr)
	if err := mock.Commit(); err != expectedErr {
		t.Errorf("want %v, got %v", expectedErr, err)
	}
}

func TestTxMock_Rollback(t *testing.T) {
	mock := NewTxMock()
	_, _ = mock.Exec("SELECT 1")

	calls, _ := mock.getExecResults()
	if calls != 1 {
		t.Errorf("want 1 call before rollback, got %d", calls)
	}

	if err := mock.Rollback(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	calls, _ = mock.getExecResults()
	if calls != 0 {
		t.Errorf("want 0 calls after rollback, got %d", calls)
	}
}

func TestTxMock_Exec(t *testing.T) {
	mock := NewTxMock()
	_, err := mock.Exec("UPDATE users SET name = 'Bob'")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	calls, query := mock.getExecResults()
	if calls != 1 {
		t.Errorf("want 1 call, got %d", calls)
	}
	if query != "UPDATE users SET name = 'Bob'" {
		t.Errorf("unexpected query: %q", query)
	}
}

func TestAssertQueryExecution_success(t *testing.T) {
	mock := NewStoreMock()
	_, _ = mock.Exec("SELECT * FROM users WHERE id = $1")

	// Should not fail
	AssertQueryExecution(t, "SELECT * FROM users WHERE id = $1", &mock)
}

func TestCleanString(t *testing.T) {
	got := cleanString("SELECT * FROM users WHERE id = $1")
	want := "SELECTFROMusersWHEREid1"
	if got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}
