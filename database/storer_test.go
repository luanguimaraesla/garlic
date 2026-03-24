//go:build unit
// +build unit

package database

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/luanguimaraesla/garlic/errors"
)

// storeMockImpl is a minimal Store implementation for testing Storer
type storeMockImpl struct {
	beginErr  error
	commitErr error
}

func (s *storeMockImpl) BeginContext(ctx context.Context) (context.Context, func() error, func() error, error) {
	if s.beginErr != nil {
		return ctx, Nop(), Nop(), s.beginErr
	}

	commit := func() error { return s.commitErr }
	rollback := func() error { return nil }
	return ctx, commit, rollback, nil
}

func (s *storeMockImpl) Create(ctx context.Context, query string, resource any) error {
	return nil
}
func (s *storeMockImpl) Read(ctx context.Context, query string, resource any, args ...any) error {
	return nil
}
func (s *storeMockImpl) Update(ctx context.Context, query string, args ...any) error {
	return nil
}
func (s *storeMockImpl) Delete(ctx context.Context, query string, args ...any) error {
	return nil
}
func (s *storeMockImpl) List(ctx context.Context, query string, resourceList any, args ...any) error {
	return nil
}
func (s *storeMockImpl) RawExec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return nil, nil
}
func (s *storeMockImpl) NamedRawExec(ctx context.Context, query string, resource any) (sql.Result, error) {
	return nil, nil
}

func TestStorer_Transaction_success(t *testing.T) {
	storer := NewStorer(&storeMockImpl{})

	called := false
	err := storer.Transaction(context.Background(), func(ctx context.Context) error {
		called = true
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("transaction function was not called")
	}
}

func TestStorer_Transaction_fnError(t *testing.T) {
	storer := NewStorer(&storeMockImpl{})

	err := storer.Transaction(context.Background(), func(ctx context.Context) error {
		return fmt.Errorf("fn failed")
	})

	if err == nil {
		t.Fatal("expected error when fn returns error, got nil")
	}
}

func TestStorer_Transaction_beginError(t *testing.T) {
	storer := NewStorer(&storeMockImpl{
		beginErr: errors.New(errors.KindDatabaseTransactionError, "begin failed"),
	})

	err := storer.Transaction(context.Background(), func(ctx context.Context) error {
		t.Fatal("fn should not be called when begin fails")
		return nil
	})

	if err == nil {
		t.Fatal("expected error when begin fails, got nil")
	}
}

func TestStorer_Transaction_commitError(t *testing.T) {
	storer := NewStorer(&storeMockImpl{
		commitErr: fmt.Errorf("commit failed"),
	})

	err := storer.Transaction(context.Background(), func(ctx context.Context) error {
		return nil
	})

	if err == nil {
		t.Fatal("expected error when commit fails, got nil")
	}
}

func TestStorer_Transaction_panic_recovery(t *testing.T) {
	storer := NewStorer(&storeMockImpl{})

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic to be re-raised")
		}
	}()

	_ = storer.Transaction(context.Background(), func(ctx context.Context) error {
		panic("test panic")
	})
}

func TestNewStorer(t *testing.T) {
	store := &storeMockImpl{}
	storer := NewStorer(store)
	if storer.Store != store {
		t.Error("NewStorer should set Store field")
	}
}
