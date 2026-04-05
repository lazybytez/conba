// Package filter applies discovery filters to container-volume targets
// based on container labels and configuration rules.
package filter

import (
	"slices"
	"strings"

	"github.com/lazybytez/conba/internal/config"
	"github.com/lazybytez/conba/internal/discovery"
)

// Result holds the outcome of applying filters to discovery targets.
type Result struct {
	Included []discovery.Target
	Excluded []Exclusion
}

// Exclusion represents a target that was filtered out, with the reason.
type Exclusion struct {
	Target discovery.Target
	Reason string
}

// Apply filters targets according to container labels and the provided
// discovery configuration. It returns which targets passed and which
// were excluded, along with the reason for each exclusion.
func Apply(targets []discovery.Target, cfg config.DiscoveryConfig) Result {
	var result Result

	for _, target := range targets {
		if reason, excluded := evaluate(target, cfg); excluded {
			result.Excluded = append(result.Excluded, Exclusion{
				Target: target,
				Reason: reason,
			})

			continue
		}

		result.Included = append(result.Included, target)
	}

	return result
}

func evaluate(target discovery.Target, cfg config.DiscoveryConfig) (string, bool) {
	if isDisabledByLabel(target) {
		return "excluded by conba.enabled=false label", true
	}

	forceIncluded := isEnabledByLabel(target)

	if !forceIncluded {
		if reason, excluded := evaluateContainerFilters(target, cfg); excluded {
			return reason, true
		}
	}

	if isExcludedByVolumeLabel(target) {
		return "excluded by conba.exclude-volumes label", true
	}

	return "", false
}

func evaluateContainerFilters(
	target discovery.Target,
	cfg config.DiscoveryConfig,
) (string, bool) {
	if !matchesIncludeList(target, cfg) {
		return "not in include list", true
	}

	if matched, value := matchesExcludeList(target, cfg); matched {
		return "matched exclude list: " + value, true
	}

	if cfg.OptInOnly {
		return "opt-in mode: missing conba.enabled=true label", true
	}

	return "", false
}

func isDisabledByLabel(target discovery.Target) bool {
	return target.Container.Labels["conba.enabled"] == "false"
}

func isEnabledByLabel(target discovery.Target) bool {
	return target.Container.Labels["conba.enabled"] == "true"
}

func isExcludedByVolumeLabel(target discovery.Target) bool {
	raw, ok := target.Container.Labels["conba.exclude-volumes"]
	if !ok {
		return false
	}

	for entry := range strings.SplitSeq(raw, ",") {
		if strings.TrimSpace(entry) == target.Mount.Name {
			return true
		}
	}

	return false
}

func matchesIncludeList(
	target discovery.Target,
	cfg config.DiscoveryConfig,
) bool {
	if len(cfg.Include.Names) == 0 && len(cfg.Include.IDs) == 0 {
		return true
	}

	return slices.Contains(cfg.Include.Names, target.Container.Name) ||
		slices.Contains(cfg.Include.IDs, target.Container.ID)
}

func matchesExcludeList(
	target discovery.Target,
	cfg config.DiscoveryConfig,
) (bool, string) {
	for _, name := range cfg.Exclude.Names {
		if name == target.Container.Name {
			return true, name
		}
	}

	for _, id := range cfg.Exclude.IDs {
		if id == target.Container.ID {
			return true, id
		}
	}

	return false, ""
}
