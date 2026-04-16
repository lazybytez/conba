package cli

import (
	"io"

	"github.com/lazybytez/conba/internal/discovery"
	"github.com/lazybytez/conba/internal/filter"
	"github.com/lazybytez/conba/internal/restic"
	"github.com/spf13/pflag"
)

// Exported aliases for unexported functions, used by tests in cli_test package.
var (
	ShortID          = shortID
	GroupByContainer = groupByContainer
	PrintResult      = printResult
	PrintExcluded    = printExcluded

	PrintStatus         = printStatus
	PrintNotInitialized = printNotInitialized
	PrintLocked         = printLocked
	HandleStatusError   = handleStatusError

	PrintDryRun = printDryRun

	PrintSnapshots      = printSnapshots
	ExtractTag          = extractTag
	BuildFilterTags     = buildFilterTags
	ReadSnapshotFilters = readSnapshotFilters
)

// SnapshotFilters exposes the unexported snapshotFilters struct for tests.
type SnapshotFilters = snapshotFilters

// SnapshotFiltersTags exposes snapshotFilters.tags() for tests.
func SnapshotFiltersTags(f SnapshotFilters) []string { return f.tags() }

// Ensure function signatures stay in sync with aliases.
var (
	_ func(string) string                           = shortID
	_ func([]discovery.Target) [][]discovery.Target = groupByContainer
	_ func(io.Writer, filter.Result) error          = printResult
	_ func(io.Writer, []filter.Exclusion) error     = printExcluded

	_ func(io.Writer, string, []restic.Snapshot, restic.RepoStats) error = printStatus
	_ func(io.Writer, string) error                                      = printNotInitialized
	_ func(io.Writer, string) error                                      = printLocked
	_ func(io.Writer, string, error) error                               = handleStatusError

	_ func(io.Writer, []discovery.Target) error = printDryRun

	_ func(io.Writer, []restic.Snapshot) error = printSnapshots
	_ func([]string, string) string            = extractTag
	_ func(string, string, string) []string    = buildFilterTags
	_ func(*pflag.FlagSet) snapshotFilters     = readSnapshotFilters
)
