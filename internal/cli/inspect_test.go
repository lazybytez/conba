package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/lazybytez/conba/internal/discovery"
	"github.com/lazybytez/conba/internal/filter"
	"github.com/lazybytez/conba/internal/runtime"
)

func TestShortID_Long(t *testing.T) {
	t.Parallel()

	id := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	got := shortID(id)

	if got != "abcdef123456" {
		t.Errorf("shortID(%q) = %q, want %q", id, got, "abcdef123456")
	}
}

func TestShortID_Exact(t *testing.T) {
	t.Parallel()

	id := "abcdef123456"
	got := shortID(id)

	if got != id {
		t.Errorf("shortID(%q) = %q, want %q", id, got, id)
	}
}

func TestShortID_Short(t *testing.T) {
	t.Parallel()

	id := "abc"
	got := shortID(id)

	if got != id {
		t.Errorf("shortID(%q) = %q, want %q", id, got, id)
	}
}

func TestShortID_Empty(t *testing.T) {
	t.Parallel()

	got := shortID("")

	if got != "" {
		t.Errorf("shortID(%q) = %q, want %q", "", got, "")
	}
}

func TestGroupByContainer_SingleContainer(t *testing.T) {
	t.Parallel()

	container := runtime.ContainerInfo{ID: "c1", Name: "app"}
	targets := []discovery.Target{
		{Container: container, Mount: runtime.MountInfo{Name: "vol1"}},
		{Container: container, Mount: runtime.MountInfo{Name: "vol2"}},
	}

	groups := groupByContainer(targets)

	if len(groups) != 1 {
		t.Fatalf("got %d groups, want 1", len(groups))
	}

	if len(groups[0]) != 2 {
		t.Errorf("group[0] has %d targets, want 2", len(groups[0]))
	}
}

func TestGroupByContainer_MultipleContainers(t *testing.T) {
	t.Parallel()

	targets := []discovery.Target{
		{Container: runtime.ContainerInfo{ID: "c1", Name: "app"}, Mount: runtime.MountInfo{Name: "v1"}},
		{Container: runtime.ContainerInfo{ID: "c2", Name: "db"}, Mount: runtime.MountInfo{Name: "v2"}},
	}

	groups := groupByContainer(targets)

	if len(groups) != 2 {
		t.Fatalf("got %d groups, want 2", len(groups))
	}

	if groups[0][0].Container.ID != "c1" {
		t.Errorf("group[0] container ID = %q, want %q", groups[0][0].Container.ID, "c1")
	}

	if groups[1][0].Container.ID != "c2" {
		t.Errorf("group[1] container ID = %q, want %q", groups[1][0].Container.ID, "c2")
	}
}

func TestGroupByContainer_PreservesOrder(t *testing.T) {
	t.Parallel()

	targets := []discovery.Target{
		{Container: runtime.ContainerInfo{ID: "c2", Name: "db"}, Mount: runtime.MountInfo{Name: "v1"}},
		{Container: runtime.ContainerInfo{ID: "c1", Name: "app"}, Mount: runtime.MountInfo{Name: "v2"}},
		{Container: runtime.ContainerInfo{ID: "c2", Name: "db"}, Mount: runtime.MountInfo{Name: "v3"}},
	}

	groups := groupByContainer(targets)

	if len(groups) != 2 {
		t.Fatalf("got %d groups, want 2", len(groups))
	}

	if groups[0][0].Container.ID != "c2" {
		t.Errorf("first group should be c2 (first seen), got %q", groups[0][0].Container.ID)
	}

	if groups[1][0].Container.ID != "c1" {
		t.Errorf("second group should be c1, got %q", groups[1][0].Container.ID)
	}

	if len(groups[0]) != 2 {
		t.Errorf("first group should have 2 targets, got %d", len(groups[0]))
	}
}

func TestGroupByContainer_Empty(t *testing.T) {
	t.Parallel()

	groups := groupByContainer(nil)

	if groups != nil {
		t.Errorf("got %v, want nil", groups)
	}
}

func TestPrintResult_Empty(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	err := printResult(&buf, filter.Result{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "No containers with volumes found.") {
		t.Errorf("output = %q, want message about no containers", buf.String())
	}
}

func TestPrintResult_IncludedOnly(t *testing.T) {
	t.Parallel()

	result := filter.Result{
		Included: []discovery.Target{
			{
				Container: runtime.ContainerInfo{ID: "abc123def456", Name: "app"},
				Mount:     runtime.MountInfo{Type: "volume", Name: "data", Destination: "/data"},
			},
		},
	}

	var buf bytes.Buffer

	err := printResult(&buf, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "=== Included ===") {
		t.Error("output should contain Included section header")
	}

	if strings.Contains(output, "=== Excluded ===") {
		t.Error("output should not contain Excluded section header")
	}
}

func TestPrintResult_ExcludedOnly(t *testing.T) {
	t.Parallel()

	result := filter.Result{
		Excluded: []filter.Exclusion{
			{
				Target: discovery.Target{
					Container: runtime.ContainerInfo{ID: "abc123def456", Name: "app"},
					Mount:     runtime.MountInfo{Type: "volume", Name: "cache", Destination: "/cache"},
				},
				Reason: "label exclude",
			},
		},
	}

	var buf bytes.Buffer

	err := printResult(&buf, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "=== Excluded ===") {
		t.Error("output should contain Excluded section header")
	}

	if strings.Contains(output, "=== Included ===") {
		t.Error("output should not contain Included section header")
	}
}

func TestPrintResult_Both(t *testing.T) {
	t.Parallel()

	result := filter.Result{
		Included: []discovery.Target{
			{
				Container: runtime.ContainerInfo{ID: "abc123def456", Name: "app"},
				Mount:     runtime.MountInfo{Type: "volume", Name: "data", Destination: "/data"},
			},
		},
		Excluded: []filter.Exclusion{
			{
				Target: discovery.Target{
					Container: runtime.ContainerInfo{ID: "def456abc123", Name: "db"},
					Mount:     runtime.MountInfo{Type: "bind", Name: "/host/path", Destination: "/mnt"},
				},
				Reason: "not matching",
			},
		},
	}

	var buf bytes.Buffer

	err := printResult(&buf, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "=== Included ===") {
		t.Error("output should contain Included section header")
	}

	if !strings.Contains(output, "=== Excluded ===") {
		t.Error("output should contain Excluded section header")
	}
}

func TestPrintExcluded_ShowsReason(t *testing.T) {
	t.Parallel()

	exclusions := []filter.Exclusion{
		{
			Target: discovery.Target{
				Container: runtime.ContainerInfo{ID: "abc123def456", Name: "worker"},
				Mount:     runtime.MountInfo{Type: "volume", Name: "tmp", Destination: "/tmp"},
			},
			Reason: "excluded by label",
		},
	}

	var buf bytes.Buffer

	err := printExcluded(&buf, exclusions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "excluded by label") {
		t.Errorf("output should contain exclusion reason, got %q", output)
	}
}
