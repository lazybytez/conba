# ADR-002: Restore command

## Status

Accepted -- 2026-04-28

## Context

Conba already produces two kinds of restic snapshots: file snapshots of
Docker volumes, and stream snapshots produced by piping the stdout of a
pre-backup command (e.g. `mysqldump`) into restic via
`--stdin-from-command`. Both are addressable through `conba snapshots`,
but operators currently have no first-class way to **get the data back
out**. Disaster recovery, point-in-time rollback, test restores, and
migrations all force operators to invoke `restic restore` and
`restic dump` directly against the underlying repository, then wire up
their own `docker exec -i` plumbing for stream snapshots.

That defeats the abstraction conba is meant to provide. Operators must
know the exact snapshot ID, the filename embedded in stream snapshots
(matching `Spec.Filename` from the original `pre-backup.command`), the
`docker exec` invocation needed to stream a dump back into the target
container, and the host-path conventions used by Docker volumes. None
of this is conba-specific knowledge -- it is restic and Docker plumbing
that the backup-side commands have already abstracted away.

This ADR captures the design of a `conba restore` command that
mirrors backup-side modes symmetrically while keeping the trust
surface and the operator-owned container lifecycle aligned with
ADR-001.

## Decision

We add a single `conba restore` command that auto-detects the restore
mode from the resolved snapshot's tags. Operators describe *what* to
restore via flags; conba picks the correct restic primitive
(`restic restore` for volume snapshots, `restic dump` piped into
`docker exec -i` for stream snapshots). Five binding choices:

1. **Single command with auto-detected mode.** There is one `conba
   restore` subcommand. Conba inspects the resolved snapshot's tags:
   a `kind=stream` tag means stream mode, otherwise volume mode.
   Wrong-flag combinations are loud errors rather than silent
   coercions -- `--volume` against a stream snapshot, `--to-command`
   against a volume snapshot, and `--volume` together with
   `--to-command` all exit non-zero with an explicit "wrong-mode flag
   combination" message. A single command keeps the operator-facing
   surface small and matches how `conba backup` already dispatches
   between volume and stream targets without operator input.
2. **Stream-restore command source: CLI flag or label.** The restore
   command for stream mode comes from one of two places: the
   `--to-command "<cmd>"` CLI flag, or a new
   `conba.pre-backup.restore-command=<cmd>` label on the target
   container. The CLI flag wins when both are set. If neither is set
   for a stream snapshot, conba refuses with an explicit "no restore
   command available" error -- restore is destructive, so we do not
   silently fall through. The label form is **locked to the labeled
   container**: there is no `conba.pre-backup.restore-container`
   override. The asymmetry with the backup-side
   `conba.pre-backup.container` label is deliberate: at restore time
   the dump must go *into* the live target container, not into a
   sidecar admin container. Sidecar restores are out of scope for v1
   and can be expressed today via `--to-command` against the operator's
   chosen container.
3. **Volume-restore destination: `--to <host-path>` only.** Volume
   restores require `--to <host-path>` as an absolute host filesystem
   path. Conba does **no** Docker-volume name resolution -- the
   operator types the path, e.g.
   `/var/lib/docker/volumes/<vol>/_data` to overwrite the live volume
   or `/tmp/recovered/<vol>` for a safe sidecar copy. If the
   destination exists and is non-empty, conba refuses unless
   `--force` is passed. The explicit path makes the destructive
   nature of a restore visible in the command line and prevents
   surprises from name-resolution magic.
4. **Snapshot selection: latest-by-tag, with overrides.** By default
   conba selects the most recent snapshot whose tags match
   `container=<name>` plus either `volume=<name>` (volume mode) or
   `kind=stream` (stream mode), filtered by the local hostname.
   `--snapshot <id>` overrides the "latest" selection with an
   explicit snapshot ID. `--all-hosts` removes the host-tag filter
   for cross-host recovery scenarios. Point-in-time selection by
   wall-clock date is deferred -- operators use `conba snapshots` to
   pick an ID and pass it via `--snapshot`.
