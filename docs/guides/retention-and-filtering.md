# Retention and filtering

Two related questions: **what gets backed up** (filtering) and **how long
snapshots are kept** (retention).

## Filtering — what gets backed up

By default conba discovers every running container and backs up all of its volume
mounts. You narrow that with config-level filters and per-container labels.

### Opt-in mode

To back up nothing unless explicitly enabled:

```yaml
discovery:
  opt_in_only: true
```

Then only containers labelled `conba.enabled=true` are considered.

### Include / exclude by name or ID

```yaml
discovery:
  exclude:
    names: ["jenkins"]
    name_patterns: ["^tmp-.*"]
  include:
    names: ["important-db"]
```

Each of `include`/`exclude` accepts `names`, `name_patterns` (regex), `ids`, and
`id_patterns` (regex). The `conba.enabled` label overrides these per container.

### Excluding specific mounts

Even on an included container, you can drop individual mounts via labels:

| Label | Effect |
|-------|--------|
| `conba.exclude-volumes` | Exclude named volumes (by `Mount.Name`). |
| `conba.exclude-mount-destinations` | Exclude by container-side path (e.g. `/var/log`). Best for bind mounts. |
| `conba.exclude-bind-mounts: true` | Exclude all bind mounts on the container. |

See [Container labels](../container-labels.md) for the full reference.

### Preview before backing up

```sh
conba inspect
```

Lists **Included** targets and **Excluded** ones (with the reason each was
dropped). Run this whenever you change filters.

### Bind mounts

Two things to know:

1. **Labels match the destination path.** Use the container-side destination in
   `conba.exclude-mount-destinations`, not the host source — destinations are
   portable across hosts, sources are not.
2. **Conba must be able to read the source.** When conba runs in a container, the
   host source of every bind mount you want backed up must be visible inside
   conba's container at the same path:

   ```sh
   docker run --rm \
     ...existing mounts... \
     -v /srv/myapp/data:/srv/myapp/data:ro \
     ghcr.io/lazybytez/conba:edge backup
   ```

   If the source isn't reachable, conba logs
   `WARN: skipping <container>/<destination>: source unreadable (...)` and
   continues with the remaining targets.

## Retention — how long snapshots are kept

Retention is applied by [`conba forget`](../commands.md#forget) (and the forget
phase of [`conba run`](../commands.md#run)). Set a global policy:

```yaml
retention:
  keep_daily: 7
  keep_weekly: 4
  keep_monthly: 6
  keep_yearly: 0
```

Any dimension left at `0` is not enforced. `forget` prunes by default; use
`--no-prune` to skip the (slow) prune step, and always preview with `--dry-run`:

```sh
conba forget --dry-run
conba forget
```

### Per-container overrides

A container can override the global policy with the `conba.retention` label —
suffix-tagged, comma-separated, order-agnostic, case-insensitive:

```yaml
labels:
  conba.retention: "7d,4w,6m,2y"   # daily, weekly, monthly, yearly
```

Missing components default to 0. This takes precedence over the global
`retention:` policy for that container.

### Scoping a forget run

`forget` is host-scoped by default (only this machine's snapshots). Adjust with:

- `--all-hosts` — apply across every hostname.
- `--container <name>` / `--volume <name>` / `--tag <tag>` — restrict the set.
