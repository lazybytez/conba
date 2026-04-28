package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/lazybytez/conba/internal/cli"
	"github.com/lazybytez/conba/internal/discovery"
	"github.com/lazybytez/conba/internal/filter"
	"github.com/lazybytez/conba/internal/runtime"
)

func TestShortID_Long(t *testing.T) {
	t.Parallel()

	id := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	got := cli.ShortID(id)

	if got != "abcdef123456" {
		t.Errorf("ShortID(%q) = %q, want %q", id, got, "abcdef123456")
	}
}

func TestShortID_Exact(t *testing.T) {
	t.Parallel()

	id := "abcdef123456"
	got := cli.ShortID(id)

	if got != id {
		t.Errorf("ShortID(%q) = %q, want %q", id, got, id)
	}
}

func TestShortID_Short(t *testing.T) {
	t.Parallel()

	id := "abc"
	got := cli.ShortID(id)

	if got != id {
		t.Errorf("ShortID(%q) = %q, want %q", id, got, id)
	}
}

func TestShortID_Empty(t *testing.T) {
	t.Parallel()

	got := cli.ShortID("")

	if got != "" {
		t.Errorf("ShortID(%q) = %q, want %q", "", got, "")
	}
}

func newContainerInfo(id, name string) runtime.ContainerInfo {
	return runtime.ContainerInfo{
		ID:     id,
		Name:   name,
		Labels: nil,
		Mounts: nil,
	}
}

func newLabeledContainerInfo(name string, labels map[string]string) runtime.ContainerInfo {
	return runtime.ContainerInfo{
		ID:     "abc123def456",
		Name:   name,
		Labels: labels,
		Mounts: nil,
	}
}

func newMountInfo(mountType, name, destination string) runtime.MountInfo {
	return runtime.MountInfo{
		Type:        mountType,
		Name:        name,
		Source:      "",
		Destination: destination,
		ReadOnly:    false,
	}
}

func TestGroupByContainer_SingleContainer(t *testing.T) {
	t.Parallel()

	ctr := newContainerInfo("c1", "app")
	targets := []discovery.Target{
		{Container: ctr, Mount: newMountInfo("volume", "vol1", "/vol1")},
		{Container: ctr, Mount: newMountInfo("volume", "vol2", "/vol2")},
	}

	groups := cli.GroupByContainer(targets)

	if len(groups) != 1 {
		t.Fatalf("got %d groups, want 1", len(groups))
	}

	if len(groups[0]) != 2 {
		t.Errorf("group[0] has %d targets, want 2", len(groups[0]))
	}
}

func TestGroupByContainer_MultipleContainers(t *testing.T) {
	t.Parallel()

	targets := []discovery.Target{
		{Container: newContainerInfo("c1", "app"), Mount: newMountInfo("volume", "v1", "/v1")},
		{Container: newContainerInfo("c2", "db"), Mount: newMountInfo("volume", "v2", "/v2")},
	}

	groups := cli.GroupByContainer(targets)

	if len(groups) != 2 {
		t.Fatalf("got %d groups, want 2", len(groups))
	}

	if groups[0][0].Container.ID != "c1" {
		t.Errorf("group[0] container ID = %q, want %q", groups[0][0].Container.ID, "c1")
	}

	if groups[1][0].Container.ID != "c2" {
		t.Errorf("group[1] container ID = %q, want %q", groups[1][0].Container.ID, "c2")
	}
}

func TestGroupByContainer_PreservesOrder(t *testing.T) {
	t.Parallel()

	targets := []discovery.Target{
		{Container: newContainerInfo("c2", "db"), Mount: newMountInfo("volume", "v1", "/v1")},
		{Container: newContainerInfo("c1", "app"), Mount: newMountInfo("volume", "v2", "/v2")},
		{Container: newContainerInfo("c2", "db"), Mount: newMountInfo("volume", "v3", "/v3")},
	}

	groups := cli.GroupByContainer(targets)

	if len(groups) != 2 {
		t.Fatalf("got %d groups, want 2", len(groups))
	}

	if groups[0][0].Container.ID != "c2" {
		t.Errorf("first group should be c2 (first seen), got %q", groups[0][0].Container.ID)
	}

	if groups[1][0].Container.ID != "c1" {
		t.Errorf("second group should be c1, got %q", groups[1][0].Container.ID)
	}

	if len(groups[0]) != 2 {
		t.Errorf("first group should have 2 targets, got %d", len(groups[0]))
	}
}

