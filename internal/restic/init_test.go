package restic_test

import (
	"context"
	"errors"
	"testing"

	"github.com/lazybytez/conba/internal/restic"
)

func TestInit_Success(t *testing.T) {
	t.Parallel()

	client := newHelperClient(t, 0, "", "")

	err := client.Init(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestInit_AlreadyInitialized(t *testing.T) {
	t.Parallel()

	client := newHelperClient(t, 1, "", "repository master key and target already initialized")

	err := client.Init(context.Background())
	if err != nil {
		t.Fatalf("expected no error for already initialized repo, got %v", err)
	}
}

func TestInit_Failure(t *testing.T) {
	t.Parallel()

	client := newHelperClient(t, 1, "", "unable to open repository")

	err := client.Init(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, restic.ErrResticFailed) {
		t.Errorf("expected error wrapping ErrResticFailed, got %v", err)
	}
}
