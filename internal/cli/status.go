package cli

import (
	"errors"
	"fmt"
	"io"

	"github.com/lazybytez/conba/internal/config"
	"github.com/lazybytez/conba/internal/logging"
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

	out := cmd.OutOrStdout()

	snapshots, err := client.Snapshots(ctx, nil)
	if err != nil {
		return handleStatusError(out, cfg.Restic.Repository, err)
	}

	stats, err := client.Stats(ctx)
	if err != nil {
		return fmt.Errorf("get repository stats: %w", err)
	}

	return printStatus(out, cfg.Restic.Repository, snapshots, stats)
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
