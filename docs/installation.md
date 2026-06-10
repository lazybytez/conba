# Installation

Conba needs three things at runtime:

1. **Access to the Docker daemon** (to discover containers and their volumes).
2. **A restic binary** — installed separately when running the host binary;
   **bundled** in the container image.
3. **Read access to the volume data** being backed up.

## Option A — Container image (recommended)

The published image bundles a pinned restic, so there is nothing else to install.

```sh
docker run --rm -it \
  --hostname "$(hostname)" \
  -v "$PWD/conba.yaml:/app/conba.yaml:ro" \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /var/lib/docker/volumes:/var/lib/docker/volumes:ro \
  ghcr.io/lazybytez/conba:edge \
  backup --dry-run
```

Drop `--dry-run` to execute. Notes on the mounts:

- `--hostname "$(hostname)"` makes snapshots carry the real host's name (conba
  tags every snapshot with the hostname) instead of a random container ID.
- The Docker socket lets conba discover running containers.
- `/var/lib/docker/volumes` exposes named-volume contents so they can be read
  for snapshotting (read-only is fine).
- `conba.yaml` is mounted to `/app/conba.yaml`, the default lookup path.

For **bind mounts**, the host source path must also be visible inside conba's
container at the same path — see
[Retention and filtering](guides/retention-and-filtering.md#bind-mounts).

## Option B — Build from source

All build operations run inside Docker via Make — no local Go toolchain needed.

```sh
git clone https://github.com/lazybytez/conba.git
cd conba
make build        # produces ./bin/conba
./bin/conba version
```

When running the host binary, install [restic](https://restic.net/) yourself and
ensure it is on `PATH` (or set `restic.binary` in the config).

## Verify the install

```sh
conba version     # prints conba, go, and restic versions
```

Next: [Getting started](guides/getting-started.md).
