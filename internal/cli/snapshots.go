package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/lazybytez/conba/internal/config"
	"github.com/lazybytez/conba/internal/logging"
	"github.com/lazybytez/conba/internal/restic"
	"github.com/spf13/cobra"
)

// NewSnapshotsCommand creates the snapshots subcommand that lists backup snapshots.
func NewSnapshotsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "snapshots",
		Short: "List backup snapshots",
		RunE:  runSnapshots,
	}

	cmd.Flags().String("container", "", "filter by container name")
	cmd.Flags().String("volume", "", "filter by volume name")
	cmd.Flags().String("hostname", "", "filter by hostname")

	return cmd
}

func runSnapshots(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	cfg := config.FromContext(ctx)
	logger := logging.FromContext(ctx)

	if cfg == nil {
		return errMissingConfig
	}

	container, err := cmd.Flags().GetString("container")
	if err != nil {
		return fmt.Errorf("reading container flag: %w", err)
	}

	volume, err := cmd.Flags().GetString("volume")
	if err != nil {
		return fmt.Errorf("reading volume flag: %w", err)
	}

	hostname, err := cmd.Flags().GetString("hostname")
	if err != nil {
		return fmt.Errorf("reading hostname flag: %w", err)
	}

	tags := buildFilterTags(container, volume, hostname)

	client, err := restic.New(cfg.Restic, logger)
	if err != nil {
		return fmt.Errorf("create restic client: %w", err)
	}

	snapshots, err := client.Snapshots(ctx, tags)
	if err != nil {
		return fmt.Errorf("list snapshots: %w", err)
	}

	out := cmd.OutOrStdout()

	if len(snapshots) == 0 {
		_, printErr := fmt.Fprintln(out, "No snapshots found.")
		if printErr != nil {
			return fmt.Errorf("writing output: %w", printErr)
		}

		return nil
	}

	return printSnapshots(out, snapshots)
}

func printSnapshots(out io.Writer, snapshots []restic.Snapshot) error {
	const rowFmt = "%-10s  %-20s  %-20s  %-25s  %s\n"

	_, err := fmt.Fprintf(out, rowFmt, "ID", "Time", "Container", "Volume", "Hostname")
	if err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	_, err = fmt.Fprintf(out, rowFmt, "----------", "--------------------",
		"--------------------", "-------------------------", "--------")
	if err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	for _, snap := range snapshots {
		_, err = fmt.Fprintf(out, rowFmt,
			snap.ID,
			snap.Time.UTC().Format("2006-01-02 15:04:05"),
			extractTag(snap.Tags, "container="),
			extractTag(snap.Tags, "volume="),
			extractTag(snap.Tags, "hostname="),
		)
		if err != nil {
			return fmt.Errorf("writing output: %w", err)
		}
	}

	_, err = fmt.Fprintf(out, "\n%d snapshot(s)\n", len(snapshots))
	if err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	return nil
}

func extractTag(tags []string, prefix string) string {
	for _, tag := range tags {
		if strings.HasPrefix(tag, prefix) {
			return tag[len(prefix):]
		}
	}

	return "-"
}

func buildFilterTags(container, volume, hostname string) []string {
	var tags []string

	if container != "" {
		tags = append(tags, "container="+container)
	}

	if volume != "" {
		tags = append(tags, "volume="+volume)
	}

	if hostname != "" {
		tags = append(tags, "hostname="+hostname)
	}

	return tags
}
