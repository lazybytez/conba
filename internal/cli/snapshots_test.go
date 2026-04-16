package cli_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/lazybytez/conba/internal/cli"
	"github.com/lazybytez/conba/internal/restic"
	"github.com/spf13/pflag"
)

func TestNewSnapshotsCommand_Use(t *testing.T) {
	t.Parallel()

	cmd := cli.NewSnapshotsCommand()

	if cmd.Use != "snapshots" {
		t.Errorf("Use = %q, want %q", cmd.Use, "snapshots")
	}
}

func TestNewSnapshotsCommand_Short(t *testing.T) {
	t.Parallel()

	cmd := cli.NewSnapshotsCommand()

	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}
}

func TestNewSnapshotsCommand_Flags(t *testing.T) {
	t.Parallel()

	cmd := cli.NewSnapshotsCommand()

	flags := []string{"container", "volume", "hostname"}
	for _, name := range flags {
		flag := cmd.Flags().Lookup(name)
		if flag == nil {
			t.Errorf("flag %q not found", name)

			continue
		}

		if flag.DefValue != "" {
			t.Errorf("flag %q default = %q, want empty", name, flag.DefValue)
		}
	}
}

func TestRunSnapshots_NilConfig(t *testing.T) {
	t.Parallel()

	assertRunEFailsWithoutConfig(t, cli.NewSnapshotsCommand)
}

func TestExtractTag_Found(t *testing.T) {
	t.Parallel()

	tags := []string{"container=app", "volume=data"}
	got := cli.ExtractTag(tags, "container=")

	if got != "app" {
		t.Errorf("ExtractTag(..., %q) = %q, want %q", "container=", got, "app")
	}
}

func TestExtractTag_NotFound(t *testing.T) {
	t.Parallel()

	tags := []string{"container=app"}
	got := cli.ExtractTag(tags, "volume=")

	if got != "-" {
		t.Errorf("ExtractTag(..., %q) = %q, want %q", "volume=", got, "-")
	}
}

func TestExtractTag_Empty(t *testing.T) {
	t.Parallel()

	got := cli.ExtractTag(nil, "container=")

	if got != "-" {
		t.Errorf("ExtractTag(nil, %q) = %q, want %q", "container=", got, "-")
	}
}

func TestBuildFilterTags_AllSet(t *testing.T) {
	t.Parallel()

	got := cli.BuildFilterTags("app", "data", "h1")

	want := []string{"container=app", "volume=data", "hostname=h1"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestBuildFilterTags_Partial(t *testing.T) {
	t.Parallel()

	got := cli.BuildFilterTags("app", "", "")

	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}

	if got[0] != "container=app" {
		t.Errorf("got[0] = %q, want %q", got[0], "container=app")
	}
}

func TestBuildFilterTags_NoneSet(t *testing.T) {
	t.Parallel()

	got := cli.BuildFilterTags("", "", "")

	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}

func TestReadSnapshotFilters_ReadsAllFlags(t *testing.T) {
	t.Parallel()

	cmd := cli.NewSnapshotsCommand()

	mustSet(t, cmd, "container", "app")
	mustSet(t, cmd, "volume", "data")
	mustSet(t, cmd, "hostname", "h1")

	got := cli.ReadSnapshotFilters(cmd.Flags())
	tags := cli.SnapshotFiltersTags(got)

	want := []string{"container=app", "volume=data", "hostname=h1"}
	if len(tags) != len(want) {
		t.Fatalf("len = %d, want %d", len(tags), len(want))
	}

	for i := range want {
		if tags[i] != want[i] {
			t.Errorf("tags[%d] = %q, want %q", i, tags[i], want[i])
		}
	}
}

func mustSet(t *testing.T, cmd interface{ Flags() *pflag.FlagSet }, name, value string) {
	t.Helper()

	err := cmd.Flags().Set(name, value)
	if err != nil {
		t.Fatalf("set flag %q: %v", name, err)
	}
}

func TestPrintSnapshots_Single(t *testing.T) {
	t.Parallel()

	snapshots := []restic.Snapshot{
		{
			ID:       "abc123",
			Time:     time.Date(2026, 4, 12, 14, 30, 0, 0, time.UTC),
			Paths:    []string{"/data"},
			Tags:     []string{"container=app", "volume=data", "hostname=h1"},
			Hostname: "h1",
		},
	}

	var buf bytes.Buffer

	err := cli.PrintSnapshots(&buf, snapshots)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	for _, want := range []string{"ID", "Time", "Container", "Volume", "Hostname"} {
		if !strings.Contains(output, want) {
			t.Errorf("output should contain header %q", want)
		}
	}

	if !strings.Contains(output, "abc123") {
		t.Error("output should contain snapshot ID")
	}

	if !strings.Contains(output, "app") {
		t.Error("output should contain container name")
	}

	if !strings.Contains(output, "data") {
		t.Error("output should contain volume name")
	}

	if !strings.Contains(output, "h1") {
		t.Error("output should contain hostname")
	}

	if !strings.Contains(output, "1 snapshot(s)") {
		t.Errorf("output should contain summary, got %q", output)
	}
}

func TestPrintSnapshots_Multiple(t *testing.T) {
	t.Parallel()

	snapshots := []restic.Snapshot{
		{
			ID:       "abc123",
			Time:     time.Date(2026, 4, 12, 14, 30, 0, 0, time.UTC),
			Paths:    []string{"/data"},
			Tags:     []string{"container=app", "volume=data", "hostname=h1"},
			Hostname: "h1",
		},
		{
			ID:       "def456",
			Time:     time.Date(2026, 4, 12, 15, 0, 0, 0, time.UTC),
			Paths:    []string{"/db"},
			Tags:     []string{"container=db", "volume=pgdata", "hostname=h2"},
			Hostname: "h2",
		},
	}

	var buf bytes.Buffer

	err := cli.PrintSnapshots(&buf, snapshots)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "abc123") {
		t.Error("output should contain first snapshot ID")
	}

	if !strings.Contains(output, "def456") {
		t.Error("output should contain second snapshot ID")
	}

	if !strings.Contains(output, "2 snapshot(s)") {
		t.Errorf("output should contain summary, got %q", output)
	}
}

func TestPrintSnapshots_Empty(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	err := cli.PrintSnapshots(&buf, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "ID") {
		t.Error("output should contain header even for empty list")
	}

	if !strings.Contains(output, "0 snapshot(s)") {
		t.Errorf("output should contain summary, got %q", output)
	}
}
