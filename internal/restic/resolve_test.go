package restic_test

import (
	"errors"
	"testing"
	"time"

	"github.com/lazybytez/conba/internal/restic"
)

func TestResolveSnapshot_EmptyList(t *testing.T) {
	t.Parallel()

	_, err := restic.ResolveSnapshot(nil, []string{"web"}, "")
	if err == nil {
		t.Fatal("expected error for empty snapshot list, got nil")
	}

	if !errors.Is(err, restic.ErrSnapshotNotFound) {
		t.Errorf("expected ErrSnapshotNotFound, got %v", err)
	}
}

func TestResolveSnapshot_NoMatch(t *testing.T) {
	t.Parallel()

	snapshots := []restic.Snapshot{
		{
			ID:       "aaa",
			Time:     time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			Paths:    []string{"/data"},
			Tags:     []string{"db"},
			Hostname: "host1",
		},
	}

	_, err := restic.ResolveSnapshot(snapshots, []string{"web"}, "")
	if err == nil {
		t.Fatal("expected error for no match, got nil")
	}

	if !errors.Is(err, restic.ErrSnapshotNotFound) {
		t.Errorf("expected ErrSnapshotNotFound, got %v", err)
	}
}

func TestResolveSnapshot_SingleMatch(t *testing.T) {
	t.Parallel()

	want := restic.Snapshot{
		ID:       "aaa",
		Time:     time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		Paths:    []string{"/data"},
		Tags:     []string{"web", "production"},
		Hostname: "host1",
	}
	snapshots := []restic.Snapshot{want}

	got, err := restic.ResolveSnapshot(snapshots, []string{"web"}, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got.ID != want.ID {
		t.Errorf("expected ID %q, got %q", want.ID, got.ID)
	}
}

func TestResolveSnapshot_MultiMatchPicksLatest(t *testing.T) {
	t.Parallel()

	older := restic.Snapshot{
		ID:       "old",
		Time:     time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		Paths:    []string{"/data"},
		Tags:     []string{"web"},
		Hostname: "host1",
	}
	newer := restic.Snapshot{
		ID:       "new",
		Time:     time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC),
		Paths:    []string{"/data"},
		Tags:     []string{"web"},
		Hostname: "host1",
	}
	middle := restic.Snapshot{
		ID:       "mid",
		Time:     time.Date(2024, 3, 1, 12, 0, 0, 0, time.UTC),
		Paths:    []string{"/data"},
		Tags:     []string{"web"},
		Hostname: "host1",
	}

	snapshots := []restic.Snapshot{older, newer, middle}

	got, err := restic.ResolveSnapshot(snapshots, []string{"web"}, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got.ID != "new" {
		t.Errorf("expected newest snapshot ID %q, got %q", "new", got.ID)
	}
}

func TestResolveSnapshot_MultiTagAllRequired(t *testing.T) {
	t.Parallel()

	withOnlyOne := restic.Snapshot{
		ID:       "one",
		Time:     time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC),
		Paths:    []string{"/data"},
		Tags:     []string{"web"},
		Hostname: "host1",
	}
	withBoth := restic.Snapshot{
		ID:       "both",
		Time:     time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		Paths:    []string{"/data"},
		Tags:     []string{"web", "production"},
		Hostname: "host1",
	}

	snapshots := []restic.Snapshot{withOnlyOne, withBoth}

	got, err := restic.ResolveSnapshot(snapshots, []string{"web", "production"}, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got.ID != "both" {
		t.Errorf("expected snapshot ID %q with all tags, got %q", "both", got.ID)
	}
}

func TestResolveSnapshot_ExplicitIDMatch(t *testing.T) {
	t.Parallel()

	older := restic.Snapshot{
		ID:       "old",
		Time:     time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		Paths:    []string{"/data"},
		Tags:     []string{"web"},
		Hostname: "host1",
	}
	newer := restic.Snapshot{
		ID:       "new",
		Time:     time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC),
		Paths:    []string{"/data"},
		Tags:     []string{"web"},
		Hostname: "host1",
	}

	snapshots := []restic.Snapshot{older, newer}

	got, err := restic.ResolveSnapshot(snapshots, []string{"web"}, "old")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got.ID != "old" {
		t.Errorf("expected explicit ID %q, got %q", "old", got.ID)
	}
}

func TestResolveSnapshot_ExplicitIDMismatch(t *testing.T) {
	t.Parallel()

	dbSnap := restic.Snapshot{
		ID:       "aaa",
		Time:     time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		Paths:    []string{"/data"},
		Tags:     []string{"db"},
		Hostname: "host1",
	}

	snapshots := []restic.Snapshot{dbSnap}

	_, err := restic.ResolveSnapshot(snapshots, []string{"web"}, "aaa")
	if err == nil {
		t.Fatal("expected error for explicit ID with tag mismatch, got nil")
	}

	if !errors.Is(err, restic.ErrSnapshotTagMismatch) {
		t.Errorf("expected ErrSnapshotTagMismatch, got %v", err)
	}
}

func TestResolveSnapshot_ExplicitIDNotFound(t *testing.T) {
	t.Parallel()

	snapshots := []restic.Snapshot{
		{
			ID:       "aaa",
			Time:     time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			Paths:    []string{"/data"},
			Tags:     []string{"web"},
			Hostname: "host1",
		},
	}

	_, err := restic.ResolveSnapshot(snapshots, []string{"web"}, "missing")
	if err == nil {
		t.Fatal("expected error for missing explicit ID, got nil")
	}

	if !errors.Is(err, restic.ErrSnapshotNotFound) {
		t.Errorf("expected ErrSnapshotNotFound, got %v", err)
	}
}

func TestResolveSnapshot_NoTagsRequired(t *testing.T) {
	t.Parallel()

	older := restic.Snapshot{
		ID:       "old",
		Time:     time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		Paths:    []string{"/data"},
		Tags:     []string{"web"},
		Hostname: "host1",
	}
	newer := restic.Snapshot{
		ID:       "new",
		Time:     time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC),
		Paths:    []string{"/data"},
		Tags:     []string{"db"},
		Hostname: "host1",
	}

	snapshots := []restic.Snapshot{older, newer}

	got, err := restic.ResolveSnapshot(snapshots, nil, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got.ID != "new" {
		t.Errorf("expected newest snapshot regardless of tags, got %q", got.ID)
	}
}
