// Package forget implements snapshot retention orchestration on
// top of the restic.Forget wrapper. It resolves per-container
// retention policies (label or global), iterates targets, and
// reports per-target outcomes.
package forget

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/lazybytez/conba/internal/config"
	"github.com/lazybytez/conba/internal/discovery"
	"github.com/lazybytez/conba/internal/filter"
)

// ErrInvalidRetentionLabel indicates the conba.retention container
// label could not be parsed. Errors returned by ParseRetentionLabel
// and Resolve wrap this sentinel so callers can identify them via
// errors.Is.
var ErrInvalidRetentionLabel = errors.New("invalid retention label")

// Resolution identifies which source provided the retention policy
// returned by Resolve. It is a string alias so it formats cleanly
// in log lines.
type Resolution string

// Resolution values reported by Resolve.
const (
	ResolutionLabel  Resolution = "label"
	ResolutionGlobal Resolution = "global"
	ResolutionNone   Resolution = "none"
)

// ParseRetentionLabel parses the conba.retention label syntax into a
// RetentionConfig. The syntax is comma-separated, suffix-tagged,
// order-agnostic, case-insensitive on suffix, and whitespace-tolerant.
// Suffixes: d (daily), w (weekly), m (monthly), y (yearly).
// An empty input string returns the zero RetentionConfig with no error.
// All errors wrap ErrInvalidRetentionLabel.
func ParseRetentionLabel(raw string) (config.RetentionConfig, error) {
	var cfg config.RetentionConfig

	if raw == "" {
		return cfg, nil
	}

	seen := map[byte]bool{}

	for entry := range strings.SplitSeq(raw, ",") {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" {
			continue
		}

		err := applyEntry(&cfg, trimmed, seen)
		if err != nil {
			return config.RetentionConfig{}, err
		}
	}

	return cfg, nil
}

// minEntryLen is the smallest valid retention entry: one digit plus a suffix.
const minEntryLen = 2

func applyEntry(cfg *config.RetentionConfig, entry string, seen map[byte]bool) error {
	if len(entry) < minEntryLen {
		return fmt.Errorf("%w: missing suffix: %q", ErrInvalidRetentionLabel, entry)
	}

	suffix := entry[len(entry)-1]
	if suffix >= 'A' && suffix <= 'Z' {
		suffix += 'a' - 'A'
	}

	field := fieldForSuffix(suffix)
	if field == nil {
		return fmt.Errorf(
			"%w: unknown suffix %q: %q",
			ErrInvalidRetentionLabel, string(suffix), entry,
		)
	}

	prefix := entry[:len(entry)-1]

	value, err := strconv.Atoi(prefix)
	if err != nil {
		return fmt.Errorf("%w: non-numeric: %q", ErrInvalidRetentionLabel, entry)
	}

	if value < 0 {
		return fmt.Errorf("%w: negative: %q", ErrInvalidRetentionLabel, entry)
	}

	if seen[suffix] {
		return fmt.Errorf("%w: suffix repeated: %q", ErrInvalidRetentionLabel, entry)
	}

	seen[suffix] = true
	*field(cfg) = value

	return nil
}

func fieldForSuffix(suffix byte) func(*config.RetentionConfig) *int {
	switch suffix {
	case 'd':
		return func(c *config.RetentionConfig) *int { return &c.KeepDaily }
	case 'w':
		return func(c *config.RetentionConfig) *int { return &c.KeepWeekly }
	case 'm':
		return func(c *config.RetentionConfig) *int { return &c.KeepMonthly }
	case 'y':
		return func(c *config.RetentionConfig) *int { return &c.KeepYearly }
	default:
		return nil
	}
}

// Resolve returns the effective retention policy for a target along
// with the source it came from. The label on the target's container
// takes precedence when present and parsing yields a non-zero policy;
// otherwise the global policy is used; otherwise ResolutionNone is
// returned. A label that fails to parse returns the parser error and
// ResolutionNone alongside.
func Resolve(
	target discovery.Target,
	global config.RetentionConfig,
) (config.RetentionConfig, Resolution, error) {
	raw := target.Container.Labels[filter.LabelRetention]

	if raw != "" {
		parsed, err := ParseRetentionLabel(raw)
		if err != nil {
			return config.RetentionConfig{}, ResolutionNone, err
		}

		if isNonZero(parsed) {
			return parsed, ResolutionLabel, nil
		}
	}

	if isNonZero(global) {
		return global, ResolutionGlobal, nil
	}

	return zeroPolicy(), ResolutionNone, nil
}

func zeroPolicy() config.RetentionConfig {
	return config.RetentionConfig{
		KeepDaily:   0,
		KeepWeekly:  0,
		KeepMonthly: 0,
		KeepYearly:  0,
	}
}

func isNonZero(c config.RetentionConfig) bool {
	return c.KeepDaily+c.KeepWeekly+c.KeepMonthly+c.KeepYearly > 0
}
