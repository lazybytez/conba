package cli_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/lazybytez/conba/internal/cli"
	"github.com/lazybytez/conba/internal/discovery"
	"github.com/lazybytez/conba/internal/filter"
	"github.com/lazybytez/conba/internal/runtime"
)

func newLabeledContainer(name string, labels map[string]string) runtime.ContainerInfo {
	return runtime.ContainerInfo{
		ID:     "c-" + name,
		Name:   name,
		Labels: labels,
		Mounts: nil,
	}
}

func TestNewBackupCommand_Use(t *testing.T) {
	t.Parallel()

	cmd := cli.NewBackupCommand()

	if cmd.Use != "backup" {
		t.Errorf("Use = %q, want %q", cmd.Use, "backup")
	}
}

func TestNewBackupCommand_Short(t *testing.T) {
	t.Parallel()

	cmd := cli.NewBackupCommand()

	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}
}

func TestNewBackupCommand_DryRunFlag(t *testing.T) {
	t.Parallel()

	cmd := cli.NewBackupCommand()

	flag := cmd.Flags().Lookup("dry-run")
	if flag == nil {
		t.Fatal("dry-run flag should exist")
	}

	if flag.DefValue != "false" {
		t.Errorf("dry-run default = %q, want %q", flag.DefValue, "false")
	}
}

func TestRunBackup_NilConfig(t *testing.T) {
	t.Parallel()

	assertRunEFailsWithoutConfig(t, cli.NewBackupCommand)
}

func TestPrintDryRun_SingleTarget(t *testing.T) {
	t.Parallel()

	mount := runtime.MountInfo{
		Type:        "volume",
		Name:        "data",
		Source:      "/var/lib/docker/volumes/data/_data",
		Destination: "/data",
		ReadOnly:    false,
	}

	targets := []discovery.Target{
		{
			Container: newContainerInfo("abc123def456789", "app"),
			Mount:     mount,
		},
	}

	var buf bytes.Buffer

	err := cli.PrintDryRun(&buf, targets)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	hostname, _ := os.Hostname()

	assertions := []string{
		"app (abc123def456)",
		"volume  data",
		"\u2192",
		"/var/lib/docker/volumes/data/_data",
		"tags: container=app",
		"volume=data",
		"hostname=" + hostname,
		"1 volume(s) would be backed up.",
	}

	for _, want := range assertions {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q\ngot:\n%s", want, output)
		}
	}
}

func TestPrintDryRun_MultipleTargets(t *testing.T) {
	t.Parallel()

	targets := []discovery.Target{
		{
			Container: newContainerInfo("abc123def456789", "app"),
			Mount: runtime.MountInfo{
				Type:        "volume",
				Name:        "data",
				Source:      "/var/lib/docker/volumes/data/_data",
				Destination: "/data",
				ReadOnly:    false,
			},
		},
		{
			Container: newContainerInfo("def456abc123789", "db"),
			Mount: runtime.MountInfo{
				Type:        "volume",
				Name:        "pgdata",
				Source:      "/var/lib/docker/volumes/pgdata/_data",
				Destination: "/var/lib/postgresql/data",
				ReadOnly:    false,
			},
		},
	}

	var buf bytes.Buffer

	err := cli.PrintDryRun(&buf, targets)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "app") {
		t.Error("output should contain first container name")
	}

	if !strings.Contains(output, "db") {
		t.Error("output should contain second container name")
	}

	if !strings.Contains(output, "2 volume(s) would be backed up.") {
		t.Errorf("output missing summary line\ngot:\n%s", output)
	}
}

func TestPrintDryRun_Empty(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	err := cli.PrintDryRun(&buf, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "0 volume(s) would be backed up.") {
		t.Errorf("output missing summary line\ngot:\n%s", output)
	}
}

func TestPrintPreBackupSummary_LabeledContainer(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		filter.LabelPreBackupCommand: "pg_dumpall -U postgres",
	}

	targets := []discovery.Target{
		{
			Container: newLabeledContainer("db", labels),
			Mount: runtime.MountInfo{
				Type:        "volume",
				Name:        "pgdata",
				Source:      "/var/lib/docker/volumes/pgdata/_data",
				Destination: "/var/lib/postgresql/data",
				ReadOnly:    false,
			},
		},
	}

	var buf bytes.Buffer

	err := cli.PrintPreBackupSummary(&buf, targets)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	wantPrefix := "pre-backup: db mode=replace exec=db filename=db"
	if !strings.Contains(output, wantPrefix) {
		t.Errorf("output missing %q\ngot:\n%s", wantPrefix, output)
	}
}

