//go:build unit
// +build unit

package database

import (
	"context"
	"testing"

	"github.com/jmoiron/sqlx"
)

func TestTransaction_returnsNilFromEmptyContext(t *testing.T) {
	tx := Transaction(context.Background())
	if tx != nil {
		t.Error("expected nil transaction from empty context")
	}
}

func TestTransaction_returnsTxFromContext(t *testing.T) {
	tx := &sqlx.Tx{}
	ctx := context.WithValue(context.Background(), TransactionKey, tx)

	got := Transaction(ctx)
	if got != tx {
		t.Error("expected to retrieve the same transaction from context")
	}
}

func TestNop_returnsNil(t *testing.T) {
	fn := Nop()
	if err := fn(); err != nil {
		t.Errorf("Nop() should return nil, got %v", err)
	}
}
