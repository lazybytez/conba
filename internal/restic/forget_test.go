package restic_test

import (
	"context"
	"errors"
	"testing"

	"github.com/lazybytez/conba/internal/restic"
)

func TestForget_ReducesSnapshots(t *testing.T) {
	repoPath, password := newTestRepo(t)
	client := newTestClient(t, repoPath, password)

	ctx := context.Background()

	err := client.Init(ctx)
	if err != nil {
		t.Fatalf("init repo: %v", err)
	}

	dataDir := t.TempDir()
	tags := []string{"forget-test"}

	createBackups(t, client, dataDir, tags, 3)

	snapshots, err := client.Snapshots(ctx, tags)
	if err != nil {
		t.Fatalf("list snapshots before forget: %v", err)
	}

	if len(snapshots) != 3 {
		t.Fatalf("expected 3 snapshots before forget, got %d", len(snapshots))
	}

	err = client.Forget(ctx, tags, restic.ForgetPolicy{
		KeepDaily:   1,
		KeepWeekly:  0,
		KeepMonthly: 0,
		KeepYearly:  0,
	})
	if err != nil {
		t.Fatalf("forget: %v", err)
	}

	snapshots, err = client.Snapshots(ctx, tags)
	if err != nil {
		t.Fatalf("list snapshots after forget: %v", err)
	}

	if len(snapshots) != 1 {
		t.Errorf("expected 1 snapshot after forget, got %d", len(snapshots))
	}
}

func TestForget_Failure(t *testing.T) {
	repoPath, password := newTestRepo(t)
	client := newTestClient(t, repoPath, password)

	err := client.Forget(context.Background(), nil, restic.ForgetPolicy{
		KeepDaily:   0,
		KeepWeekly:  0,
		KeepMonthly: 0,
		KeepYearly:  0,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, restic.ErrResticFailed) {
		t.Errorf("expected error wrapping ErrResticFailed, got %v", err)
	}
}
