package filter_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/lazybytez/conba/internal/discovery"
	"github.com/lazybytez/conba/internal/filter"
	"github.com/lazybytez/conba/internal/runtime"
)

func makePreBackupTarget(labels map[string]string) discovery.Target {
	return discovery.Target{
		Container: runtime.ContainerInfo{
			ID:     "c1",
			Name:   "app",
			Labels: labels,
			Mounts: nil,
		},
		Mount: runtime.MountInfo{
			Type:        runtime.MountTypeVolume,
			Name:        "data",
			Source:      "",
			Destination: "/data",
			ReadOnly:    false,
		},
	}
}

func TestPreBackup_NoCommandLabel(t *testing.T) {
	t.Parallel()

	target := makePreBackupTarget(map[string]string{})

	spec, ok, err := filter.PreBackup(target)
	if err != nil {
		t.Fatalf("want no error, got %v", err)
	}

	if ok {
		t.Errorf("want ok=false, got ok=true")
	}

	if spec != (filter.Spec{Command: "", Mode: "", Container: "", Filename: "", RestoreCommand: ""}) {
		t.Errorf("want zero spec, got %+v", spec)
	}
}

// TestPreBackup_EmptyCommandLabel asserts that an empty-string command label
// is treated as "no spec" rather than as an enabled-but-empty pre-backup
// command. An empty command would otherwise produce a useless empty stream
// snapshot, which is never the user's intent.
func TestPreBackup_EmptyCommandLabel(t *testing.T) {
	t.Parallel()

	target := makePreBackupTarget(map[string]string{
		filter.LabelPreBackupCommand: "",
	})

	spec, ok, err := filter.PreBackup(target)
	if err != nil {
		t.Fatalf("want no error, got %v", err)
	}

	if ok {
		t.Errorf("want ok=false for empty command, got ok=true")
	}

	if spec != (filter.Spec{Command: "", Mode: "", Container: "", Filename: "", RestoreCommand: ""}) {
		t.Errorf("want zero spec for empty command, got %+v", spec)
	}
}

func TestPreBackup_CommandOnlyAppliesDefaults(t *testing.T) {
	t.Parallel()

	target := makePreBackupTarget(map[string]string{
		filter.LabelPreBackupCommand: "mysqldump --all-databases",
	})

	spec, ok, err := filter.PreBackup(target)
	if err != nil {
		t.Fatalf("want no error, got %v", err)
	}

	if !ok {
		t.Fatalf("want ok=true, got ok=false")
	}

	if spec.Command != "mysqldump --all-databases" {
		t.Errorf("want command 'mysqldump --all-databases', got %q", spec.Command)
	}

	if spec.Mode != filter.ModeReplace {
		t.Errorf("want mode ModeReplace, got %q", spec.Mode)
	}

	if spec.Container != "" {
		t.Errorf("want container '' (default), got %q", spec.Container)
	}

	if spec.Filename != "" {
		t.Errorf("want filename '' (default), got %q", spec.Filename)
	}

	if spec.RestoreCommand != "" {
		t.Errorf("want restore command '' (default), got %q", spec.RestoreCommand)
	}
}

func TestPreBackup_AllLabelsSet(t *testing.T) {
	t.Parallel()

	target := makePreBackupTarget(map[string]string{
		filter.LabelPreBackupCommand:        "pg_dump mydb",
		filter.LabelPreBackupMode:           "alongside",
		filter.LabelPreBackupContainer:      "sidecar",
		filter.LabelPreBackupFilename:       "dump.sql",
		filter.LabelPreBackupRestoreCommand: "psql mydb",
	})

	spec, ok, err := filter.PreBackup(target)
	if err != nil {
		t.Fatalf("want no error, got %v", err)
	}

	if !ok {
		t.Fatalf("want ok=true, got ok=false")
	}

	want := filter.Spec{
		Command:        "pg_dump mydb",
		Mode:           filter.ModeAlongside,
		Container:      "sidecar",
		Filename:       "dump.sql",
		RestoreCommand: "psql mydb",
	}

	if spec != want {
		t.Errorf("want %+v, got %+v", want, spec)
	}
}

func TestPreBackup_ModeReplaceExplicit(t *testing.T) {
	t.Parallel()

	target := makePreBackupTarget(map[string]string{
		filter.LabelPreBackupCommand: "echo hello",
		filter.LabelPreBackupMode:    "replace",
	})

	spec, ok, err := filter.PreBackup(target)
	if err != nil {
		t.Fatalf("want no error, got %v", err)
	}

	if !ok {
		t.Fatalf("want ok=true, got ok=false")
	}

	if spec.Mode != filter.ModeReplace {
		t.Errorf("want ModeReplace, got %q", spec.Mode)
	}
}

