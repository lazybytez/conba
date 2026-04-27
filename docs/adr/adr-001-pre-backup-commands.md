# ADR-001: Pre-backup commands

## Status

Accepted — 2026-04-26

## Context

Conba backs up Docker volumes by snapshotting their on-disk contents
through restic. For stateful services like databases, the at-rest files
are typically inconsistent: MySQL's InnoDB redo log, Postgres's WAL, and
Redis's append-only file are only safe to copy when the engine is
quiesced or when the data has been routed through the engine's own
export tool. Operators wanting a consistent database backup today have
no first-class way to express "before backing up this container, run
mysqldump and store the dump as the snapshot." They either accept
silently inconsistent volume snapshots, or layer their own scripting
around conba — which defeats the point of having a backup tool.

Restic already supports `--stdin-from-command`, which streams a
command's stdout into a snapshot. The remaining design question was how
operators should express the dump command and how that integrates with
the existing volume-backup orchestration. Three patterns were weighed:
container-level Docker labels (consistent with conba's existing label
model for excludes); a separate config-file section keyed by container
name (decouples definition from runtime); and one-shot sidecar
containers spun up at backup time (cleanest isolation, but introduces
a new container-lifecycle responsibility into the tool).

## Decision

We adopt **container-level Docker labels** under the `conba.pre-backup.*`
namespace as the configuration surface, with the following binding
choices:

1. **Labels on the container, not config-file entries.** Pre-backup
   commands live next to the workload they describe, matching the
   existing `conba.exclude-*` model. Operators already manage container
   labels through their orchestration tooling.
2. **`replace` mode is the default; `alongside` is opt-in.** Most
   stateful services need the dump to *replace* the inconsistent
   on-disk snapshot. `mode=alongside` exists for mixed workloads
   (a container with a DB plus a separate uploads volume) and produces
   N+1 snapshots per cycle.
3. **Per-target failure semantics match volume backups.** A failed dump
   command fails the target. The cycle continues with other targets and
   exits non-zero via `backup.ErrTargetsFailed`. Critically, **there is
   no fall-back to a volume backup when the dump fails** — a silent
   inconsistent snapshot is exactly what the operator labeled the
   container to avoid.
4. **The whole feature is opt-in at the config level.** A new
   `pre_backup_commands.enabled` key in `conba.yaml` defaults to `false`.
   When disabled, all `conba.pre-backup.*` labels are ignored and
   conba's behaviour is unchanged. Activating label-driven exec is an
   explicit operator choice.
5. **The command runs as `docker exec <container> sh -c "<user-string>"`.**
   The exec target defaults to the labeled container. The optional
   `conba.pre-backup.container=<name>` label redirects exec to a
   sidecar that must already be running.
6. **One-shot sidecar containers are deferred.** Spinning up a fresh
   container per backup (Pattern 3 from brainstorming) is explicitly
   out of scope for v1. The override-container label covers the
   sidecar use case for already-running admin containers without
   conba taking on container-lifecycle responsibility.

## Consequences

### Positive

- Consistent database backups become a first-class capability rather
  than something operators must script around conba.
- The label model is already familiar to operators using
  `conba.exclude-volumes` and `conba.exclude-bind-mounts` — no new
  configuration surface to learn.
- No new flags on `conba backup`. The CLI surface is unchanged.
- Reuses the existing per-target outcome model and
  `backup.ErrTargetsFailed` orchestration. A pre-backup target
  succeeds, fails, or is skipped exactly like a volume target.
- Operators can mix volume and stream backups within a single cycle,
  per container, with the existing `conba.exclude-*` labels remaining
  orthogonal.

### Negative

- Label-driven `docker exec` is a qualitative change in conba's trust
  surface. Anyone able to set labels on a container can cause conba
  to execute arbitrary shell strings inside that container during the
  backup cycle. In environments where container labels are managed by
  a less-trusted tier than the backup operator, this matters.

### Mitigations

- The `pre_backup_commands.enabled: false` default means the new trust
  surface is dormant unless an operator explicitly opts in. Clusters
  that do not need dump-style backups never expose it.
- Per-mount granularity within a `mode=alongside` target is not a new
  concept: operators reuse `conba.exclude-volumes`,
  `conba.exclude-bind-mounts`, and `conba.exclude-mount-destinations`
  to skip mounts already covered by the dump. No new exclusion
  vocabulary is introduced.
- Deferring one-shot sidecars keeps the trust surface bounded to
  containers the operator is already running. Conba does not gain the
  ability to start new containers as a side effect of a backup cycle.

## References

- Spec: [.pz-ai-tools/specs/pre-backup-commands-2026-04-26.md](../../.pz-ai-tools/specs/pre-backup-commands-2026-04-26.md)