func TestPrintPreBackupSummary_NoLabels(t *testing.T) {
	t.Parallel()

	targets := []discovery.Target{
		{
			Container: newContainerInfo("c1", "app"),
			Mount: runtime.MountInfo{
				Type:        "volume",
				Name:        "data",
				Source:      "/var/lib/docker/volumes/data/_data",
				Destination: "/data",
				ReadOnly:    false,
			},
		},
	}

	var buf bytes.Buffer

	err := cli.PrintPreBackupSummary(&buf, targets)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.Len() != 0 {
		t.Errorf("expected no output for unlabeled targets, got: %q", buf.String())
	}
}

func TestPrintPreBackupSummary_DedupePerContainer(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		filter.LabelPreBackupCommand: "pg_dumpall -U postgres",
	}

	ctr := newLabeledContainer("db", labels)

	targets := []discovery.Target{
		{
			Container: ctr,
			Mount: runtime.MountInfo{
				Type:        "volume",
				Name:        "pgdata",
				Source:      "/var/lib/docker/volumes/pgdata/_data",
				Destination: "/var/lib/postgresql/data",
				ReadOnly:    false,
			},
		},
		{
			Container: ctr,
			Mount: runtime.MountInfo{
				Type:        "volume",
				Name:        "pgconfig",
				Source:      "/var/lib/docker/volumes/pgconfig/_data",
				Destination: "/etc/postgresql",
				ReadOnly:    false,
			},
		},
	}

	var buf bytes.Buffer

	err := cli.PrintPreBackupSummary(&buf, targets)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	count := strings.Count(output, "pre-backup: db ")

	if count != 1 {
		t.Errorf("want 1 summary line for the container, got %d\noutput:\n%s", count, output)
	}
}

func TestPrintPreBackupSummary_ResolvesExecAndFilenameOverrides(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		filter.LabelPreBackupCommand:   "mysqldump --all-databases",
		filter.LabelPreBackupMode:      "alongside",
		filter.LabelPreBackupContainer: "mysql-sidecar",
		filter.LabelPreBackupFilename:  "mysql.sql",
	}

	targets := []discovery.Target{
		{
			Container: newLabeledContainer("mysql", labels),
			Mount: runtime.MountInfo{
				Type:        "volume",
				Name:        "mysqldata",
				Source:      "/var/lib/docker/volumes/mysqldata/_data",
				Destination: "/var/lib/mysql",
				ReadOnly:    false,
			},
		},
	}

	var buf bytes.Buffer

	err := cli.PrintPreBackupSummary(&buf, targets)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "pre-backup: mysql mode=alongside exec=mysql-sidecar filename=mysql.sql"
	if !strings.Contains(buf.String(), want) {
		t.Errorf("output missing %q\ngot:\n%s", want, buf.String())
	}
}

func TestPrintDryRunWithPreBackup_ReplaceModeReplacesVolumeLine(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		filter.LabelPreBackupCommand: "pg_dumpall -U postgres",
	}

	targets := []discovery.Target{
		{
			Container: newLabeledContainer("db", labels),
			Mount: runtime.MountInfo{
				Type:        "volume",
				Name:        "pgdata",
				Source:      "/var/lib/docker/volumes/pgdata/_data",
				Destination: "/var/lib/postgresql/data",
				ReadOnly:    false,
			},
		},
	}

	var buf bytes.Buffer

	err := cli.PrintDryRunWithPreBackup(&buf, targets)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	wantRun := "would run: pg_dumpall -U postgres in db"
	if !strings.Contains(output, wantRun) {
		t.Errorf("output missing %q\ngot:\n%s", wantRun, output)
	}

	wantSkip := "would skip: db/pgdata"
	if !strings.Contains(output, wantSkip) {
		t.Errorf("output missing %q\ngot:\n%s", wantSkip, output)
	}

	wantReason := "replaced by pre-backup stream"
	if !strings.Contains(output, wantReason) {
		t.Errorf("output missing %q\ngot:\n%s", wantReason, output)
	}

	// Replace mode must NOT show the standard "Would back up" volume line
	// for the replaced mount. The legacy `printDryRun` listing emitted the
	// mount source path; assert the mount source is absent here.
	if strings.Contains(output, "/var/lib/docker/volumes/pgdata/_data") {
		t.Errorf(
			"replace mode should not emit the legacy volume listing; got:\n%s",
			output,
		)
	}
}

