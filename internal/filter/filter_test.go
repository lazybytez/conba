package filter_test

import (
	"testing"

	"github.com/lazybytez/conba/internal/config"
	"github.com/lazybytez/conba/internal/discovery"
	"github.com/lazybytez/conba/internal/filter"
	"github.com/lazybytez/conba/internal/runtime"
)

func emptyConfig() config.DiscoveryConfig {
	return config.DiscoveryConfig{
		OptInOnly: false,
		Include:   config.FilterList{Names: nil, NamePatterns: nil, IDs: nil, IDPatterns: nil},
		Exclude:   config.FilterList{Names: nil, NamePatterns: nil, IDs: nil, IDPatterns: nil},
	}
}

func makeTarget(
	containerID, name string,
	labels map[string]string,
	mountName, mountDest string,
) discovery.Target {
	return discovery.Target{
		Container: runtime.ContainerInfo{
			ID:     containerID,
			Name:   name,
			Labels: labels,
			Mounts: nil,
		},
		Mount: runtime.MountInfo{
			Type:        "volume",
			Name:        mountName,
			Destination: mountDest,
			ReadOnly:    false,
		},
	}
}

func TestApply_DefaultInclude(t *testing.T) {
	t.Parallel()

	targets := []discovery.Target{
		makeTarget("c1", "app", map[string]string{}, "data", "/data"),
		makeTarget("c1", "app", map[string]string{}, "logs", "/logs"),
	}

	result := filter.Apply(targets, emptyConfig())

	if len(result.Included) != 2 {
		t.Errorf("want 2 included, got %d", len(result.Included))
	}

	if len(result.Excluded) != 0 {
		t.Errorf("want 0 excluded, got %d", len(result.Excluded))
	}
}

func TestApply_DisabledByLabel(t *testing.T) {
	t.Parallel()

	labels := map[string]string{filter.LabelEnabled: filter.LabelValueFalse}
	targets := []discovery.Target{
		makeTarget("c1", "app", labels, "data", "/data"),
		makeTarget("c1", "app", labels, "logs", "/logs"),
	}

	result := filter.Apply(targets, emptyConfig())

	if len(result.Included) != 0 {
		t.Errorf("want 0 included, got %d", len(result.Included))
	}

	if len(result.Excluded) != 2 {
		t.Fatalf("want 2 excluded, got %d", len(result.Excluded))
	}

	for _, exclusion := range result.Excluded {
		if exclusion.Reason != "excluded by conba.enabled=false label" {
			t.Errorf(
				"want reason 'excluded by conba.enabled=false label', got %q",
				exclusion.Reason,
			)
		}
	}
}

func TestApply_EnabledOverridesExcludeList(t *testing.T) {
	t.Parallel()

	labels := map[string]string{filter.LabelEnabled: filter.LabelValueTrue}
	targets := []discovery.Target{
		makeTarget("c1", "app", labels, "data", "/data"),
	}

	cfg := config.DiscoveryConfig{
		OptInOnly: false,
		Include:   config.FilterList{Names: nil, NamePatterns: nil, IDs: nil, IDPatterns: nil},
		Exclude: config.FilterList{
			Names: []string{"app"}, NamePatterns: nil, IDs: nil, IDPatterns: nil,
		},
	}

	result := filter.Apply(targets, cfg)

	if len(result.Included) != 1 {
		t.Errorf("want 1 included, got %d", len(result.Included))
	}

	if len(result.Excluded) != 0 {
		t.Errorf("want 0 excluded, got %d", len(result.Excluded))
	}
}

