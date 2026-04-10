package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/lazybytez/conba/internal/config"
	"github.com/lazybytez/conba/internal/logging"
	"github.com/lazybytez/conba/internal/restic"
	"github.com/spf13/cobra"
)

var (
	errRepoNotInitialized = errors.New("repository not initialized")
	errRepoLocked         = errors.New("repository is locked")
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

	client := restic.New(cfg.Restic, logger)
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
	errMsg := err.Error()

	if strings.Contains(errMsg, "Is there a repository at the following location?") ||
		strings.Contains(errMsg, "unable to open config file") {
		writeErr := printNotInitialized(out, repo)
		if writeErr != nil {
			return writeErr
		}

		return errRepoNotInitialized
	}

	if strings.Contains(errMsg, "unable to create lock") ||
		strings.Contains(errMsg, "repository is already locked") {
		writeErr := printLocked(out, repo)
		if writeErr != nil {
			return writeErr
		}

		return errRepoLocked
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
		latestTime = latest.Time.Format("2006-01-02 15:04:05")
	}

	_, err := fmt.Fprintf(out,
		"Repository: %s\nStatus:     ready\nSnapshots:  %d\nLatest:     %s\nTotal size: %s\n",
		repo,
		len(snapshots),
		latestTime,
		formatSize(stats.TotalSize),
	)
	if err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	return nil
}

func formatSize(bytes uint64) string {
	const (
		kib = 1024
		mib = kib * 1024
		gib = mib * 1024
		tib = gib * 1024
	)

	switch {
	case bytes >= tib:
		return fmt.Sprintf("%.2f TiB", float64(bytes)/float64(tib))
	case bytes >= gib:
		return fmt.Sprintf("%.2f GiB", float64(bytes)/float64(gib))
	case bytes >= mib:
		return fmt.Sprintf("%.2f MiB", float64(bytes)/float64(mib))
	case bytes >= kib:
		return fmt.Sprintf("%.2f KiB", float64(bytes)/float64(kib))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
