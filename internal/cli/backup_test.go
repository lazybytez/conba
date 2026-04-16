package cli_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/lazybytez/conba/internal/cli"
	"github.com/lazybytez/conba/internal/discovery"
	"github.com/lazybytez/conba/internal/runtime"
)

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