func TestApply_ExcludeVolumesLabel(t *testing.T) {
	t.Parallel()

	labels := map[string]string{filter.LabelExcludeVolumes: "logs, temp"}
	targets := []discovery.Target{
		makeTarget("c1", "app", labels, "data", "/data"),
		makeTarget("c1", "app", labels, "logs", "/logs"),
	}

	result := filter.Apply(targets, emptyConfig())

	if len(result.Included) != 1 {
		t.Fatalf("want 1 included, got %d", len(result.Included))
	}

	if len(result.Excluded) != 1 {
		t.Fatalf("want 1 excluded, got %d", len(result.Excluded))
	}

	if result.Excluded[0].Reason != "excluded by conba.exclude-volumes label" {
		t.Errorf(
			"want reason 'excluded by conba.exclude-volumes label', got %q",
			result.Excluded[0].Reason,
		)
	}

	if result.Excluded[0].Target.Mount.Name != "logs" {
		t.Errorf(
			"want excluded mount 'logs', got %q",
			result.Excluded[0].Target.Mount.Name,
		)
	}
}

func TestApply_IncludeListByName(t *testing.T) {
	t.Parallel()

	targets := []discovery.Target{
		makeTarget("c1", "app", map[string]string{}, "data", "/data"),
		makeTarget("c2", "db", map[string]string{}, "pgdata", "/var/lib/pg"),
	}

	cfg := config.DiscoveryConfig{
		OptInOnly: false,
		Include: config.FilterList{
			Names: []string{"app"}, NamePatterns: nil, IDs: nil, IDPatterns: nil,
		},
		Exclude: config.FilterList{Names: nil, NamePatterns: nil, IDs: nil, IDPatterns: nil},
	}

	result := filter.Apply(targets, cfg)

	if len(result.Included) != 1 {
		t.Fatalf("want 1 included, got %d", len(result.Included))
	}

	if result.Included[0].Container.Name != "app" {
		t.Errorf(
			"want included container 'app', got %q",
			result.Included[0].Container.Name,
		)
	}

	if len(result.Excluded) != 1 {
		t.Fatalf("want 1 excluded, got %d", len(result.Excluded))
	}

	if result.Excluded[0].Reason != "not in include list" {
		t.Errorf(
			"want reason 'not in include list', got %q",
			result.Excluded[0].Reason,
		)
	}
}

func TestApply_IncludeListByID(t *testing.T) {
	t.Parallel()

	targets := []discovery.Target{
		makeTarget("abc123", "app", map[string]string{}, "data", "/data"),
		makeTarget("def456", "db", map[string]string{}, "pgdata", "/var/lib/pg"),
	}

	cfg := config.DiscoveryConfig{
		OptInOnly: false,
		Include: config.FilterList{
			Names: nil, NamePatterns: nil, IDs: []string{"abc123"}, IDPatterns: nil,
		},
		Exclude: config.FilterList{Names: nil, NamePatterns: nil, IDs: nil, IDPatterns: nil},
	}

	result := filter.Apply(targets, cfg)

	if len(result.Included) != 1 {
		t.Fatalf("want 1 included, got %d", len(result.Included))
	}

	if result.Included[0].Container.ID != "abc123" {
		t.Errorf(
			"want included container ID 'abc123', got %q",
			result.Included[0].Container.ID,
		)
	}

	if len(result.Excluded) != 1 {
		t.Errorf("want 1 excluded, got %d", len(result.Excluded))
	}
}

func TestApply_ExcludeListByName(t *testing.T) {
	t.Parallel()

	targets := []discovery.Target{
		makeTarget("c1", "app", map[string]string{}, "data", "/data"),
		makeTarget("c2", "db", map[string]string{}, "pgdata", "/var/lib/pg"),
	}

	cfg := config.DiscoveryConfig{
		OptInOnly: false,
		Include:   config.FilterList{Names: nil, NamePatterns: nil, IDs: nil, IDPatterns: nil},
		Exclude: config.FilterList{
			Names: []string{"db"}, NamePatterns: nil, IDs: nil, IDPatterns: nil,
		},
	}

	result := filter.Apply(targets, cfg)

	if len(result.Included) != 1 {
		t.Fatalf("want 1 included, got %d", len(result.Included))
	}

	if len(result.Excluded) != 1 {
		t.Fatalf("want 1 excluded, got %d", len(result.Excluded))
	}

	if result.Excluded[0].Reason != "matched exclude list: db" {
		t.Errorf(
			"want reason 'matched exclude list: db', got %q",
			result.Excluded[0].Reason,
		)
	}
}

