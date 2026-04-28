package cli

import (
	"fmt"

	"github.com/lazybytez/conba/internal/config"
	"github.com/lazybytez/conba/internal/logging"
	"github.com/lazybytez/conba/internal/restic"
	"github.com/spf13/cobra"
)

// diffArgsCount is the exact number of positional snapshot identifiers
// `conba diff` accepts.
const diffArgsCount = 2

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

	out, err := client.Diff(ctx, args[0], args[1])
	if err != nil {
		return fmt.Errorf("diff snapshots: %w", err)
	}

	_, err = cmd.OutOrStdout().Write(out)
	if err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	return nil
}
