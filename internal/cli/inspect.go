package cli

import (
	"fmt"
	"io"

	"github.com/lazybytez/conba/internal/config"
	"github.com/lazybytez/conba/internal/discovery"
	"github.com/lazybytez/conba/internal/filter"
	"github.com/lazybytez/conba/internal/logging"
	"github.com/lazybytez/conba/internal/runtime/docker"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// NewInspectCommand creates the inspect subcommand that lists all
// discovered containers and volumes with filter results.
func NewInspectCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "inspect",
		Short: "Show containers and volumes that would be backed up",
		RunE:  runInspect,
	}
}

func runInspect(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	cfg := config.FromContext(ctx)
	logger := logging.FromContext(ctx)

	if cfg == nil {
		return errMissingConfig
	}

	logger.Debug("connecting to docker",
		zap.String("host", cfg.Runtime.Docker.Host))

	runtime, err := docker.New(ctx, cfg.Runtime.Docker.Host)
	if err != nil {
		return fmt.Errorf("connect to docker: %w", err)
	}

	defer func() {
		closeErr := runtime.Close()
		if closeErr != nil {
			logger.Warn("failed to close docker client",
				zap.Error(closeErr))
		}
	}()

	targets, err := discovery.Discover(ctx, runtime)
	if err != nil {
		return fmt.Errorf("discover volumes: %w", err)
	}

	result := filter.Apply(targets, cfg.Discovery)

	return printResultWithFeatureFlag(
		cmd.OutOrStdout(), result, cfg.PreBackupCommands.Enabled,
	)
}

// printResult is the convenience entry point for callers that do not
// construct a config; it renders as if the feature were enabled.
func printResult(out io.Writer, result filter.Result) error {
	return printResultWithFeatureFlag(out, result, true)
}

func printResultWithFeatureFlag(
	out io.Writer,
	result filter.Result,
	preBackupEnabled bool,
) error {
	if len(result.Included) == 0 && len(result.Excluded) == 0 {
		_, err := fmt.Fprintln(out, "No containers with volumes found.")
		if err != nil {
			return fmt.Errorf("writing output: %w", err)
		}

		return nil
	}

	if len(result.Included) > 0 {
		err := printIncludedSection(out, result.Included, preBackupEnabled)
		if err != nil {
			return err
		}
	}

	if len(result.Excluded) > 0 {
		err := printExcludedSection(out, result.Excluded)
		if err != nil {
			return err
		}
	}

	return nil
}

func printIncludedSection(
	out io.Writer,
	included []discovery.Target,
	preBackupEnabled bool,
) error {
	_, err := fmt.Fprintf(out, "=== Included ===\n\n")
	if err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	return printIncluded(out, included, preBackupEnabled)
}

func printExcludedSection(out io.Writer, excluded []filter.Exclusion) error {
	_, err := fmt.Fprintf(out, "=== Excluded ===\n\n")
	if err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	return printExcluded(out, excluded)
}

func printIncluded(
	out io.Writer,
	targets []discovery.Target,
	preBackupEnabled bool,
) error {
	grouped := groupByContainer(targets)

	for _, group := range grouped {
		first := group[0]

		_, err := fmt.Fprintf(out, "%s (%s)\n",
			first.Container.Name,
			shortID(first.Container.ID))
		if err != nil {
			return fmt.Errorf("writing output: %w", err)
		}

		for _, target := range group {
			_, err = fmt.Fprintf(out, "  %s  %s → %s\n",
				target.Mount.Type,
				target.Mount.Name,
				target.Mount.Destination)
			if err != nil {
				return fmt.Errorf("writing output: %w", err)
			}
		}

		err = printPreBackupDetails(out, first, preBackupEnabled)
		if err != nil {
			return err
		}

		_, err = fmt.Fprintln(out)
		if err != nil {
			return fmt.Errorf("writing output: %w", err)
		}
	}

	return nil
}

// printPreBackupDetails renders the verbose pre-backup subsection for a
// container. It emits nothing when the container carries no pre-backup
// labels, and an "invalid" marker line when the labels fail to parse.
// When preBackupEnabled is false, the section header gets a marker noting
// the feature flag is off so operators understand the labels are dormant.
func printPreBackupDetails(
	out io.Writer,
	target discovery.Target,
	preBackupEnabled bool,
) error {
	spec, hasSpec, err := filter.PreBackup(target)
	if err != nil {
		rawMode := target.Container.Labels[filter.LabelPreBackupMode]

		_, writeErr := fmt.Fprintf(out, "  pre-backup: invalid (mode=%s)\n", rawMode)
		if writeErr != nil {
			return fmt.Errorf("writing output: %w", writeErr)
		}

		return nil
	}

	if !hasSpec {
		return nil
	}

	execContainer := spec.Container
	if execContainer == "" {
		execContainer = target.Container.Name
	}

	filename := spec.Filename
	if filename == "" {
		filename = target.Container.Name
	}

	header := "  pre-backup:\n"
	if !preBackupEnabled {
		header = "  pre-backup: (disabled, pre_backup_commands.enabled is false)\n"
	}

	_, err = fmt.Fprintf(
		out,
		header+
			"    command:   %s\n"+
			"    mode:      %s\n"+
			"    exec:      %s\n"+
			"    filename:  %s\n",
		spec.Command,
		spec.Mode,
		execContainer,
		filename,
	)
	if err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	return nil
}

func printExcluded(out io.Writer, exclusions []filter.Exclusion) error {
	for _, excl := range exclusions {
		_, err := fmt.Fprintf(out, "%s (%s)  %s → %s\n  reason: %s\n\n",
			excl.Target.Container.Name,
			shortID(excl.Target.Container.ID),
			excl.Target.Mount.Type,
			excl.Target.Mount.Name,
			excl.Reason)
		if err != nil {
			return fmt.Errorf("writing output: %w", err)
		}
	}

	return nil
}

func shortID(containerID string) string {
	const maxLen = 12
	if len(containerID) <= maxLen {
		return containerID
	}

	return containerID[:maxLen]
}

func groupByContainer(targets []discovery.Target) [][]discovery.Target {
	var (
		groups [][]discovery.Target
		index  = make(map[string]int)
	)

	for _, target := range targets {
		idx, exists := index[target.Container.ID]
		if !exists {
			idx = len(groups)
			index[target.Container.ID] = idx

			groups = append(groups, nil)
		}

		groups[idx] = append(groups[idx], target)
	}

	return groups
}
