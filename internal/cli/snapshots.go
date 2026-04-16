package cli

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/lazybytez/conba/internal/config"
	"github.com/lazybytez/conba/internal/logging"
	"github.com/lazybytez/conba/internal/restic"
	"github.com/lazybytez/conba/internal/support/format"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	tagPrefixContainer = "container="
	tagPrefixVolume    = "volume="
	tagPrefixHostname  = "hostname="

	tablePadding = 2
)

// snapshotFilters holds the CLI-provided filters for the snapshots command.
type snapshotFilters struct {
	container string
	volume    string
	hostname  string
}

// tags returns the restic tag filters derived from the user-provided flags.
// An empty field contributes nothing; the returned slice is AND-combined
// by the caller when handed to restic.
func (f snapshotFilters) tags() []string {
	return buildFilterTags(f.container, f.volume, f.hostname)
}

// NewSnapshotsCommand creates the snapshots subcommand that lists backup
// snapshots, optionally filtered by container, volume, or hostname tags.
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

// runSnapshots is the cobra RunE that lists snapshots from the configured
// restic repository, filtered by the command's flags.
func runSnapshots(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	cfg := config.FromContext(ctx)
	logger := logging.FromContext(ctx)

	if cfg == nil {
		return errMissingConfig
	}

	filters := readSnapshotFilters(cmd.Flags())

	client, err := restic.New(cfg.Restic, logger)
	if err != nil {
		return fmt.Errorf("create restic client: %w", err)
	}

	snapshots, err := client.Snapshots(ctx, filters.tags())
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

// readSnapshotFilters reads the user-provided filter flags into a struct.
func readSnapshotFilters(flags *pflag.FlagSet) snapshotFilters {
	return snapshotFilters{
		container: flagString(flags, "container"),
		volume:    flagString(flags, "volume"),
		hostname:  flagString(flags, "hostname"),
	}
}

// flagString returns the value of a string flag, silently yielding "" when
// the flag is missing or of the wrong type.
func flagString(flags *pflag.FlagSet, name string) string {
	value, _ := flags.GetString(name)

	return value
}

// printSnapshots writes a tabular listing of snapshots followed by a
// summary line with the total count.
func printSnapshots(out io.Writer, snapshots []restic.Snapshot) error {
	table := tabwriter.NewWriter(out, 0, 0, tablePadding, ' ', 0)

	_, err := fmt.Fprintln(table, "ID\tTime\tContainer\tVolume\tHostname")
	if err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	for _, snap := range snapshots {
		_, err = fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\n",
			snap.ID,
			format.Time(snap.Time.UTC()),
			extractTag(snap.Tags, tagPrefixContainer),
			extractTag(snap.Tags, tagPrefixVolume),
			extractTag(snap.Tags, tagPrefixHostname),
		)
		if err != nil {
			return fmt.Errorf("writing output: %w", err)
		}
	}

	err = table.Flush()
	if err != nil {
		return fmt.Errorf("flushing output: %w", err)
	}

	_, err = fmt.Fprintln(out)
	if err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	_, err = fmt.Fprintf(out, "%d snapshot(s)\n", len(snapshots))
	if err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	return nil
}

// buildFilterTags converts non-empty filter values into restic tag
// arguments of the form "key=value".
func buildFilterTags(container, volume, hostname string) []string {
	var tags []string

	if container != "" {
		tags = append(tags, tagPrefixContainer+container)
	}

	if volume != "" {
		tags = append(tags, tagPrefixVolume+volume)
	}

	if hostname != "" {
		tags = append(tags, tagPrefixHostname+hostname)
	}

	return tags
}

// extractTag returns the value portion of the first tag whose key matches
// prefix, or "-" if no such tag is present.
func extractTag(tags []string, prefix string) string {
	for _, tag := range tags {
		if strings.HasPrefix(tag, prefix) {
			return tag[len(prefix):]
		}
	}

	return "-"
}
