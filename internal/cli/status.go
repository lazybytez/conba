package cli

import (
	"errors"
	"fmt"
	"io"

	"github.com/lazybytez/conba/internal/config"
	"github.com/lazybytez/conba/internal/logging"
	"github.com/lazybytez/conba/internal/report"
	"github.com/lazybytez/conba/internal/restic"
	"github.com/lazybytez/conba/internal/support/format"
	"github.com/spf13/cobra"
)

// NewStatusCommand creates the status subcommand that shows
// repository status and statistics.
func NewStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show repository status and statistics",
		RunE:  runStatus,
	}
}

func runStatus(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	cfg := config.FromContext(ctx)
	logger := logging.FromContext(ctx)

	if cfg == nil {
		return errMissingConfig
	}

	client, err := restic.New(cfg.Restic, logger)
	if err != nil {
		return fmt.Errorf("create restic client: %w", err)
	}

	reporter := report.FromContext(ctx)

	snapshots, err := client.Snapshots(ctx, nil)
	if err != nil {
		return reportStatusError(reporter, cfg.Restic.Repository, err)
	}

	stats, err := client.Stats(ctx)
	if err != nil {
		return fmt.Errorf("get repository stats: %w", err)
	}

	if reporter.Mode() == report.ModeJSON {
		emitRepoStatus(reporter, cfg.Restic.Repository, "ok",
			report.F("snapshots", len(snapshots)),
			report.F("latest", latestSnapshotTime(snapshots)),
			report.F("total_size", stats.TotalSize))

		return nil
	}

	return printStatus(reporter.Out(), cfg.Restic.Repository, snapshots, stats)
}

// reportStatusError renders a not-initialized or locked repository as a
// repo.status event (json) or the human status block (text), returning nil;
// unrecognized errors are wrapped and returned.
func reportStatusError(reporter report.Reporter, repo string, err error) error {
	if reporter.Mode() != report.ModeJSON {
		return handleStatusError(reporter.Out(), repo, err)
	}

	classified := restic.ClassifyError(err)

	switch {
	case errors.Is(classified, restic.ErrRepoNotInitialized):
		emitRepoStatus(reporter, repo, "not_initialized")
	case errors.Is(classified, restic.ErrRepoLocked):
		emitRepoStatus(reporter, repo, "locked")
	default:
		return fmt.Errorf("check repository: %w", err)
	}

	return nil
}

// emitRepoStatus emits a single repo.status event with repository and state
// plus any extra fields.
func emitRepoStatus(reporter report.Reporter, repo, state string, extra ...report.Field) {
	fields := append([]report.Field{
		report.F("repository", repo),
		report.F("state", state),
	}, extra...)

	reporter.Emit(report.Event{Level: report.LevelInfo, Name: "repo.status", Fields: fields})
}

// latestSnapshotTime returns the formatted time of the most recent snapshot,
// or "n/a" when there are none.
func latestSnapshotTime(snapshots []restic.Snapshot) string {
	if len(snapshots) == 0 {
		return "n/a"
	}

	return format.Time(snapshots[len(snapshots)-1].Time)
}

func handleStatusError(out io.Writer, repo string, err error) error {
	classified := restic.ClassifyError(err)

	if errors.Is(classified, restic.ErrRepoNotInitialized) {
		return printNotInitialized(out, repo)
	}

	if errors.Is(classified, restic.ErrRepoLocked) {
		return printLocked(out, repo)
	}

	return fmt.Errorf("check repository: %w", err)
}

func printNotInitialized(out io.Writer, repo string) error {
	_, err := fmt.Fprintf(out,
		"Repository: %s\nStatus:     not initialized (run 'conba init' to create)\n",
		repo)
	if err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	return nil
}

func printLocked(out io.Writer, repo string) error {
	_, err := fmt.Fprintf(out,
		"Repository: %s\nStatus:     locked (run 'conba unlock' to remove stale locks)\n",
		repo)
	if err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	return nil
}

func printStatus(
	out io.Writer,
	repo string,
	snapshots []restic.Snapshot,
	stats restic.RepoStats,
) error {
	latestTime := "n/a"

	if len(snapshots) > 0 {
		latest := snapshots[len(snapshots)-1]
		latestTime = format.Time(latest.Time)
	}

	_, err := fmt.Fprintf(out,
		"Repository: %s\nStatus:     ready\nSnapshots:  %d\nLatest:     %s\nTotal size: %s\n",
		repo,
		len(snapshots),
		latestTime,
		format.Bytes(stats.TotalSize),
	)
	if err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	return nil
}
