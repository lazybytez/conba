package cli_test

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/lazybytez/conba/internal/cli"
	"github.com/lazybytez/conba/internal/restic"
)

var errStatusStub = errors.New("stub: status error")

func TestNewStatusCommand_Use(t *testing.T) {
	t.Parallel()

	cmd := cli.NewStatusCommand()

	if cmd.Use != "status" {
		t.Errorf("Use = %q, want %q", cmd.Use, "status")
	}
}

func TestNewStatusCommand_Short(t *testing.T) {
	t.Parallel()

	cmd := cli.NewStatusCommand()

	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}
}

func TestFormatSize_Bytes(t *testing.T) {
	t.Parallel()

	got := cli.FormatSize(500)

	if got != "500 B" {
		t.Errorf("FormatSize(500) = %q, want %q", got, "500 B")
	}
}

func TestFormatSize_KiB(t *testing.T) {
	t.Parallel()

	got := cli.FormatSize(2048)

	if got != "2.00 KiB" {
		t.Errorf("FormatSize(2048) = %q, want %q", got, "2.00 KiB")
	}
}

func TestFormatSize_MiB(t *testing.T) {
	t.Parallel()

	got := cli.FormatSize(5 * 1024 * 1024)

	if got != "5.00 MiB" {
		t.Errorf("FormatSize(5*1024*1024) = %q, want %q", got, "5.00 MiB")
	}
}

func TestFormatSize_Zero(t *testing.T) {
	t.Parallel()

	got := cli.FormatSize(0)

	if got != "0 B" {
		t.Errorf("FormatSize(0) = %q, want %q", got, "0 B")
	}
}

func TestFormatSize_GiB(t *testing.T) {
	t.Parallel()

	got := cli.FormatSize(3 * 1024 * 1024 * 1024)

	if got != "3.00 GiB" {
		t.Errorf("FormatSize(3*1024*1024*1024) = %q, want %q", got, "3.00 GiB")
	}
}

func TestFormatSize_TiB(t *testing.T) {
	t.Parallel()

	got := cli.FormatSize(2 * 1024 * 1024 * 1024 * 1024)

	if got != "2.00 TiB" {
		t.Errorf("FormatSize(2*TiB) = %q, want %q", got, "2.00 TiB")
	}
}

func TestPrintStatus_Ready(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	snapshots := []restic.Snapshot{
		{
			ID:       "abc123",
			Time:     time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC),
			Paths:    []string{"/data"},
			Tags:     []string{"daily"},
			Hostname: "host1",
		},
	}
	stats := restic.RepoStats{
		TotalSize:      5 * 1024 * 1024,
		TotalFileCount: 42,
	}

	err := cli.PrintStatus(&buf, "/repo/path", snapshots, stats)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "Repository:") {
		t.Error("output should contain Repository:")
	}

	if !strings.Contains(output, "Status:     ready") {
		t.Error("output should contain Status:     ready")
	}

	if !strings.Contains(output, "Snapshots:") {
		t.Error("output should contain Snapshots:")
	}

	if !strings.Contains(output, "Total size:") {
		t.Error("output should contain Total size:")
	}
}

func TestPrintStatus_NoSnapshots(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	stats := restic.RepoStats{
		TotalSize:      0,
		TotalFileCount: 0,
	}

	err := cli.PrintStatus(&buf, "/repo/path", nil, stats)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "n/a") {
		t.Errorf("output should contain n/a for latest snapshot, got %q", output)
	}
}

func TestPrintNotInitialized(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	err := cli.PrintNotInitialized(&buf, "/repo/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "not initialized") {
		t.Error("output should contain 'not initialized'")
	}

	if !strings.Contains(output, "conba init") {
		t.Error("output should contain 'conba init'")
	}
}

func TestHandleStatusError_NotInitialized(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	err := cli.HandleStatusError(&buf, "/repo/path",
		fmt.Errorf("Is there a repository at the following location?: %w", errStatusStub))
	if err != nil {
		t.Fatalf("want nil, got %v", err)
	}

	if !strings.Contains(buf.String(), "not initialized") {
		t.Error("output should contain 'not initialized'")
	}
}

func TestHandleStatusError_Locked(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	err := cli.HandleStatusError(&buf, "/repo/path",
		fmt.Errorf("unable to create lock: %w", errStatusStub))
	if err != nil {
		t.Fatalf("want nil, got %v", err)
	}

	if !strings.Contains(buf.String(), "locked") {
		t.Error("output should contain 'locked'")
	}
}

func TestHandleStatusError_Unknown(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	err := cli.HandleStatusError(&buf, "/repo/path",
		fmt.Errorf("some unexpected error: %w", errStatusStub))
	if err == nil {
		t.Fatal("want error, got nil")
	}

	if !strings.Contains(err.Error(), "check repository") {
		t.Errorf("error should contain 'check repository', got %v", err)
	}

	if buf.Len() != 0 {
		t.Errorf("no output expected for unknown errors, got %q", buf.String())
	}
}

func TestPrintLocked(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	err := cli.PrintLocked(&buf, "/repo/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "locked") {
		t.Error("output should contain 'locked'")
	}

	if !strings.Contains(output, "conba unlock") {
		t.Error("output should contain 'conba unlock'")
	}
}
