package backup_test

import (
	"testing"

	"github.com/lazybytez/conba/internal/backup"
	"github.com/lazybytez/conba/internal/discovery"
	"github.com/lazybytez/conba/internal/runtime"
)

func TestBuildTags_Volume(t *testing.T) {
	t.Parallel()

	target := discovery.Target{
		Container: runtime.ContainerInfo{
			ID:     "abc123",
			Name:   "myapp",
			Labels: nil,
			Mounts: nil,
		},
		Mount: runtime.MountInfo{
			Type:        runtime.MountTypeVolume,
			Name:        "data",
			Source:      "/var/lib/docker/volumes/data/_data",
			Destination: "/data",
			ReadOnly:    false,
		},
	}

	tags := backup.BuildTags(target, "server01")

	if len(tags) != 3 {
		t.Fatalf("want 3 tags, got %d", len(tags))
	}

	if tags[0] != "container=myapp" {
		t.Errorf("tags[0] = %q, want %q", tags[0], "container=myapp")
	}

	if tags[1] != "volume=data" {
		t.Errorf("tags[1] = %q, want %q", tags[1], "volume=data")
	}

	if tags[2] != "hostname=server01" {
		t.Errorf("tags[2] = %q, want %q", tags[2], "hostname=server01")
	}
}

func TestBuildTags_Bind(t *testing.T) {
	t.Parallel()

	target := discovery.Target{
		Container: runtime.ContainerInfo{
			ID:     "def456",
			Name:   "webapp",
			Labels: nil,
			Mounts: nil,
		},
		Mount: runtime.MountInfo{
			Type:        runtime.MountTypeBind,
			Name:        "/host/data",
			Source:      "/host/data",
			Destination: "/mnt/data",
			ReadOnly:    false,
		},
	}

	tags := backup.BuildTags(target, "node42")

	if len(tags) != 3 {
		t.Fatalf("want 3 tags, got %d", len(tags))
	}

	if tags[0] != "container=webapp" {
		t.Errorf("tags[0] = %q, want %q", tags[0], "container=webapp")
	}

	if tags[1] != "volume=/host/data" {
		t.Errorf("tags[1] = %q, want %q", tags[1], "volume=/host/data")
	}

	if tags[2] != "hostname=node42" {
		t.Errorf("tags[2] = %q, want %q", tags[2], "hostname=node42")
	}
}

func TestBuildTags_EmptyHostname(t *testing.T) {
	t.Parallel()

	target := discovery.Target{
		Container: runtime.ContainerInfo{
			ID:     "ghi789",
			Name:   "service",
			Labels: nil,
			Mounts: nil,
		},
		Mount: runtime.MountInfo{
			Type:        runtime.MountTypeVolume,
			Name:        "logs",
			Source:      "",
			Destination: "/logs",
			ReadOnly:    false,
		},
	}

	tags := backup.BuildTags(target, "")

	if len(tags) != 3 {
		t.Fatalf("want 3 tags, got %d", len(tags))
	}

	if tags[2] != "hostname=" {
		t.Errorf("tags[2] = %q, want %q", tags[2], "hostname=")
	}
}

func TestBuildStreamTags_Happy(t *testing.T) {
	t.Parallel()

	tags := backup.BuildStreamTags("mysql", "host01")

	if len(tags) != 3 {
		t.Fatalf("want 3 tags, got %d", len(tags))
	}

	if tags[0] != "container=mysql" {
		t.Errorf("tags[0] = %q, want %q", tags[0], "container=mysql")
	}

	if tags[1] != "hostname=host01" {
		t.Errorf("tags[1] = %q, want %q", tags[1], "hostname=host01")
	}

	if tags[2] != "kind=stream" {
		t.Errorf("tags[2] = %q, want %q", tags[2], "kind=stream")
	}
}

func TestBuildStreamTags_EmptyHostname(t *testing.T) {
	t.Parallel()

	tags := backup.BuildStreamTags("postgres", "")

	if len(tags) != 3 {
		t.Fatalf("want 3 tags, got %d", len(tags))
	}

	if tags[0] != "container=postgres" {
		t.Errorf("tags[0] = %q, want %q", tags[0], "container=postgres")
	}

	if tags[1] != "hostname=" {
		t.Errorf("tags[1] = %q, want %q", tags[1], "hostname=")
	}

	if tags[2] != "kind=stream" {
		t.Errorf("tags[2] = %q, want %q", tags[2], "kind=stream")
	}
}

func TestBuildTags_SpecialChars(t *testing.T) {
	t.Parallel()

	target := discovery.Target{
		Container: runtime.ContainerInfo{
			ID:     "jkl012",
			Name:   "my-app_v2",
			Labels: nil,
			Mounts: nil,
		},
		Mount: runtime.MountInfo{
			Type:        runtime.MountTypeVolume,
			Name:        "db_data-main",
			Source:      "/var/lib/docker/volumes/db_data-main/_data",
			Destination: "/var/lib/db",
			ReadOnly:    false,
		},
	}

	tags := backup.BuildTags(target, "host-01_prod")

	if tags[0] != "container=my-app_v2" {
		t.Errorf("tags[0] = %q, want %q", tags[0], "container=my-app_v2")
	}

	if tags[1] != "volume=db_data-main" {
		t.Errorf("tags[1] = %q, want %q", tags[1], "volume=db_data-main")
	}

	if tags[2] != "hostname=host-01_prod" {
		t.Errorf("tags[2] = %q, want %q", tags[2], "hostname=host-01_prod")
	}
}
