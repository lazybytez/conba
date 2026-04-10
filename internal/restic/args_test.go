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
				"--tag", "web", "--tag", "production",
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
			want: []string{"snapshots", "--json", "--tag", "web", "--tag", "production"},
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
				"--tag", "web", "--tag", "production",
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
