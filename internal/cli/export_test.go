package cli

import (
	"io"

	"github.com/lazybytez/conba/internal/config"
	"github.com/lazybytez/conba/internal/discovery"
	"github.com/lazybytez/conba/internal/filter"
	"github.com/lazybytez/conba/internal/restic"
)

// Exported aliases for unexported functions, used by tests in cli_test package.
var (
	ShortID          = shortID
	GroupByContainer = groupByContainer
	PrintResult      = printResult
	PrintExcluded    = printExcluded

	FormatSize          = formatSize
	PrintStatus         = printStatus
	PrintNotInitialized = printNotInitialized
	PrintLocked         = printLocked
	HandleStatusError   = handleStatusError

	ErrMissingRepository = errMissingRepository
	ErrMissingPassword   = errMissingPassword

	RequireResticConfig = requireResticConfig
)

// Ensure function signatures stay in sync with aliases.
var (
	_ func(string) string                           = shortID
	_ func([]discovery.Target) [][]discovery.Target = groupByContainer
	_ func(io.Writer, filter.Result) error          = printResult
	_ func(io.Writer, []filter.Exclusion) error     = printExcluded

	_ func(uint64) string                                                = formatSize
	_ func(io.Writer, string, []restic.Snapshot, restic.RepoStats) error = printStatus
	_ func(io.Writer, string) error                                      = printNotInitialized
	_ func(io.Writer, string) error                                      = printLocked
	_ func(io.Writer, string, error) error                               = handleStatusError
	_ func(config.ResticConfig) error                                    = requireResticConfig
)
