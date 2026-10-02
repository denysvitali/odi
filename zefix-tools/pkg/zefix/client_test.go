package zefix

import (
	"context"
	"errors"
	"testing"
)

func TestNewWithContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client, err := NewWithContext(ctx, "host=127.0.0.1 port=1 user=synthetic dbname=synthetic sslmode=disable")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled initialization, got %v", err)
	}
	if client != nil {
		t.Fatal("failed initialization returned a client")
	}
}
