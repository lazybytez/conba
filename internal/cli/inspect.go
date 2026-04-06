package cli

import (
	"errors"
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

var errMissingConfig = errors.New("config not available in context")

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

	return printResult(cmd.OutOrStdout(), result)
}

func printResult(out io.Writer, result filter.Result) error {
	if len(result.Included) == 0 && len(result.Excluded) == 0 {
		_, err := fmt.Fprintln(out, "No containers with volumes found.")
		if err != nil {
			return fmt.Errorf("writing output: %w", err)
		}

		return nil
	}

	if len(result.Included) > 0 {
		err := printSection(out, "Included", result.Included, nil)
		if err != nil {
			return err
		}
	}

	if len(result.Excluded) > 0 {
		err := printSection(out, "Excluded", nil, result.Excluded)
		if err != nil {
			return err
		}
	}

	return nil
}

func printSection(
	out io.Writer,
	title string,
	included []discovery.Target,
	excluded []filter.Exclusion,
) error {
	_, err := fmt.Fprintf(out, "=== %s ===\n\n", title)
	if err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	if len(included) > 0 {
		return printIncluded(out, included)
	}

	return printExcluded(out, excluded)
}

func printIncluded(out io.Writer, targets []discovery.Target) error {
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

		_, err = fmt.Fprintln(out)
		if err != nil {
			return fmt.Errorf("writing output: %w", err)
		}
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
