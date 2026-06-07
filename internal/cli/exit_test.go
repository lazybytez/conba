package cli_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/lazybytez/conba/internal/backup"
	"github.com/lazybytez/conba/internal/cli"
	"github.com/lazybytez/conba/internal/config"
	"github.com/lazybytez/conba/internal/forget"
	"github.com/lazybytez/conba/internal/restic"
)

var errUnexpected = errors.New("something unexpected")

type exitCodeCase struct {
	name string
	err  error
	want int
}

func runExitCodeCases(t *testing.T, cases []exitCodeCase) {
	t.Helper()

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := cli.ExitCode(test.err)
			if got != test.want {
				t.Errorf("ExitCode(%v) = %d, want %d", test.err, got, test.want)
			}
		})
	}
}

func TestExitCode_TargetFailures(t *testing.T) {
	t.Parallel()

	runExitCodeCases(t, []exitCodeCase{
		{
			name: "backup partial failure",
			err:  &backup.RunError{Succeeded: 1, Skipped: 0, Failed: 1},
			want: 2,
		},
		{
			name: "backup total failure",
			err:  &backup.RunError{Succeeded: 0, Skipped: 0, Failed: 2},
			want: 3,
		},
		{
			name: "forget partial failure",
			err:  &forget.RunError{Succeeded: 2, Skipped: 1, Failed: 1},
			want: 2,
		},
		{
			name: "wrapped backup error stays classified",
			err: fmt.Errorf(
				"run backup: %w", &backup.RunError{Succeeded: 3, Skipped: 0, Failed: 1},
			),
			want: 2,
		},
	})
}

func TestExitCode_Preconditions(t *testing.T) {
	t.Parallel()

	runExitCodeCases(t, []exitCodeCase{
		{
			name: "repo locked",
			err:  fmt.Errorf("check: %w", restic.ErrRepoLocked),
			want: 1,
		},
		{
			name: "missing repository",
			err:  fmt.Errorf("load config: %w", config.ErrMissingRepository),
			want: 1,
		},
		{
			name: "invalid output format",
			err:  fmt.Errorf("load config: %w", config.ErrInvalidOutputFormat),
			want: 1,
		},
		{
			name: "unclassified error is fatal",
			err:  errUnexpected,
			want: 3,
		},
	})
}
