// Package filter applies discovery filters to container-volume targets
// based on container labels and configuration rules.
package filter

import (
	"regexp"
	"slices"
	"strings"

	"github.com/lazybytez/conba/internal/config"
	"github.com/lazybytez/conba/internal/discovery"
	"github.com/lazybytez/conba/internal/runtime"
)

// Container label keys used by the filter engine.
const (
	LabelEnabled                  = "conba.enabled"
	LabelExcludeVolumes           = "conba.exclude-volumes"
	LabelExcludeBindMounts        = "conba.exclude-bind-mounts"
	LabelExcludeMountDestinations = "conba.exclude-mount-destinations"
)

// Label values for the enabled label.
const (
	LabelValueTrue  = "true"
	LabelValueFalse = "false"
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

	if isExcludedByBindMountToggle(target) {
		return "excluded by conba.exclude-bind-mounts label", true
	}

	if isExcludedByDestination(target) {
		return "excluded by conba.exclude-mount-destinations label", true
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
	return target.Container.Labels[LabelEnabled] == LabelValueFalse
}

func isEnabledByLabel(target discovery.Target) bool {
	return target.Container.Labels[LabelEnabled] == LabelValueTrue
}

func isExcludedByVolumeLabel(target discovery.Target) bool {
	raw, ok := target.Container.Labels[LabelExcludeVolumes]
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

func isExcludedByBindMountToggle(target discovery.Target) bool {
	if target.Mount.Type != runtime.MountTypeBind {
		return false
	}

	return target.Container.Labels[LabelExcludeBindMounts] == LabelValueTrue
}

func isExcludedByDestination(target discovery.Target) bool {
	raw, ok := target.Container.Labels[LabelExcludeMountDestinations]
	if !ok {
		return false
	}

	for entry := range strings.SplitSeq(raw, ",") {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" {
			continue
		}

		if trimmed == target.Mount.Destination {
			return true
		}
	}

	return false
}

func matchesIncludeList(
	target discovery.Target,
	cfg config.DiscoveryConfig,
) bool {
	hasRules := len(cfg.Include.Names) > 0 ||
		len(cfg.Include.NamePatterns) > 0 ||
		len(cfg.Include.IDs) > 0 ||
		len(cfg.Include.IDPatterns) > 0

	if !hasRules {
		return true
	}

	return matchesFilterList(target, cfg.Include)
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

	for _, pattern := range cfg.Exclude.NamePatterns {
		if matchesPattern(pattern, target.Container.Name) {
			return true, pattern
		}
	}

	for _, id := range cfg.Exclude.IDs {
		if id == target.Container.ID {
			return true, id
		}
	}

	for _, pattern := range cfg.Exclude.IDPatterns {
		if matchesPattern(pattern, target.Container.ID) {
			return true, pattern
		}
	}

	return false, ""
}

func matchesFilterList(
	target discovery.Target,
	list config.FilterList,
) bool {
	if slices.Contains(list.Names, target.Container.Name) {
		return true
	}

	if slices.Contains(list.IDs, target.Container.ID) {
		return true
	}

	for _, pattern := range list.NamePatterns {
		if matchesPattern(pattern, target.Container.Name) {
			return true
		}
	}

	for _, pattern := range list.IDPatterns {
		if matchesPattern(pattern, target.Container.ID) {
			return true
		}
	}

	return false
}

func matchesPattern(pattern string, value string) bool {
	matched, err := regexp.MatchString(pattern, value)

	return err == nil && matched
}
