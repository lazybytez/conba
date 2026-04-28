package restic_test

import (
	"slices"
	"testing"

	"github.com/lazybytez/conba/internal/restic"
)

func TestBuildInitArgs(t *testing.T) {
	t.Parallel()

	got := restic.BuildInitArgs()
	want := []string{"init"}

	if !slices.Equal(got, want) {
		t.Errorf("BuildInitArgs() = %v, want %v", got, want)
	}
}

func TestBuildBackupArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		tags []string
		want []string
	}{
		{
			name: "no tags",
			path: "/data/volumes/app",
			tags: nil,
			want: []string{"backup", "/data/volumes/app", "--json"},
		},
		{
			name: "one tag",
			path: "/data/volumes/app",
			tags: []string{"mycontainer"},
			want: []string{"backup", "/data/volumes/app", "--json", "--tag", "mycontainer"},
		},
		{
			name: "multiple tags",
			path: "/data/volumes/app",
			tags: []string{"web", "production"},
			want: []string{
				"backup", "/data/volumes/app", "--json",
				"--tag", "web,production",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := restic.BuildBackupArgs(test.path, test.tags)
			if !slices.Equal(got, test.want) {
				t.Errorf("BuildBackupArgs(%q, %v) = %v, want %v",
					test.path, test.tags, got, test.want)
			}
		})
	}
}

func TestBuildSnapshotArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		tags []string
		want []string
	}{
		{
			name: "no tags",
			tags: nil,
			want: []string{"snapshots", "--json"},
		},
		{
			name: "multiple tags",
			tags: []string{"web", "production"},
			want: []string{"snapshots", "--json", "--tag", "web,production"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := restic.BuildSnapshotArgs(test.tags)
			if !slices.Equal(got, test.want) {
				t.Errorf("BuildSnapshotArgs(%v) = %v, want %v",
					test.tags, got, test.want)
			}
		})
	}
}

func TestBuildForgetArgs_PolicyFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		tags   []string
		policy restic.ForgetPolicy
		want   []string
	}{
		{
			name: "all policy fields set",
			tags: []string{"web"},
			policy: restic.ForgetPolicy{
				KeepDaily:   7,
				KeepWeekly:  4,
				KeepMonthly: 12,
				KeepYearly:  3,
			},
			want: []string{
				"forget", "--prune", "--json",
				"--tag", "web",
				"--keep-daily", "7",
				"--keep-weekly", "4",
				"--keep-monthly", "12",
				"--keep-yearly", "3",
			},
		},
		{
			name: "some zero fields omitted",
			tags: []string{"db"},
			policy: restic.ForgetPolicy{
				KeepDaily:   7,
				KeepWeekly:  0,
				KeepMonthly: 6,
				KeepYearly:  0,
			},
			want: []string{
				"forget", "--prune", "--json",
				"--tag", "db",
				"--keep-daily", "7",
				"--keep-monthly", "6",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := restic.BuildForgetArgs(test.tags, test.policy)
			if !slices.Equal(got, test.want) {
				t.Errorf("BuildForgetArgs(%v, %+v) = %v, want %v",
					test.tags, test.policy, got, test.want)
			}
		})
	}
}

func TestBuildUnlockArgs(t *testing.T) {
	t.Parallel()

	got := restic.BuildUnlockArgs()
	want := []string{"unlock"}

	if !slices.Equal(got, want) {
		t.Errorf("BuildUnlockArgs() = %v, want %v", got, want)
	}
}

func TestBuildStatsArgs(t *testing.T) {
	t.Parallel()

	got := restic.BuildStatsArgs()
	want := []string{"stats", "--json"}

	if !slices.Equal(got, want) {
		t.Errorf("BuildStatsArgs() = %v, want %v", got, want)
	}
}

func TestBuildRestoreArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		snapshotID string
		targetPath string
		dryRun     bool
		want       []string
	}{
		{
			name:       "no dry-run",
			snapshotID: "abc",
			targetPath: "/tmp/r",
			dryRun:     false,
			want:       []string{"restore", "abc", "--target", "/tmp/r"},
		},
		{
			name:       "with dry-run",
			snapshotID: "abc",
			targetPath: "/tmp/r",
			dryRun:     true,
			want:       []string{"restore", "abc", "--target", "/tmp/r", "--dry-run"},
		},
		{
			name:       "target path with spaces",
			snapshotID: "deadbeef",
			targetPath: "/tmp/restore target",
			dryRun:     false,
			want:       []string{"restore", "deadbeef", "--target", "/tmp/restore target"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := restic.BuildRestoreArgs(test.snapshotID, test.targetPath, test.dryRun)
			if !slices.Equal(got, test.want) {
				t.Errorf("BuildRestoreArgs(%q, %q, %v) = %v, want %v",
					test.snapshotID, test.targetPath, test.dryRun, got, test.want)
			}
		})
	}
}

func TestBuildDumpArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		snapshotID string
		filename   string
		want       []string
	}{
		{
			name:       "simple filename",
			snapshotID: "abc",
			filename:   "dump.sql",
			want:       []string{"dump", "abc", "dump.sql"},
		},
		{
			name:       "absolute filename",
			snapshotID: "deadbeef",
			filename:   "/data/dump.sql",
			want:       []string{"dump", "deadbeef", "/data/dump.sql"},
		},
		{
			name:       "filename with spaces",
			snapshotID: "abc",
			filename:   "/data/my dump.sql",
			want:       []string{"dump", "abc", "/data/my dump.sql"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := restic.BuildDumpArgs(test.snapshotID, test.filename)
			if !slices.Equal(got, test.want) {
				t.Errorf("BuildDumpArgs(%q, %q) = %v, want %v",
					test.snapshotID, test.filename, got, test.want)
			}
		})
	}
}

func TestBuildBackupFromCommandArgs(t *testing.T) {
	t.Parallel()

	dumpCmd := []string{"docker", "exec", "mysql", "sh", "-c", "mysqldump"}
	prefix := func(filename string) []string {
		return []string{"backup", "--stdin-from-command", "--stdin-filename=" + filename}
	}

	tests := []struct {
		name     string
		filename string
		tags     []string
		args     []string
		want     []string
	}{
		{
			name: "no tags, no args", filename: "dump.sql", tags: nil, args: nil,
			want: append(prefix("dump.sql"), "--"),
		},
		{
			name: "no tags, with args", filename: "dump.sql", tags: nil, args: dumpCmd,
			want: append(append(prefix("dump.sql"), "--"), dumpCmd...),
		},
		{
			name: "one tag, with args", filename: "dump.sql", tags: []string{"a"}, args: dumpCmd,
			want: append(append(prefix("dump.sql"), "--tag", "a", "--"), dumpCmd...),
		},
		{
			name: "multi tags, with args", filename: "dump.sql",
			tags: []string{"w", "p"}, args: dumpCmd,
			want: append(append(prefix("dump.sql"), "--tag", "w,p", "--"), dumpCmd...),
		},
		{
			name: "multi tags, empty args", filename: "stream.bin",
			tags: []string{"w", "p"}, args: []string{},
			want: append(prefix("stream.bin"), "--tag", "w,p", "--"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := restic.BuildBackupFromCommandArgs(test.filename, test.tags, test.args)
			if !slices.Equal(got, test.want) {
				t.Errorf("BuildBackupFromCommandArgs(%q, %v, %v) = %v, want %v",
					test.filename, test.tags, test.args, got, test.want)
			}
		})
	}
}

func TestBuildForgetArgs_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		tags   []string
		policy restic.ForgetPolicy
		want   []string
	}{
		{
			name: "all zero",
			tags: []string{"app"},
			policy: restic.ForgetPolicy{
				KeepDaily:   0,
				KeepWeekly:  0,
				KeepMonthly: 0,
				KeepYearly:  0,
			},
			want: []string{
				"forget", "--prune", "--json",
				"--tag", "app",
			},
		},
		{
			name: "multiple tags",
			tags: []string{"web", "production"},
			policy: restic.ForgetPolicy{
				KeepDaily:   3,
				KeepWeekly:  0,
				KeepMonthly: 0,
				KeepYearly:  0,
			},
			want: []string{
				"forget", "--prune", "--json",
				"--tag", "web,production",
				"--keep-daily", "3",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := restic.BuildForgetArgs(test.tags, test.policy)
			if !slices.Equal(got, test.want) {
				t.Errorf("BuildForgetArgs(%v, %+v) = %v, want %v",
					test.tags, test.policy, got, test.want)
			}
		})
	}
}