func TestPrintDryRunWithPreBackup_AlongsideModeShowsBoth(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		filter.LabelPreBackupCommand: "mysqldump --all-databases",
		filter.LabelPreBackupMode:    "alongside",
	}

	targets := []discovery.Target{
		{
			Container: newLabeledContainer("mysql", labels),
			Mount: runtime.MountInfo{
				Type:        "volume",
				Name:        "mysqldata",
				Source:      "/var/lib/docker/volumes/mysqldata/_data",
				Destination: "/var/lib/mysql",
				ReadOnly:    false,
			},
		},
	}

	var buf bytes.Buffer

	err := cli.PrintDryRunWithPreBackup(&buf, targets)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	wantRun := "would run: mysqldump --all-databases in mysql"
	if !strings.Contains(output, wantRun) {
		t.Errorf("output missing %q\ngot:\n%s", wantRun, output)
	}

	// Alongside mode keeps the existing "Would back up" volume listing.
	if !strings.Contains(output, "/var/lib/docker/volumes/mysqldata/_data") {
		t.Errorf(
			"alongside mode should still emit volume listing for %q; got:\n%s",
			"/var/lib/docker/volumes/mysqldata/_data", output,
		)
	}

	if strings.Contains(output, "would skip") {
		t.Errorf(
			"alongside mode must not emit a skip line; got:\n%s",
			output,
		)
	}
}

func TestPrintDryRunWithPreBackup_OverrideContainerInRunLine(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		filter.LabelPreBackupCommand:   "cat /tmp/payload",
		filter.LabelPreBackupContainer: "admin-sidecar",
	}

	targets := []discovery.Target{
		{
			Container: newLabeledContainer("app", labels),
			Mount: runtime.MountInfo{
				Type:        "volume",
				Name:        "appdata",
				Source:      "/var/lib/docker/volumes/appdata/_data",
				Destination: "/data",
				ReadOnly:    false,
			},
		},
	}

	var buf bytes.Buffer

	err := cli.PrintDryRunWithPreBackup(&buf, targets)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	want := "would run: cat /tmp/payload in admin-sidecar"
	if !strings.Contains(output, want) {
		t.Errorf("output missing %q\ngot:\n%s", want, output)
	}
}

func TestPrintDryRunWithPreBackup_UnlabeledContainerUsesLegacyOutput(t *testing.T) {
	t.Parallel()

	targets := []discovery.Target{
		{
			Container: newContainerInfo("abc123def456789", "app"),
			Mount: runtime.MountInfo{
				Type:        "volume",
				Name:        "data",
				Source:      "/var/lib/docker/volumes/data/_data",
				Destination: "/data",
				ReadOnly:    false,
			},
		},
	}

	var buf bytes.Buffer

	err := cli.PrintDryRunWithPreBackup(&buf, targets)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	if strings.Contains(output, "would run:") {
		t.Errorf(
			"unlabeled container must not emit a 'would run:' line; got:\n%s",
			output,
		)
	}

	if strings.Contains(output, "would skip:") {
		t.Errorf(
			"unlabeled container must not emit a 'would skip:' line; got:\n%s",
			output,
		)
	}

	if !strings.Contains(output, "/var/lib/docker/volumes/data/_data") {
		t.Errorf(
			"unlabeled container should still emit volume listing; got:\n%s",
			output,
		)
	}
}

func TestPrintDryRunWithPreBackup_RunLineEmittedOncePerContainer(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		filter.LabelPreBackupCommand: "pg_dumpall -U postgres",
	}
	ctr := newLabeledContainer("db", labels)

	targets := []discovery.Target{
		{
			Container: ctr,
			Mount: runtime.MountInfo{
				Type:        "volume",
				Name:        "pgdata",
				Source:      "/var/lib/docker/volumes/pgdata/_data",
				Destination: "/var/lib/postgresql/data",
				ReadOnly:    false,
			},
		},
		{
			Container: ctr,
			Mount: runtime.MountInfo{
				Type:        "volume",
				Name:        "pgconfig",
				Source:      "/var/lib/docker/volumes/pgconfig/_data",
				Destination: "/etc/postgresql",
				ReadOnly:    false,
			},
		},
	}

	var buf bytes.Buffer

	err := cli.PrintDryRunWithPreBackup(&buf, targets)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	count := strings.Count(output, "would run:")
	if count != 1 {
		t.Errorf("want 1 'would run:' line per container, got %d\noutput:\n%s",
			count, output)
	}

	skipCount := strings.Count(output, "would skip:")
	if skipCount != 2 {
		t.Errorf("want 2 'would skip:' lines (one per mount), got %d\noutput:\n%s",
			skipCount, output)
	}
}

func TestPrintPreBackupSummary_SkipsInvalidMode(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		filter.LabelPreBackupCommand: "pg_dumpall",
		filter.LabelPreBackupMode:    "bogus",
	}

	targets := []discovery.Target{
		{
			Container: newLabeledContainer("db", labels),
			Mount: runtime.MountInfo{
				Type:        "volume",
				Name:        "pgdata",
				Source:      "/var/lib/docker/volumes/pgdata/_data",
				Destination: "/var/lib/postgresql/data",
				ReadOnly:    false,
			},
		},
	}

	var buf bytes.Buffer

	err := cli.PrintPreBackupSummary(&buf, targets)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.Len() != 0 {
		t.Errorf("expected no output when mode is invalid, got: %q", buf.String())
	}
}
