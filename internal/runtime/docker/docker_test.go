package docker

import (
	"testing"

	"github.com/docker/docker/api/types/container"

	"github.com/lazybytez/conba/internal/runtime"
)

func TestContainerName_StripSlash(t *testing.T) {
	t.Parallel()

	got := containerName([]string{"/myapp"})

	if got != "myapp" {
		t.Errorf("want %q, got %q", "myapp", got)
	}
}

func TestContainerName_NoSlash(t *testing.T) {
	t.Parallel()

	got := containerName([]string{"myapp"})

	if got != "myapp" {
		t.Errorf("want %q, got %q", "myapp", got)
	}
}

func TestContainerName_EmptySlice(t *testing.T) {
	t.Parallel()

	got := containerName([]string{})

	if got != "" {
		t.Errorf("want empty string, got %q", got)
	}
}

func TestMapMounts_Volume(t *testing.T) {
	t.Parallel()

	mounts := []container.MountPoint{
		{
			Type:        "volume",
			Name:        "my-volume",
			Source:      "/var/lib/docker/volumes/my-volume/_data",
			Destination: "/data",
			RW:          true,
		},
	}

	got := mapMounts(mounts)

	if len(got) != 1 {
		t.Fatalf("want 1 mount, got %d", len(got))
	}

	if got[0].Name != "my-volume" {
		t.Errorf("want Name %q, got %q", "my-volume", got[0].Name)
	}

	if got[0].Type != "volume" {
		t.Errorf("want Type %q, got %q", "volume", got[0].Type)
	}

	if got[0].Destination != "/data" {
		t.Errorf("want Destination %q, got %q", "/data", got[0].Destination)
	}
}

func TestMapMounts_Bind(t *testing.T) {
	t.Parallel()

	mounts := []container.MountPoint{
		{
			Type:        runtime.MountTypeBind,
			Name:        "",
			Source:      "/host/path",
			Destination: "/container/path",
			RW:          true,
		},
	}

	got := mapMounts(mounts)

	if len(got) != 1 {
		t.Fatalf("want 1 mount, got %d", len(got))
	}

	if got[0].Name != "/host/path" {
		t.Errorf("want Name %q (source path), got %q", "/host/path", got[0].Name)
	}

	if got[0].Type != runtime.MountTypeBind {
		t.Errorf("want Type %q, got %q", runtime.MountTypeBind, got[0].Type)
	}
}

func TestMapMounts_ReadOnly(t *testing.T) {
	t.Parallel()

	mounts := []container.MountPoint{
		{
			Type:        "volume",
			Name:        "ro-vol",
			Destination: "/data",
			RW:          false,
		},
	}

	got := mapMounts(mounts)

	if len(got) != 1 {
		t.Fatalf("want 1 mount, got %d", len(got))
	}

	if !got[0].ReadOnly {
		t.Errorf("want ReadOnly true when RW is false")
	}
}

func TestMapMounts_ReadWrite(t *testing.T) {
	t.Parallel()

	mounts := []container.MountPoint{
		{
			Type:        "volume",
			Name:        "rw-vol",
			Destination: "/data",
			RW:          true,
		},
	}

	got := mapMounts(mounts)

	if len(got) != 1 {
		t.Fatalf("want 1 mount, got %d", len(got))
	}

	if got[0].ReadOnly {
		t.Errorf("want ReadOnly false when RW is true")
	}
}

func TestMapMounts_Empty(t *testing.T) {
	t.Parallel()

	got := mapMounts([]container.MountPoint{})

	if len(got) != 0 {
		t.Errorf("want 0 mounts, got %d", len(got))
	}
}

func TestMapMounts_Mixed(t *testing.T) {
	t.Parallel()

	mounts := []container.MountPoint{
		{
			Type:        "volume",
			Name:        "db-data",
			Source:      "/var/lib/docker/volumes/db-data/_data",
			Destination: "/var/lib/postgresql/data",
			RW:          true,
		},
		{
			Type:        runtime.MountTypeBind,
			Name:        "",
			Source:      "/etc/config",
			Destination: "/config",
			RW:          false,
		},
		{
			Type:        "tmpfs",
			Name:        "",
			Source:      "",
			Destination: "/tmp",
			RW:          true,
		},
	}

	got := mapMounts(mounts)

	if len(got) != 3 {
		t.Fatalf("want 3 mounts, got %d", len(got))
	}

	if got[0].Name != "db-data" {
		t.Errorf("volume: want Name %q, got %q", "db-data", got[0].Name)
	}

	if got[0].ReadOnly {
		t.Errorf("volume: want ReadOnly false")
	}

	if got[1].Name != "/etc/config" {
		t.Errorf("bind: want Name %q (source), got %q", "/etc/config", got[1].Name)
	}

	if !got[1].ReadOnly {
		t.Errorf("bind: want ReadOnly true")
	}

	if got[2].Type != "tmpfs" {
		t.Errorf("tmpfs: want Type %q, got %q", "tmpfs", got[2].Type)
	}

	if got[2].Destination != "/tmp" {
		t.Errorf("tmpfs: want Destination %q, got %q", "/tmp", got[2].Destination)
	}
}
