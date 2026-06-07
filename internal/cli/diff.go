package cli

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/lazybytez/conba/internal/config"
	"github.com/lazybytez/conba/internal/logging"
	"github.com/lazybytez/conba/internal/report"
	"github.com/lazybytez/conba/internal/restic"
	"github.com/spf13/cobra"
)

// diffArgsCount is the exact number of positional snapshot identifiers
// `conba diff` accepts.
const diffArgsCount = 2

// Scanner buffer sizing for diff lines (paths can be long).
const (
	diffScanInitial = 64 * 1024
	diffScanMax     = 1024 * 1024
)

// NewDiffCommand creates the diff subcommand. It compares two snapshots
// and prints restic's diff output. Snapshot identifiers may be full IDs,
// short IDs, or the literal "latest".
func NewDiffCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "diff <snapshot-a> <snapshot-b>",
		Short: "Show differences between two snapshots",
		Args:  cobra.ExactArgs(diffArgsCount),
		RunE:  runDiff,
	}
}

func runDiff(cmd *cobra.Command, args []string) error {
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
	if reporter.Mode() == report.ModeJSON {
		return runDiffJSON(ctx, reporter, client, args[0], args[1])
	}

	raw, err := client.Diff(ctx, args[0], args[1])
	if err != nil {
		return fmt.Errorf("diff snapshots: %w", err)
	}

	return writeColoredDiff(reporter, raw)
}

func runDiffJSON(
	ctx context.Context,
	reporter report.Reporter,
	client *restic.Client,
	snapA, snapB string,
) error {
	changes, stats, err := client.DiffChanges(ctx, snapA, snapB)
	if err != nil {
		return fmt.Errorf("diff snapshots: %w", err)
	}

	for _, change := range changes {
		reporter.Emit(report.Event{
			Level: report.LevelInfo,
			Name:  "diff.change",
			Fields: []report.Field{
				report.F("path", change.Path),
				report.F("modifier", change.Modifier),
			},
		})
	}

	reporter.Emit(report.Event{
		Level: report.LevelInfo,
		Name:  "diff.summary",
		Fields: []report.Field{
			report.F("changed_files", stats.ChangedFiles),
			report.F("added_files", stats.Added.Files),
			report.F("added_bytes", stats.Added.Bytes),
			report.F("removed_files", stats.Removed.Files),
			report.F("removed_bytes", stats.Removed.Bytes),
		},
	})

	return nil
}

// writeColoredDiff writes restic's human diff, coloring added lines green
// and removed lines red when color is enabled. With color off the bytes are
// reproduced line by line unchanged.
func writeColoredDiff(reporter report.Reporter, raw []byte) error {
	out := reporter.Out()
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 0, diffScanInitial), diffScanMax)

	for scanner.Scan() {
		line := scanner.Text()

		switch {
		case strings.HasPrefix(line, "+"):
			line = reporter.Colorize(report.StyleSuccess, line)
		case strings.HasPrefix(line, "-"):
			line = reporter.Colorize(report.StyleError, line)
		}

		_, err := fmt.Fprintln(out, line)
		if err != nil {
			return fmt.Errorf("writing output: %w", err)
		}
	}

	err := scanner.Err()
	if err != nil {
		return fmt.Errorf("scan diff output: %w", err)
	}

	return nil
}
