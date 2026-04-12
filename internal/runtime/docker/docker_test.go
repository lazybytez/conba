package docker_test

import (
	"testing"

	"github.com/lazybytez/conba/internal/runtime"
	"github.com/lazybytez/conba/internal/runtime/docker"
)

func TestContainerName_StripSlash(t *testing.T) {
	t.Parallel()

	got := docker.ContainerName([]string{"/myapp"})

	if got != "myapp" {
		t.Errorf("want %q, got %q", "myapp", got)
	}
}

func TestContainerName_NoSlash(t *testing.T) {
	t.Parallel()

	got := docker.ContainerName([]string{"myapp"})

	if got != "myapp" {
		t.Errorf("want %q, got %q", "myapp", got)
	}
}

func TestContainerName_EmptySlice(t *testing.T) {
	t.Parallel()

	got := docker.ContainerName([]string{})

	if got != "" {
		t.Errorf("want empty string, got %q", got)
	}
}

func TestMapMounts_Volume(t *testing.T) {
	t.Parallel()

	mounts := []docker.MountPoint{
		{
			Type:        "volume",
			Name:        "my-volume",
			Source:      "/var/lib/docker/volumes/my-volume/_data",
			Destination: "/data",
			Driver:      "local",
			Mode:        "",
			RW:          true,
			Propagation: "",
		},
	}

	got := docker.MapMounts(mounts)

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

	if got[0].Source != "/var/lib/docker/volumes/my-volume/_data" {
		t.Errorf("want Source %q, got %q", "/var/lib/docker/volumes/my-volume/_data", got[0].Source)
	}
}

func TestMapMounts_Bind(t *testing.T) {
	t.Parallel()

	mounts := []docker.MountPoint{
		{
			Type:        runtime.MountTypeBind,
			Name:        "",
			Source:      "/host/path",
			Destination: "/container/path",
			Driver:      "",
			Mode:        "",
			RW:          true,
			Propagation: "",
		},
	}

	got := docker.MapMounts(mounts)

	if len(got) != 1 {
		t.Fatalf("want 1 mount, got %d", len(got))
	}

	if got[0].Name != "/host/path" {
		t.Errorf("want Name %q (source path), got %q", "/host/path", got[0].Name)
	}

	if got[0].Type != runtime.MountTypeBind {
		t.Errorf("want Type %q, got %q", runtime.MountTypeBind, got[0].Type)
	}

	if got[0].Source != "/host/path" {
		t.Errorf("want Source %q, got %q", "/host/path", got[0].Source)
	}
}

func TestMapMounts_ReadOnly(t *testing.T) {
	t.Parallel()

	mounts := []docker.MountPoint{
		{
			Type:        "volume",
			Name:        "ro-vol",
			Source:      "/some/path",
			Destination: "/data",
			Driver:      "",
			Mode:        "",
			RW:          false,
			Propagation: "",
		},
	}

	got := docker.MapMounts(mounts)

	if len(got) != 1 {
		t.Fatalf("want 1 mount, got %d", len(got))
	}

	if !got[0].ReadOnly {
		t.Errorf("want ReadOnly true when RW is false")
	}

	if got[0].Source != "/some/path" {
		t.Errorf("want Source %q, got %q", "/some/path", got[0].Source)
	}
}

func TestMapMounts_ReadWrite(t *testing.T) {
	t.Parallel()

	mounts := []docker.MountPoint{
		{
			Type:        "volume",
			Name:        "rw-vol",
			Source:      "/some/path",
			Destination: "/data",
			Driver:      "",
			Mode:        "",
			RW:          true,
			Propagation: "",
		},
	}

	got := docker.MapMounts(mounts)

	if len(got) != 1 {
		t.Fatalf("want 1 mount, got %d", len(got))
	}

	if got[0].ReadOnly {
		t.Errorf("want ReadOnly false when RW is true")
	}

	if got[0].Source != "/some/path" {
		t.Errorf("want Source %q, got %q", "/some/path", got[0].Source)
	}
}

func TestMapMounts_Empty(t *testing.T) {
	t.Parallel()

	got := docker.MapMounts([]docker.MountPoint{})

	if len(got) != 0 {
		t.Errorf("want 0 mounts, got %d", len(got))
	}
}

func mixedMounts() []docker.MountPoint {
	return []docker.MountPoint{
		{
			Type:        "volume",
			Name:        "db-data",
			Source:      "/var/lib/docker/volumes/db-data/_data",
			Destination: "/var/lib/postgresql/data",
			Driver:      "local",
			Mode:        "",
			RW:          true,
			Propagation: "",
		},
		{
			Type:        runtime.MountTypeBind,
			Name:        "",
			Source:      "/etc/config",
			Destination: "/config",
			Driver:      "",
			Mode:        "",
			RW:          false,
			Propagation: "",
		},
		{
			Type:        "tmpfs",
			Name:        "",
			Source:      "",
			Destination: "/tmp",
			Driver:      "",
			Mode:        "",
			RW:          true,
			Propagation: "",
		},
	}
}

func TestMapMounts_MixedCount(t *testing.T) {
	t.Parallel()

	got := docker.MapMounts(mixedMounts())

	if len(got) != 3 {
		t.Fatalf("want 3 mounts, got %d", len(got))
	}
}

func TestMapMounts_MixedVolume(t *testing.T) {
	t.Parallel()

	got := docker.MapMounts(mixedMounts())

	if got[0].Name != "db-data" {
		t.Errorf("want Name %q, got %q", "db-data", got[0].Name)
	}

	if got[0].ReadOnly {
		t.Errorf("want ReadOnly false")
	}

	if got[0].Source != "/var/lib/docker/volumes/db-data/_data" {
		t.Errorf("want Source %q, got %q", "/var/lib/docker/volumes/db-data/_data", got[0].Source)
	}
}

func TestMapMounts_MixedBind(t *testing.T) {
	t.Parallel()

	got := docker.MapMounts(mixedMounts())

	if got[1].Name != "/etc/config" {
		t.Errorf("want Name %q (source), got %q", "/etc/config", got[1].Name)
	}

	if !got[1].ReadOnly {
		t.Errorf("want ReadOnly true")
	}

	if got[1].Source != "/etc/config" {
		t.Errorf("want Source %q, got %q", "/etc/config", got[1].Source)
	}
}

func TestMapMounts_MixedTmpfs(t *testing.T) {
	t.Parallel()

	got := docker.MapMounts(mixedMounts())

	if got[2].Type != "tmpfs" {
		t.Errorf("want Type %q, got %q", "tmpfs", got[2].Type)
	}

	if got[2].Destination != "/tmp" {
		t.Errorf("want Destination %q, got %q", "/tmp", got[2].Destination)
	}

	if got[2].Source != "" {
		t.Errorf("want Source %q (empty for tmpfs), got %q", "", got[2].Source)
	}
}
