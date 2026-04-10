package cli

import (
	"fmt"

	"github.com/lazybytez/conba/internal/config"
	"github.com/lazybytez/conba/internal/logging"
	"github.com/lazybytez/conba/internal/restic"
	"github.com/spf13/cobra"
)

// NewInitCommand creates the init subcommand that initialises
// the restic repository.
func NewInitCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize the restic repository",
		RunE:  runInit,
	}
}

func runInit(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	cfg := config.FromContext(ctx)
	logger := logging.FromContext(ctx)

	if cfg == nil {
		return errMissingConfig
	}

	err := cfg.Restic.Validate()
	if err != nil {
		return fmt.Errorf("invalid restic config: %w", err)
	}

	client := restic.New(cfg.Restic, logger)

	err = client.Init(ctx)
	if err != nil {
		return fmt.Errorf("init repository: %w", err)
	}

	_, err = fmt.Fprintln(cmd.OutOrStdout(), "Repository initialized.")
	if err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	return nil
}