func TestApply_ExcludeListByID(t *testing.T) {
	t.Parallel()

	targets := []discovery.Target{
		makeTarget("abc123", "app", map[string]string{}, "data", "/data"),
		makeTarget("def456", "db", map[string]string{}, "pgdata", "/var/lib/pg"),
	}

	cfg := config.DiscoveryConfig{
		OptInOnly: false,
		Include:   config.FilterList{Names: nil, NamePatterns: nil, IDs: nil, IDPatterns: nil},
		Exclude: config.FilterList{
			Names: nil, NamePatterns: nil, IDs: []string{"def456"}, IDPatterns: nil,
		},
	}

	result := filter.Apply(targets, cfg)

	if len(result.Included) != 1 {
		t.Fatalf("want 1 included, got %d", len(result.Included))
	}

	if len(result.Excluded) != 1 {
		t.Fatalf("want 1 excluded, got %d", len(result.Excluded))
	}

	if result.Excluded[0].Reason != "matched exclude list: def456" {
		t.Errorf(
			"want reason 'matched exclude list: def456', got %q",
			result.Excluded[0].Reason,
		)
	}
}

func TestApply_OptInMode(t *testing.T) {
	t.Parallel()

	targets := []discovery.Target{
		makeTarget(
			"c1", "app",
			map[string]string{filter.LabelEnabled: filter.LabelValueTrue},
			"data", "/data",
		),
		makeTarget("c2", "db", map[string]string{}, "pgdata", "/var/lib/pg"),
	}

	cfg := config.DiscoveryConfig{
		OptInOnly: true,
		Include:   config.FilterList{Names: nil, NamePatterns: nil, IDs: nil, IDPatterns: nil},
		Exclude:   config.FilterList{Names: nil, NamePatterns: nil, IDs: nil, IDPatterns: nil},
	}

	result := filter.Apply(targets, cfg)

	if len(result.Included) != 1 {
		t.Fatalf("want 1 included, got %d", len(result.Included))
	}

	if result.Included[0].Container.Name != "app" {
		t.Errorf(
			"want included container 'app', got %q",
			result.Included[0].Container.Name,
		)
	}

	if len(result.Excluded) != 1 {
		t.Fatalf("want 1 excluded, got %d", len(result.Excluded))
	}

	if result.Excluded[0].Reason != "opt-in mode: missing conba.enabled=true label" {
		t.Errorf("want opt-in reason, got %q", result.Excluded[0].Reason)
	}
}

func TestApply_EnabledWithExcludeVolumes(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		filter.LabelEnabled:        filter.LabelValueTrue,
		filter.LabelExcludeVolumes: "logs",
	}
	targets := []discovery.Target{
		makeTarget("c1", "app", labels, "data", "/data"),
		makeTarget("c1", "app", labels, "logs", "/logs"),
	}

	cfg := config.DiscoveryConfig{
		OptInOnly: true,
		Include:   config.FilterList{Names: nil, NamePatterns: nil, IDs: nil, IDPatterns: nil},
		Exclude:   config.FilterList{Names: nil, NamePatterns: nil, IDs: nil, IDPatterns: nil},
	}

	result := filter.Apply(targets, cfg)

	if len(result.Included) != 1 {
		t.Fatalf("want 1 included, got %d", len(result.Included))
	}

	if result.Included[0].Mount.Name != "data" {
		t.Errorf(
			"want included mount 'data', got %q",
			result.Included[0].Mount.Name,
		)
	}

	if len(result.Excluded) != 1 {
		t.Fatalf("want 1 excluded, got %d", len(result.Excluded))
	}

	if result.Excluded[0].Target.Mount.Name != "logs" {
		t.Errorf(
			"want excluded mount 'logs', got %q",
			result.Excluded[0].Target.Mount.Name,
		)
	}
}

