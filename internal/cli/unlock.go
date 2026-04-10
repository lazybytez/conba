package cli

import (
	"fmt"

	"github.com/lazybytez/conba/internal/config"
	"github.com/lazybytez/conba/internal/logging"
	"github.com/lazybytez/conba/internal/restic"
	"github.com/spf13/cobra"
)

// NewUnlockCommand creates the unlock subcommand that removes
// stale locks from the restic repository.
func NewUnlockCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "unlock",
		Short: "Remove stale locks from the restic repository",
		RunE:  runUnlock,
	}
}

func runUnlock(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	cfg := config.FromContext(ctx)
	logger := logging.FromContext(ctx)

	if cfg == nil {
		return errMissingConfig
	}

	err := requireResticConfig(cfg.Restic)
	if err != nil {
		return err
	}

	client := restic.New(cfg.Restic, logger)

	err = client.Unlock(ctx)
	if err != nil {
		return fmt.Errorf("unlock repository: %w", err)
	}

	_, err = fmt.Fprintln(cmd.OutOrStdout(), "Repository unlocked.")
	if err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	return nil
}
