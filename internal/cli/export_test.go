package cli

import (
	"io"

	"github.com/lazybytez/conba/internal/discovery"
	"github.com/lazybytez/conba/internal/filter"
)

// Exported aliases for unexported functions, used by tests in cli_test package.
var (
	ShortID          = shortID
	GroupByContainer = groupByContainer
	PrintResult      = printResult
	PrintExcluded    = printExcluded
)

// Ensure function signatures stay in sync with aliases.
var (
	_ func(string) string                           = shortID
	_ func([]discovery.Target) [][]discovery.Target = groupByContainer
	_ func(io.Writer, filter.Result) error          = printResult
	_ func(io.Writer, []filter.Exclusion) error     = printExcluded
)
