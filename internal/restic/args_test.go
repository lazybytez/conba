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
				"forget", "--json",
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
				"forget", "--json",
				"--tag", "db",
				"--keep-daily", "7",
				"--keep-monthly", "6",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := restic.BuildForgetArgs(test.tags, test.policy, restic.ForgetOptions{
				Prune:  false,
				DryRun: false,
			})
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

func TestBuildCheckArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		readData bool
		want     []string
	}{
		{name: "default", readData: false, want: []string{"check"}},
		{name: "with read-data", readData: true, want: []string{"check", "--read-data"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := restic.BuildCheckArgs(test.readData)
			if !slices.Equal(got, test.want) {
				t.Errorf("BuildCheckArgs(%v) = %v, want %v",
					test.readData, got, test.want)
			}
		})
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

func TestBuildBackupFromStdinArgs(t *testing.T) {
	t.Parallel()

	prefix := func(filename string) []string {
		return []string{"backup", "--stdin", "--stdin-filename=" + filename}
	}

	tests := []struct {
		name     string
		filename string
		tags     []string
		want     []string
	}{
		{
			name:     "no tags",
			filename: "dump.sql",
			tags:     nil,
			want:     prefix("dump.sql"),
		},
		{
			name:     "one tag",
			filename: "dump.sql",
			tags:     []string{"a"},
			want:     append(prefix("dump.sql"), "--tag", "a"),
		},
		{
			name:     "multiple tags",
			filename: "stream.bin",
			tags:     []string{"web", "production"},
			want:     append(prefix("stream.bin"), "--tag", "web,production"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := restic.BuildBackupFromStdinArgs(test.filename, test.tags)
			if !slices.Equal(got, test.want) {
				t.Errorf("BuildBackupFromStdinArgs(%q, %v) = %v, want %v",
					test.filename, test.tags, got, test.want)
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
				"forget", "--json",
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
				"forget", "--json",
				"--tag", "web,production",
				"--keep-daily", "3",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := restic.BuildForgetArgs(test.tags, test.policy, restic.ForgetOptions{
				Prune:  false,
				DryRun: false,
			})
			if !slices.Equal(got, test.want) {
				t.Errorf("BuildForgetArgs(%v, %+v) = %v, want %v",
					test.tags, test.policy, got, test.want)
			}
		})
	}
}

func TestBuildForgetArgs_PruneFlag(t *testing.T) {
	t.Parallel()

	policy := restic.ForgetPolicy{KeepDaily: 1, KeepWeekly: 0, KeepMonthly: 0, KeepYearly: 0}
	tags := []string{"web"}

	tests := []struct {
		name      string
		opts      restic.ForgetOptions
		wantFlag  []string
		notWanted []string
	}{
		{
			name:      "prune true appends --prune",
			opts:      restic.ForgetOptions{Prune: true, DryRun: false},
			wantFlag:  []string{"--prune"},
			notWanted: []string{"--dry-run"},
		},
		{
			name:      "prune false omits --prune",
			opts:      restic.ForgetOptions{Prune: false, DryRun: false},
			wantFlag:  nil,
			notWanted: []string{"--prune", "--dry-run"},
		},
		{
			name:      "dry-run true appends --dry-run",
			opts:      restic.ForgetOptions{Prune: false, DryRun: true},
			wantFlag:  []string{"--dry-run"},
			notWanted: []string{"--prune"},
		},
		{
			name:      "dry-run false omits --dry-run",
			opts:      restic.ForgetOptions{Prune: false, DryRun: false},
			wantFlag:  nil,
			notWanted: []string{"--prune", "--dry-run"},
		},
		{
			name:      "both true appends --prune and --dry-run",
			opts:      restic.ForgetOptions{Prune: true, DryRun: true},
			wantFlag:  []string{"--prune", "--dry-run"},
			notWanted: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := restic.BuildForgetArgs(tags, policy, test.opts)
			assertFlagPresence(t, got, test.wantFlag, test.notWanted)
		})
	}
}

func assertFlagPresence(t *testing.T, got, want, notWanted []string) {
	t.Helper()

	for _, flag := range want {
		if !slices.Contains(got, flag) {
			t.Errorf("BuildForgetArgs(...) = %v, expected to contain %q", got, flag)
		}
	}

	for _, flag := range notWanted {
		if slices.Contains(got, flag) {
			t.Errorf("BuildForgetArgs(...) = %v, expected NOT to contain %q", got, flag)
		}
	}
}

func TestBuildDiffArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		snapA  string
		snapB  string
		asJSON bool
		want   []string
	}{
		{
			name: "two short ids", snapA: "abc", snapB: "def", asJSON: false,
			want: []string{"diff", "abc", "def"},
		},
		{
			name: "passthrough verbatim", snapA: "first-id", snapB: "latest", asJSON: false,
			want: []string{"diff", "first-id", "latest"},
		},
		{
			name: "json adds flag", snapA: "abc", snapB: "def", asJSON: true,
			want: []string{"diff", "--json", "abc", "def"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := restic.BuildDiffArgs(test.snapA, test.snapB, test.asJSON)
			if !slices.Equal(got, test.want) {
				t.Errorf("BuildDiffArgs(%q, %q, %v) = %v, want %v",
					test.snapA, test.snapB, test.asJSON, got, test.want)
			}
		})
	}
}
