package restic_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/lazybytez/conba/internal/restic"
)

func TestDump_Success(t *testing.T) {
	t.Parallel()

	client := newStreamTestClient(t)
	ctx := context.Background()

	err := client.Init(ctx)
	if err != nil {
		t.Fatalf("init repo: %v", err)
	}

	// Create a known stdin via /bin/sh -c 'printf hello-dump'
	// (printf avoids the trailing newline echo would add).
	err = client.BackupFromCommand(
		ctx,
		"payload.txt",
		[]string{"dump-test"},
		[]string{"/bin/sh", "-c", "printf hello-dump"},
	)
	if err != nil {
		t.Fatalf("backup-from-command: %v", err)
	}

	snapshots, err := client.Snapshots(ctx, []string{"dump-test"})
	if err != nil {
		t.Fatalf("snapshots: %v", err)
	}

	if len(snapshots) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snapshots))
	}

	var buf bytes.Buffer

	err = client.Dump(ctx, snapshots[0].ID, "/payload.txt", &buf)
	if err != nil {
		t.Fatalf("dump: %v", err)
	}

	if got := buf.String(); got != "hello-dump" {
		t.Errorf("expected dumped content %q, got %q", "hello-dump", got)
	}
}

func TestDump_NonZeroExit(t *testing.T) {
	t.Parallel()

	client := newStreamTestClient(t)
	ctx := context.Background()

	err := client.Init(ctx)
	if err != nil {
		t.Fatalf("init repo: %v", err)
	}

	var buf bytes.Buffer

	err = client.Dump(ctx, "doesnotexist123", "/payload.txt", &buf)
	if err == nil {
		t.Fatal("expected error for unknown snapshot id, got nil")
	}

	if !errors.Is(err, restic.ErrResticFailed) {
		t.Errorf("expected error wrapping ErrResticFailed, got %v", err)
	}
}