func TestGroupByContainer_Empty(t *testing.T) {
	t.Parallel()

	groups := cli.GroupByContainer(nil)

	if groups != nil {
		t.Errorf("got %v, want nil", groups)
	}
}

func TestPrintResult_Empty(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	err := cli.PrintResult(&buf, filter.Result{
		Included: nil,
		Excluded: nil,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "No containers with volumes found.") {
		t.Errorf("output = %q, want message about no containers", buf.String())
	}
}

func TestPrintResult_IncludedOnly(t *testing.T) {
	t.Parallel()

	result := filter.Result{
		Included: []discovery.Target{
			{
				Container: newContainerInfo("abc123def456", "app"),
				Mount:     newMountInfo("volume", "data", "/data"),
			},
		},
		Excluded: nil,
	}

	var buf bytes.Buffer

	err := cli.PrintResult(&buf, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "=== Included ===") {
		t.Error("output should contain Included section header")
	}

	if strings.Contains(output, "=== Excluded ===") {
		t.Error("output should not contain Excluded section header")
	}
}

func TestPrintResult_ExcludedOnly(t *testing.T) {
	t.Parallel()

	result := filter.Result{
		Included: nil,
		Excluded: []filter.Exclusion{
			{
				Target: discovery.Target{
					Container: newContainerInfo("abc123def456", "app"),
					Mount:     newMountInfo("volume", "cache", "/cache"),
				},
				Reason: "label exclude",
			},
		},
	}

	var buf bytes.Buffer

	err := cli.PrintResult(&buf, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "=== Excluded ===") {
		t.Error("output should contain Excluded section header")
	}

	if strings.Contains(output, "=== Included ===") {
		t.Error("output should not contain Included section header")
	}
}

func TestPrintResult_Both(t *testing.T) {
	t.Parallel()

	result := filter.Result{
		Included: []discovery.Target{
			{
				Container: newContainerInfo("abc123def456", "app"),
				Mount:     newMountInfo("volume", "data", "/data"),
			},
		},
		Excluded: []filter.Exclusion{
			{
				Target: discovery.Target{
					Container: newContainerInfo("def456abc123", "db"),
					Mount:     newMountInfo("bind", "/host/path", "/mnt"),
				},
				Reason: "not matching",
			},
		},
	}

	var buf bytes.Buffer

	err := cli.PrintResult(&buf, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "=== Included ===") {
		t.Error("output should contain Included section header")
	}

	if !strings.Contains(output, "=== Excluded ===") {
		t.Error("output should contain Excluded section header")
	}
}

func TestPrintExcluded_ShowsReason(t *testing.T) {
	t.Parallel()

	exclusions := []filter.Exclusion{
		{
			Target: discovery.Target{
				Container: newContainerInfo("abc123def456", "worker"),
				Mount:     newMountInfo("volume", "tmp", "/tmp"),
			},
			Reason: "excluded by label",
		},
	}

	var buf bytes.Buffer

	err := cli.PrintExcluded(&buf, exclusions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "excluded by label") {
		t.Errorf("output should contain exclusion reason, got %q", output)
	}
}

func TestPrintResult_LabeledContainerShowsPreBackupSection(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		"conba.pre-backup.command":   "pg_dump -U postgres app",
		"conba.pre-backup.mode":      "alongside",
		"conba.pre-backup.container": "db",
		"conba.pre-backup.filename":  "app.sql",
	}

	result := filter.Result{
		Included: []discovery.Target{
			{
				Container: newLabeledContainerInfo("app", labels),
				Mount:     newMountInfo("volume", "data", "/data"),
			},
		},
		Excluded: nil,
	}

	var buf bytes.Buffer

	err := cli.PrintResult(&buf, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "pre-backup:") {
		t.Errorf("output should contain pre-backup section header, got:\n%s", output)
	}

	if !strings.Contains(output, "command:   pg_dump -U postgres app") {
		t.Errorf("output should contain command line, got:\n%s", output)
	}

	if !strings.Contains(output, "mode:      alongside") {
		t.Errorf("output should contain mode line, got:\n%s", output)
	}

	if !strings.Contains(output, "exec:      db") {
		t.Errorf("output should contain exec line, got:\n%s", output)
	}

	if !strings.Contains(output, "filename:  app.sql") {
		t.Errorf("output should contain filename line, got:\n%s", output)
	}
}

