package restic_test

import (
	"context"
	"errors"
	"testing"

	"github.com/lazybytez/conba/internal/restic"
)

func TestForget_ReducesSnapshots(t *testing.T) {
	t.Parallel()

	runForgetReducesSnapshots(t, "forget-test", restic.ForgetOptions{
		Prune:  false,
		DryRun: false,
	})
}

func TestForget_Failure(t *testing.T) {
	t.Parallel()

	repoPath, password := newTestRepo(t)
	client := newTestClient(t, repoPath, password)

	err := client.Forget(context.Background(), nil, restic.ForgetPolicy{
		KeepDaily:   0,
		KeepWeekly:  0,
		KeepMonthly: 0,
		KeepYearly:  0,
	}, restic.ForgetOptions{
		Prune:  false,
		DryRun: false,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, restic.ErrResticFailed) {
		t.Errorf("expected error wrapping ErrResticFailed, got %v", err)
	}
}

func TestForget_PruneFlagAppliesToRepo(t *testing.T) {
	t.Parallel()

	runForgetReducesSnapshots(t, "forget-prune-test", restic.ForgetOptions{
		Prune:  true,
		DryRun: false,
	})
}

// runForgetReducesSnapshots seeds a fresh restic repo with three tagged
// snapshots, calls Forget with KeepDaily=1 plus the supplied options, and
// asserts the snapshot count drops to exactly 1.
func runForgetReducesSnapshots(t *testing.T, tag string, opts restic.ForgetOptions) {
	t.Helper()

	repoPath, password := newTestRepo(t)
	client := newTestClient(t, repoPath, password)

	ctx := context.Background()

	err := client.Init(ctx)
	if err != nil {
		t.Fatalf("init repo: %v", err)
	}

	dataDir := t.TempDir()
	tags := []string{tag}

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
	}, opts)
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
