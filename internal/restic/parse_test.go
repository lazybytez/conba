package restic_test

import (
	"strings"
	"testing"
	"time"

	"github.com/lazybytez/conba/internal/restic"
)

func TestParseSnapshotsValidJSON(t *testing.T) {
	t.Parallel()

	input := []byte(`[
		{
			"short_id":"abc12345",
			"time":"2025-01-15T10:30:00Z",
			"paths":["/data"],
			"tags":["container=app","volume=data"],
			"hostname":"backup-host"
		},
		{
			"short_id":"def67890",
			"time":"2025-02-20T14:00:00Z",
			"paths":["/logs"],
			"tags":["container=app","volume=logs"],
			"hostname":"backup-host"
		}
	]`)

	snapshots, err := restic.ParseSnapshots(input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(snapshots) != 2 {
		t.Fatalf("expected 2 snapshots, got %d", len(snapshots))
	}

	t.Run("first snapshot", func(t *testing.T) {
		t.Parallel()
		assertSnapshot(t, snapshots[0],
			"abc12345",
			time.Date(2025, time.January, 15, 10, 30, 0, 0, time.UTC),
			[]string{"/data"},
			[]string{"container=app", "volume=data"},
			"backup-host",
		)
	})

	t.Run("second snapshot", func(t *testing.T) {
		t.Parallel()
		assertSnapshot(t, snapshots[1],
			"def67890",
			time.Date(2025, time.February, 20, 14, 0, 0, 0, time.UTC),
			[]string{"/logs"},
			[]string{"container=app", "volume=logs"},
			"backup-host",
		)
	})
}

func TestParseSnapshotsEmptyArray(t *testing.T) {
	t.Parallel()

	snapshots, err := restic.ParseSnapshots([]byte(`[]`))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(snapshots) != 0 {
		t.Errorf("expected empty slice, got %d elements", len(snapshots))
	}
}

func TestParseSnapshotsInvalidJSON(t *testing.T) {
	t.Parallel()

	_, err := restic.ParseSnapshots([]byte(`not json`))
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if got := err.Error(); !strings.Contains(got, "parsing snapshots") {
		t.Errorf(
			"expected error containing 'parsing snapshots', got %q",
			got,
		)
	}
}

func TestParseSnapshotsNullInput(t *testing.T) {
	t.Parallel()

	snapshots, err := restic.ParseSnapshots([]byte(`null`))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(snapshots) != 0 {
		t.Errorf("expected empty or nil slice, got %v", snapshots)
	}
}

func assertSnapshot(
	t *testing.T,
	snap restic.Snapshot,
	wantID string,
	wantTime time.Time,
	wantPaths []string,
	wantTags []string,
	wantHostname string,
) {
	t.Helper()

	if snap.ID != wantID {
		t.Errorf("expected ID %s, got %s", wantID, snap.ID)
	}

	if !snap.Time.Equal(wantTime) {
		t.Errorf("expected time %v, got %v", wantTime, snap.Time)
	}

	assertStringSlice(t, "paths", wantPaths, snap.Paths)
	assertStringSlice(t, "tags", wantTags, snap.Tags)

	if snap.Hostname != wantHostname {
		t.Errorf(
			"expected hostname %s, got %s",
			wantHostname,
			snap.Hostname,
		)
	}
}

func TestParseStatsValidJSON(t *testing.T) {
	t.Parallel()

	input := []byte(`{"total_size":123456,"total_file_count":42}`)

	stats, err := restic.ParseStats(input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if stats.TotalSize != 123456 {
		t.Errorf("expected TotalSize 123456, got %d", stats.TotalSize)
	}

	if stats.TotalFileCount != 42 {
		t.Errorf("expected TotalFileCount 42, got %d", stats.TotalFileCount)
	}
}

func TestParseStatsZeroValues(t *testing.T) {
	t.Parallel()

	input := []byte(`{"total_size":0,"total_file_count":0}`)

	stats, err := restic.ParseStats(input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if stats.TotalSize != 0 {
		t.Errorf("expected TotalSize 0, got %d", stats.TotalSize)
	}

	if stats.TotalFileCount != 0 {
		t.Errorf("expected TotalFileCount 0, got %d", stats.TotalFileCount)
	}
}

func TestParseStatsInvalidJSON(t *testing.T) {
	t.Parallel()

	_, err := restic.ParseStats([]byte(`not json`))
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if got := err.Error(); !strings.Contains(got, "parsing stats") {
		t.Errorf(
			"expected error containing 'parsing stats', got %q",
			got,
		)
	}
}

func assertStringSlice(
	t *testing.T,
	field string,
	want []string,
	got []string,
) {
	t.Helper()

	if len(got) != len(want) {
		t.Errorf("expected %s %v, got %v", field, want, got)

		return
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("expected %s[%d] %q, got %q", field, i, want[i], got[i])
		}
	}
}