func TestPrintResult_LabeledContainerResolvesDefaults(t *testing.T) {
	t.Parallel()

	// Only command label set; exec and filename default to the labeled
	// container name; mode defaults to replace.
	labels := map[string]string{
		"conba.pre-backup.command": "redis-cli SAVE",
	}

	result := filter.Result{
		Included: []discovery.Target{
			{
				Container: newLabeledContainerInfo("cache", labels),
				Mount:     newMountInfo("volume", "redis-data", "/data"),
			},
		},
		Excluded: nil,
	}

	var buf bytes.Buffer

	err := cli.PrintResult(&buf, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "command:   redis-cli SAVE") {
		t.Errorf("output should contain command line, got:\n%s", output)
	}

	if !strings.Contains(output, "mode:      replace") {
		t.Errorf("output should contain default mode replace, got:\n%s", output)
	}

	if !strings.Contains(output, "exec:      cache") {
		t.Errorf("output should default exec to labeled container name, got:\n%s", output)
	}

	if !strings.Contains(output, "filename:  cache") {
		t.Errorf("output should default filename to labeled container name, got:\n%s", output)
	}
}

func TestPrintResult_UnlabeledContainerHasNoPreBackupSection(t *testing.T) {
	t.Parallel()

	result := filter.Result{
		Included: []discovery.Target{
			{
				Container: newContainerInfo("abc123def456", "app"),
				Mount:     newMountInfo("volume", "data", "/data"),
			},
		},
		Excluded: nil,
	}

	var buf bytes.Buffer

	err := cli.PrintResult(&buf, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	if strings.Contains(output, "pre-backup") {
		t.Errorf(
			"output should not contain pre-backup section for unlabeled container, got:\n%s",
			output,
		)
	}
}

func TestPrintResult_MultiMountLabeledContainerSectionAppearsOnce(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		"conba.pre-backup.command": "pg_dump -U postgres app",
	}
	ctr := newLabeledContainerInfo("app", labels)

	result := filter.Result{
		Included: []discovery.Target{
			{Container: ctr, Mount: newMountInfo("volume", "data", "/data")},
			{Container: ctr, Mount: newMountInfo("volume", "logs", "/logs")},
			{Container: ctr, Mount: newMountInfo("volume", "cache", "/cache")},
		},
		Excluded: nil,
	}

	var buf bytes.Buffer

	err := cli.PrintResult(&buf, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	count := strings.Count(output, "pre-backup:")

	if count != 1 {
		t.Errorf(
			"pre-backup section should appear exactly once for multi-mount container, got %d:\n%s",
			count,
			output,
		)
	}
}

func TestPrintResult_LabeledContainerWithFeatureDisabledNotes(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		"conba.pre-backup.command": "pg_dump -U postgres app",
	}

	result := filter.Result{
		Included: []discovery.Target{
			{
				Container: newLabeledContainerInfo("app", labels),
				Mount:     newMountInfo("volume", "data", "/data"),
			},
		},
		Excluded: nil,
	}

	var buf bytes.Buffer

	// preBackupEnabled = false should append a "(disabled, ...)" marker
	// to the pre-backup section header so operators reading inspect output
	// understand the labels are dormant.
	err := cli.PrintResultWithFeatureFlag(&buf, result, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "pre-backup") {
		t.Fatalf(
			"output should still contain pre-backup section even when disabled, got:\n%s",
			output,
		)
	}

	if !strings.Contains(output, "disabled") {
		t.Errorf(
			"output should mark the pre-backup section as disabled, got:\n%s",
			output,
		)
	}

	if !strings.Contains(output, "pre_backup_commands.enabled") {
		t.Errorf(
			"output should reference the config key in the disabled note, got:\n%s",
			output,
		)
	}
}

func TestPrintResult_LabeledContainerWithFeatureEnabledNoNote(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		"conba.pre-backup.command": "pg_dump -U postgres app",
	}

	result := filter.Result{
		Included: []discovery.Target{
			{
				Container: newLabeledContainerInfo("app", labels),
				Mount:     newMountInfo("volume", "data", "/data"),
			},
		},
		Excluded: nil,
	}

	var buf bytes.Buffer

	err := cli.PrintResultWithFeatureFlag(&buf, result, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	if strings.Contains(output, "disabled") {
		t.Errorf(
			"output must not mark the pre-backup section as disabled when "+
				"the feature is enabled, got:\n%s",
			output,
		)
	}
}

