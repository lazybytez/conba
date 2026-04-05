package restic_test

import (
	"context"
	"errors"
	"testing"

	"github.com/lazybytez/conba/internal/restic"
)

func TestSnapshots_Success(t *testing.T) {
	t.Parallel()

	stdout := `[{"short_id":"abc12345","time":"2025-01-15T10:30:00Z",` +
		`"paths":["/data"],"tags":["container=app"],"hostname":"host1"}]`
	client := newHelperClient(t, 0, stdout, "")

	snapshots, err := client.Snapshots(context.Background(), []string{"container=app"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(snapshots) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snapshots))
	}

	if snapshots[0].ID != "abc12345" {
		t.Errorf("expected ID %q, got %q", "abc12345", snapshots[0].ID)
	}

	if snapshots[0].Hostname != "host1" {
		t.Errorf("expected hostname %q, got %q", "host1", snapshots[0].Hostname)
	}
}

func TestSnapshots_Empty(t *testing.T) {
	t.Parallel()

	client := newHelperClient(t, 0, "[]", "")

	snapshots, err := client.Snapshots(context.Background(), nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(snapshots) != 0 {
		t.Errorf("expected empty slice, got %d snapshots", len(snapshots))
	}
}

func TestSnapshots_Failure(t *testing.T) {
	t.Parallel()

	client := newHelperClient(t, 1, "", "fatal: unable to open repo")

	_, err := client.Snapshots(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, restic.ErrResticFailed) {
		t.Errorf("expected error wrapping ErrResticFailed, got %v", err)
	}
}

func TestSnapshots_InvalidJSON(t *testing.T) {
	t.Parallel()

	client := newHelperClient(t, 0, "not-json", "")

	_, err := client.Snapshots(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}
