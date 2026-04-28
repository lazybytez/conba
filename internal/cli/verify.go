package cli

import (
	"fmt"

	"github.com/lazybytez/conba/internal/config"
	"github.com/lazybytez/conba/internal/logging"
	"github.com/lazybytez/conba/internal/restic"
	"github.com/spf13/cobra"
)

// NewVerifyCommand creates the verify subcommand that checks restic
// repository integrity. By default it verifies repository structure;
// --read-data extends the check to all data blobs (slow but exhaustive).
func NewVerifyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify restic repository integrity",
		RunE:  runVerify,
	}

	cmd.Flags().Bool("read-data", false,
		"verify all data blobs (slow; full data read)")

	return cmd
}

func runVerify(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	cfg := config.FromContext(ctx)
	logger := logging.FromContext(ctx)

	if cfg == nil {
		return errMissingConfig
	}

	readData, err := cmd.Flags().GetBool("read-data")
	if err != nil {
		return fmt.Errorf("read --read-data flag: %w", err)
	}

	client, err := restic.New(cfg.Restic, logger)
	if err != nil {
		return fmt.Errorf("create restic client: %w", err)
	}

	err = client.Check(ctx, readData)
	if err != nil {
		return fmt.Errorf("verify repository: %w", err)
	}

	_, err = fmt.Fprintln(cmd.OutOrStdout(), "Repository verified.")
	if err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	return nil
}
