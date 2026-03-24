//go:build unit
// +build unit

package tracing

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestSetAndGetRequestId(t *testing.T) {
	id := uuid.New()
	ctx := SetContextRequestId(context.Background(), id)

	got, err := GetRequestIdFromContext(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != id {
		t.Errorf("want %s, got %s", id, got)
	}
}

func TestGetRequestIdFromContext_missing(t *testing.T) {
	_, err := GetRequestIdFromContext(context.Background())
	if err == nil {
		t.Fatal("expected error for missing request id, got nil")
	}
}

func TestGetRequestIdFromContext_wrongType(t *testing.T) {
	ctx := context.WithValue(context.Background(), RequestIdKey, "not-a-uuid")

	_, err := GetRequestIdFromContext(ctx)
	if err == nil {
		t.Fatal("expected error for wrong type, got nil")
	}
}

func TestSetAndGetSessionId(t *testing.T) {
	const sessionID = "session-abc-123"
	ctx := SetContextSessionId(context.Background(), sessionID)

	got, err := GetSessionIdFromContext(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != sessionID {
		t.Errorf("want %s, got %s", sessionID, got)
	}
}

func TestGetSessionIdFromContext_missing(t *testing.T) {
	_, err := GetSessionIdFromContext(context.Background())
	if err == nil {
		t.Fatal("expected error for missing session id, got nil")
	}
}

func TestGetSessionIdFromContext_wrongType(t *testing.T) {
	ctx := context.WithValue(context.Background(), SessionIdKey, 12345)

	_, err := GetSessionIdFromContext(ctx)
	if err == nil {
		t.Fatal("expected error for wrong type, got nil")
	}
}

func TestMustGetRequestIdFromContext_success(t *testing.T) {
	id := uuid.New()
	ctx := SetContextRequestId(context.Background(), id)

	got := MustGetRequestIdFromContext(ctx)
	if got != id {
		t.Errorf("want %s, got %s", id, got)
	}
}

func TestMustGetSessionIdFromContext_success(t *testing.T) {
	const sessionID = "session-xyz"
	ctx := SetContextSessionId(context.Background(), sessionID)

	got := MustGetSessionIdFromContext(ctx)
	if got != sessionID {
		t.Errorf("want %s, got %s", sessionID, got)
	}
}

func TestRequestIdRoundTrip_preservesValue(t *testing.T) {
	id := uuid.New()
	ctx := context.Background()
	ctx = SetContextRequestId(ctx, id)
	ctx = SetContextSessionId(ctx, "session-1")

	gotID, err := GetRequestIdFromContext(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotID != id {
		t.Errorf("request id: want %s, got %s", id, gotID)
	}

	gotSession, err := GetSessionIdFromContext(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotSession != "session-1" {
		t.Errorf("session id: want session-1, got %s", gotSession)
	}
}