func TestPrintResult_LabeledContainerShowsRestoreCommandWhenSet(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		"conba.pre-backup.command":         "pg_dump -U postgres app",
		"conba.pre-backup.restore-command": "psql -U postgres app",
	}

	result := filter.Result{
		Included: []discovery.Target{
			{
				Container: newLabeledContainerInfo("app", labels),
				Mount:     newMountInfo("volume", "data", "/data"),
			},
		},
		Excluded: nil,
	}

	var buf bytes.Buffer

	err := cli.PrintResult(&buf, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "restore-command: psql -U postgres app") {
		t.Errorf("output should contain restore-command line, got:\n%s", output)
	}
}

func TestPrintResult_LabeledContainerShowsRestoreCommandUnsetMarker(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		"conba.pre-backup.command": "pg_dump -U postgres app",
	}

	result := filter.Result{
		Included: []discovery.Target{
			{
				Container: newLabeledContainerInfo("app", labels),
				Mount:     newMountInfo("volume", "data", "/data"),
			},
		},
		Excluded: nil,
	}

	var buf bytes.Buffer

	err := cli.PrintResult(&buf, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "restore-command: (unset)") {
		t.Errorf("output should contain restore-command (unset) marker, got:\n%s", output)
	}
}

func TestPrintResult_UnlabeledContainerHasNoRestoreCommandLine(t *testing.T) {
	t.Parallel()

	result := filter.Result{
		Included: []discovery.Target{
			{
				Container: newContainerInfo("abc123def456", "app"),
				Mount:     newMountInfo("volume", "data", "/data"),
			},
		},
		Excluded: nil,
	}

	var buf bytes.Buffer

	err := cli.PrintResult(&buf, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	if strings.Contains(output, "restore-command") {
		t.Errorf(
			"output should not contain restore-command line for unlabeled container, got:\n%s",
			output,
		)
	}
}

func TestPrintResult_MultiMountLabeledContainerRestoreCommandAppearsOnce(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		"conba.pre-backup.command":         "pg_dump -U postgres app",
		"conba.pre-backup.restore-command": "psql -U postgres app",
	}
	ctr := newLabeledContainerInfo("app", labels)

	result := filter.Result{
		Included: []discovery.Target{
			{Container: ctr, Mount: newMountInfo("volume", "data", "/data")},
			{Container: ctr, Mount: newMountInfo("volume", "logs", "/logs")},
			{Container: ctr, Mount: newMountInfo("volume", "cache", "/cache")},
		},
		Excluded: nil,
	}

	var buf bytes.Buffer

	err := cli.PrintResult(&buf, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	count := strings.Count(output, "restore-command:")

	if count != 1 {
		t.Errorf(
			"restore-command line should appear exactly once for multi-mount container, got %d:\n%s",
			count,
			output,
		)
	}
}

func TestPrintResult_LabeledContainerRestoreCommandWithFeatureDisabled(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		"conba.pre-backup.command":         "pg_dump -U postgres app",
		"conba.pre-backup.restore-command": "psql -U postgres app",
	}

	result := filter.Result{
		Included: []discovery.Target{
			{
				Container: newLabeledContainerInfo("app", labels),
				Mount:     newMountInfo("volume", "data", "/data"),
			},
		},
		Excluded: nil,
	}

	var buf bytes.Buffer

	err := cli.PrintResultWithFeatureFlag(&buf, result, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "disabled") {
		t.Errorf(
			"output should still mark the pre-backup section as disabled, got:\n%s",
			output,
		)
	}

	if !strings.Contains(output, "restore-command: psql -U postgres app") {
		t.Errorf(
			"output should still render restore-command line when feature disabled, got:\n%s",
			output,
		)
	}
}

func TestPrintResult_InvalidModeEmitsErrorLine(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		"conba.pre-backup.command": "pg_dump",
		"conba.pre-backup.mode":    "bogus",
	}

	result := filter.Result{
		Included: []discovery.Target{
			{
				Container: newLabeledContainerInfo("db", labels),
				Mount:     newMountInfo("volume", "pgdata", "/var/lib/postgresql/data"),
			},
		},
		Excluded: nil,
	}

	var buf bytes.Buffer

	err := cli.PrintResult(&buf, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "pre-backup: invalid") {
		t.Errorf("output should contain pre-backup invalid marker, got:\n%s", output)
	}

	if !strings.Contains(output, "mode=bogus") {
		t.Errorf("output should mention the invalid mode value, got:\n%s", output)
	}
}