func TestApply_EmptyTargets(t *testing.T) {
	t.Parallel()

	result := filter.Apply([]discovery.Target{}, emptyConfig())

	if len(result.Included) != 0 {
		t.Errorf("want 0 included, got %d", len(result.Included))
	}

	if len(result.Excluded) != 0 {
		t.Errorf("want 0 excluded, got %d", len(result.Excluded))
	}
}

func TestApply_IncludeByNamePattern(t *testing.T) {
	t.Parallel()

	targets := []discovery.Target{
		makeTarget("c1", "app-web", nil, "data", "/data"),
		makeTarget("c2", "db-postgres", nil, "pgdata", "/var/lib/pg"),
		makeTarget("c3", "app-api", nil, "logs", "/logs"),
	}

	cfg := config.DiscoveryConfig{
		OptInOnly: false,
		Include: config.FilterList{
			Names: nil, NamePatterns: []string{"^app-"}, IDs: nil, IDPatterns: nil,
		},
		Exclude: config.FilterList{Names: nil, NamePatterns: nil, IDs: nil, IDPatterns: nil},
	}

	result := filter.Apply(targets, cfg)

	if len(result.Included) != 2 {
		t.Fatalf("want 2 included, got %d", len(result.Included))
	}

	if len(result.Excluded) != 1 {
		t.Fatalf("want 1 excluded, got %d", len(result.Excluded))
	}
}

func TestApply_ExcludeByNamePattern(t *testing.T) {
	t.Parallel()

	targets := []discovery.Target{
		makeTarget("c1", "app-web", nil, "data", "/data"),
		makeTarget("c2", "db-postgres", nil, "pgdata", "/var/lib/pg"),
	}

	cfg := config.DiscoveryConfig{
		OptInOnly: false,
		Include:   config.FilterList{Names: nil, NamePatterns: nil, IDs: nil, IDPatterns: nil},
		Exclude: config.FilterList{
			Names: nil, NamePatterns: []string{"^db-"}, IDs: nil, IDPatterns: nil,
		},
	}

	result := filter.Apply(targets, cfg)

	if len(result.Included) != 1 {
		t.Fatalf("want 1 included, got %d", len(result.Included))
	}

	if len(result.Excluded) != 1 {
		t.Fatalf("want 1 excluded, got %d", len(result.Excluded))
	}
}

func TestApply_IncludeByIDPattern(t *testing.T) {
	t.Parallel()

	targets := []discovery.Target{
		makeTarget("abc123", "app", nil, "data", "/data"),
		makeTarget("def456", "db", nil, "pgdata", "/var/lib/pg"),
	}

	cfg := config.DiscoveryConfig{
		OptInOnly: false,
		Include: config.FilterList{
			Names: nil, NamePatterns: nil, IDs: nil, IDPatterns: []string{"^abc"},
		},
		Exclude: config.FilterList{Names: nil, NamePatterns: nil, IDs: nil, IDPatterns: nil},
	}

	result := filter.Apply(targets, cfg)

	if len(result.Included) != 1 {
		t.Fatalf("want 1 included, got %d", len(result.Included))
	}

	if result.Included[0].Container.ID != "abc123" {
		t.Errorf("want abc123, got %s", result.Included[0].Container.ID)
	}
}

func TestApply_ExcludeByIDPattern(t *testing.T) {
	t.Parallel()

	targets := []discovery.Target{
		makeTarget("abc123", "app", nil, "data", "/data"),
		makeTarget("def456", "db", nil, "pgdata", "/var/lib/pg"),
	}

	cfg := config.DiscoveryConfig{
		OptInOnly: false,
		Include:   config.FilterList{Names: nil, NamePatterns: nil, IDs: nil, IDPatterns: nil},
		Exclude: config.FilterList{
			Names: nil, NamePatterns: nil, IDs: nil, IDPatterns: []string{"^def"},
		},
	}

	result := filter.Apply(targets, cfg)

	if len(result.Included) != 1 {
		t.Fatalf("want 1 included, got %d", len(result.Included))
	}

	if len(result.Excluded) != 1 {
		t.Fatalf("want 1 excluded, got %d", len(result.Excluded))
	}
}
