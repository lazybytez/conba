package discovery_test

import (
	"context"
	"errors"
	"testing"

	"github.com/lazybytez/conba/internal/discovery"
	"github.com/lazybytez/conba/internal/runtime"
)

type mockRuntime struct {
	containers []runtime.ContainerInfo
	err        error
}

func (m *mockRuntime) ListContainers(_ context.Context) ([]runtime.ContainerInfo, error) {
	return m.containers, m.err
}

func (m *mockRuntime) Close() error { return nil }

var errConnectionRefused = errors.New("connection refused")

func TestDiscover_ExpandsMounts(t *testing.T) {
	t.Parallel()

	mock := &mockRuntime{
		containers: []runtime.ContainerInfo{
			{
				ID:     "c1",
				Name:   "app",
				Labels: nil,
				Mounts: []runtime.MountInfo{
					{Type: "volume", Name: "data", Destination: "/data", ReadOnly: false},
					{Type: "volume", Name: "logs", Destination: "/logs", ReadOnly: false},
				},
			},
		},
		err: nil,
	}

	targets, err := discovery.Discover(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(targets) != 2 {
		t.Fatalf("want 2 targets, got %d", len(targets))
	}

	if targets[0].Mount.Destination != "/data" {
		t.Errorf("want /data, got %s", targets[0].Mount.Destination)
	}

	if targets[1].Mount.Destination != "/logs" {
		t.Errorf("want /logs, got %s", targets[1].Mount.Destination)
	}
}

func TestDiscover_SkipsTmpfs(t *testing.T) {
	t.Parallel()

	mock := &mockRuntime{
		containers: []runtime.ContainerInfo{
			{
				ID:     "c1",
				Name:   "app",
				Labels: nil,
				Mounts: []runtime.MountInfo{
					{Type: "volume", Name: "data", Destination: "/data", ReadOnly: false},
					{Type: "tmpfs", Name: "", Destination: "/tmp", ReadOnly: false},
				},
			},
		},
		err: nil,
	}

	targets, err := discovery.Discover(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(targets) != 1 {
		t.Fatalf("want 1 target, got %d", len(targets))
	}

	if targets[0].Mount.Type != "volume" {
		t.Errorf("want volume, got %s", targets[0].Mount.Type)
	}
}

func TestDiscover_IncludesBindMounts(t *testing.T) {
	t.Parallel()

	mock := &mockRuntime{
		containers: []runtime.ContainerInfo{
			{
				ID:     "c1",
				Name:   "app",
				Labels: nil,
				Mounts: []runtime.MountInfo{
					{Type: "bind", Name: "config", Destination: "/etc/app", ReadOnly: false},
				},
			},
		},
		err: nil,
	}

	targets, err := discovery.Discover(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(targets) != 1 {
		t.Fatalf("want 1 target, got %d", len(targets))
	}

	if targets[0].Mount.Type != "bind" {
		t.Errorf("want bind, got %s", targets[0].Mount.Type)
	}
}

func TestDiscover_EmptyContainers(t *testing.T) {
	t.Parallel()

	mock := &mockRuntime{
		containers: []runtime.ContainerInfo{},
		err:        nil,
	}

	targets, err := discovery.Discover(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(targets) != 0 {
		t.Fatalf("want 0 targets, got %d", len(targets))
	}
}

func TestDiscover_RuntimeError(t *testing.T) {
	t.Parallel()

	mock := &mockRuntime{
		containers: nil,
		err:        errConnectionRefused,
	}

	_, err := discovery.Discover(context.Background(), mock)
	if err == nil {
		t.Fatal("want error, got nil")
	}

	if !errors.Is(err, errConnectionRefused) {
		t.Errorf("want wrapped original error, got %v", err)
	}
}