func TestPreBackup_ModeAlongsideExplicit(t *testing.T) {
	t.Parallel()

	target := makePreBackupTarget(map[string]string{
		filter.LabelPreBackupCommand: "echo hello",
		filter.LabelPreBackupMode:    "alongside",
	})

	spec, ok, err := filter.PreBackup(target)
	if err != nil {
		t.Fatalf("want no error, got %v", err)
	}

	if !ok {
		t.Fatalf("want ok=true, got ok=false")
	}

	if spec.Mode != filter.ModeAlongside {
		t.Errorf("want ModeAlongside, got %q", spec.Mode)
	}
}

func TestPreBackup_InvalidMode(t *testing.T) {
	t.Parallel()

	target := makePreBackupTarget(map[string]string{
		filter.LabelPreBackupCommand: "echo hello",
		filter.LabelPreBackupMode:    "bogus",
	})

	spec, ok, err := filter.PreBackup(target)
	if err == nil {
		t.Fatalf("want error, got nil")
	}

	if ok {
		t.Errorf("want ok=false, got ok=true")
	}

	if spec != (filter.Spec{Command: "", Mode: "", Container: "", Filename: "", RestoreCommand: ""}) {
		t.Errorf("want zero spec, got %+v", spec)
	}

	if !strings.Contains(err.Error(), `"bogus"`) {
		t.Errorf("want error to name offending value %q, got %q", "bogus", err.Error())
	}

	if !errors.Is(err, filter.ErrInvalidPreBackupMode) {
		t.Errorf("want errors.Is(err, ErrInvalidPreBackupMode) to be true, got false")
	}
}

func TestPreBackup_LabelConstantValues(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		got  string
		want string
	}{
		{"LabelPreBackupCommand", filter.LabelPreBackupCommand, "conba.pre-backup.command"},
		{"LabelPreBackupMode", filter.LabelPreBackupMode, "conba.pre-backup.mode"},
		{"LabelPreBackupContainer", filter.LabelPreBackupContainer, "conba.pre-backup.container"},
		{"LabelPreBackupFilename", filter.LabelPreBackupFilename, "conba.pre-backup.filename"},
		{
			"LabelPreBackupRestoreCommand", filter.LabelPreBackupRestoreCommand,
			"conba.pre-backup.restore-command",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if testCase.got != testCase.want {
				t.Errorf("want %q, got %q", testCase.want, testCase.got)
			}
		})
	}
}

// TestPreBackup_RestoreCommandLabelSet asserts that the restore-command label
// value is propagated to Spec.RestoreCommand when both the command label and
// the restore-command label are set.
func TestPreBackup_RestoreCommandLabelSet(t *testing.T) {
	t.Parallel()

	target := makePreBackupTarget(map[string]string{
		filter.LabelPreBackupCommand:        "pg_dump mydb",
		filter.LabelPreBackupRestoreCommand: "psql mydb",
	})

	spec, ok, err := filter.PreBackup(target)
	if err != nil {
		t.Fatalf("want no error, got %v", err)
	}

	if !ok {
		t.Fatalf("want ok=true, got ok=false")
	}

	if spec.RestoreCommand != "psql mydb" {
		t.Errorf("want restore command 'psql mydb', got %q", spec.RestoreCommand)
	}
}

// TestPreBackup_RestoreCommandLabelAbsent asserts that an absent
// restore-command label produces an empty Spec.RestoreCommand without
// affecting the (populated, true, nil) result for the parent command.
func TestPreBackup_RestoreCommandLabelAbsent(t *testing.T) {
	t.Parallel()

	target := makePreBackupTarget(map[string]string{
		filter.LabelPreBackupCommand: "pg_dump mydb",
	})

	spec, ok, err := filter.PreBackup(target)
	if err != nil {
		t.Fatalf("want no error, got %v", err)
	}

	if !ok {
		t.Fatalf("want ok=true, got ok=false")
	}

	if spec.RestoreCommand != "" {
		t.Errorf("want restore command '' when label absent, got %q", spec.RestoreCommand)
	}
}

// TestPreBackup_RestoreCommandWithAllLabels asserts the full round-trip when
// every supported pre-backup label is set, including restore-command.
func TestPreBackup_RestoreCommandWithAllLabels(t *testing.T) {
	t.Parallel()

	target := makePreBackupTarget(map[string]string{
		filter.LabelPreBackupCommand:        "pg_dump mydb",
		filter.LabelPreBackupMode:           "alongside",
		filter.LabelPreBackupContainer:      "sidecar",
		filter.LabelPreBackupFilename:       "dump.sql",
		filter.LabelPreBackupRestoreCommand: "psql mydb < /restore/dump.sql",
	})

	spec, ok, err := filter.PreBackup(target)
	if err != nil {
		t.Fatalf("want no error, got %v", err)
	}

	if !ok {
		t.Fatalf("want ok=true, got ok=false")
	}

	want := filter.Spec{
		Command:        "pg_dump mydb",
		Mode:           filter.ModeAlongside,
		Container:      "sidecar",
		Filename:       "dump.sql",
		RestoreCommand: "psql mydb < /restore/dump.sql",
	}

	if spec != want {
		t.Errorf("want %+v, got %+v", want, spec)
	}
}