5. **Operator owns container lifecycle.** Conba does not stop or
   start containers as part of a restore. Stream restore requires
   the target container to be running (it is a `docker exec`
   requirement); conba verifies this and refuses with a clear
   "container <name> is not running" error otherwise. Volume restore
   makes no container demands -- restoring into a path mounted by a
   running container is the operator's call. Both modes support
   `--dry-run` to print the planned action without invoking restic
   or docker.

## Consequences

### Positive

- Restore becomes a first-class capability symmetric with the two
  existing backup modes. Operators no longer need to know the restic
  argv or the `docker exec -i` plumbing to recover data.
- Reuses the existing `conba.pre-backup.*` label namespace for the
  restore-command source, keeping the configuration surface familiar
  to operators already running label-driven backups.
- Reuses the existing `pre_backup_commands.enabled` feature flag
  (introduced in ADR-001) to gate the label-driven restore path.
  When the flag is false, the `conba.pre-backup.restore-command`
  label is ignored exactly as the backup-side `command` label is --
  one switch governs both directions.
- Explicit `--to <host-path>` prevents accidental overwrites from
  name-resolution surprises; `--force` makes the destructive
  intent visible in shell history and audit logs.
- `--dry-run` is supported in both modes, so operators can verify
  the planned action before performing a destructive restore.

### Negative

- Destructive overwrite is now possible via `--force`. An operator
  who passes `--force` against a populated host path causes data
  loss without any further prompt. This is a deliberate trade-off
  against silent refusal -- restore is destructive by definition.
- Arbitrary command execution into a target container is now
  possible via `--to-command` or via the
  `conba.pre-backup.restore-command` label. The label form expands
  the trust surface that ADR-001 already documents for backup-side
  pre-backup commands: anyone who can set labels on a container can
  cause conba to execute arbitrary shell strings inside that
  container during a restore.

### Mitigations

- The label form of restore is gated by the same
  `pre_backup_commands.enabled` feature flag as the backup-side
  pre-backup command. Clusters that have not opted in to label-driven
  exec for backups never expose the label-driven restore path either.
- The CLI flag `--to-command` is always available, but it requires
  an explicit operator decision at restore time. There is no
  scenario in which a malicious label can trigger a restore without
  an operator running `conba restore`.
- Volume `--force` is required to overwrite a non-empty destination.
  The default behaviour is to refuse with a clear error, so the
  destructive path is opt-in per invocation rather than implicit.
- The restore-command label is locked to the labeled container --
  no sidecar override exists. Operators who need to restore into a
  different container do so explicitly via `--to-command` against
  that container, which keeps the dump destination visible in the
  command line.

### Trade-offs and deferred work

- Sidecar restore (one-shot containers, or a
  `conba.pre-backup.restore-container` label override) is explicitly
  out of scope for v1. Operators who need it can DIY via
  `--to-command` against an already-running admin container.
- Automatic container stop/start around a restore is not provided.
  Operators decide whether to stop the target container before a
  volume restore, restart it after a stream restore, or accept the
  risk of restoring into a live mount.
- A Docker-volume name shortcut (e.g. `--to-volume <name>`) is not
  provided. The mapping from volume name to host path is trivial
  for operators who run conba; making it implicit would hide the
  destructive target.
- Point-in-time selection by wall-clock date (e.g. `--at <date>`)
  is not provided. Operators use `conba snapshots` to enumerate
  candidates and pass the chosen ID via `--snapshot`.

## References

- Spec: [.pz-ai-tools/specs/restore-command-2026-04-28.md](../../.pz-ai-tools/specs/restore-command-2026-04-28.md)
- ADR-001: [adr-001-pre-backup-commands.md](adr-001-pre-backup-commands.md) --
  introduces the `conba.pre-backup.*` label namespace and the
  `pre_backup_commands.enabled` feature flag that gate the
  label-driven restore path described here.
