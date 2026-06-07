# Automation (Ansible, monitoring, machine output)

Conba is built to run both interactively and unattended. When dispatched as a
container (for example by Ansible) and scraped by a log monitor, two features
matter: the **JSON event stream** and **differentiated exit codes**.

## Output modes

Command results go to **stdout**; the diagnostic logger goes to **stderr**.
The result format is chosen by `output.format` (config) or `--output` (flag):

- `text` — human-readable lines and tables (colored on a terminal).
- `json` — newline-delimited JSON (NDJSON), one object per event.
- `auto` (default) — `text` on a terminal, `json` otherwise.

Because the default is `auto`, a conba container with a non-TTY stdout (the
normal case under Ansible/cron/CI) emits JSON automatically. No flag needed.
To force it, set `output.format: json`, `CONBA_OUTPUT_FORMAT=json`, or pass
`--output json`.

## Exit codes

Branch automation on the exit code rather than parsing text:

| Code | Meaning | Typical reaction |
|------|---------|------------------|
| `0` | Success | continue |
| `1` | Config / precondition error (bad config, repo locked or not initialised) | fix configuration; do not retry blindly |
| `2` | Partial failure — some targets succeeded, some failed | alert; the successful targets are backed up |
| `3` | Total failure or unclassified fatal error | alert; investigate |

## Event schema

Every JSON line is a flat object with three always-present keys:

| Field | Meaning |
|-------|---------|
| `time` | RFC3339 UTC timestamp |
| `level` | `info`, `warn`, or `error` |
| `event` | dotted event name (see below) |

Event-specific fields are added per event. A log monitor can filter on
`level` for problems and on `event` for structure. Secrets (the restic
password, password-file contents, `restic.environment` values) never appear
in events.

| Command | Events |
|---------|--------|
| `init` | `init.done` |
| `unlock` | `unlock.done` |
| `verify` | `verify.done` (`read_data`) |
| `backup` | `backup.target` (`container`, `volume`, `outcome`, `reason`/`error`), `backup.stream`, `backup.summary` (`succeeded`, `skipped`, `failed`); `backup.plan` / `backup.plan.summary` under `--dry-run` |
| `forget` | `forget.target`, `forget.summary` (`succeeded`, `skipped`, `failed`, `dry_run`); `forget.surgical` for the tag-filtered path |
| `run` | `run.phase` (`phase`) plus the `init`/`backup`/`forget` events above |
| `snapshots` | `snapshot` (`id`, `time`, `container`, `volume`, `hostname`, `tags`) per row, then `snapshots.summary` (`count`) |
| `inspect` | `inspect.target` (`container`, `volume`, `included`, `reason`) per row, then `inspect.summary` |
| `diff` | `diff.change` (`path`, `modifier`) per change, then `diff.summary` |
| `status` | `repo.status` (`repository`, `state`, `snapshots`, `latest`, `total_size`) |
| `restore` | `restore.plan` under `--dry-run`, `restore.done` on success |
| `version` | `version` (`version`, `go`, `restic`) |
| any fatal error | `fatal` (`error`) on stdout in json mode |

Example (`conba backup --output json`):

```json
{"time":"2026-06-07T20:11:03Z","level":"info","event":"backup.target","container":"mysql","volume":"mysql-data","outcome":"success"}
{"time":"2026-06-07T20:11:04Z","level":"warn","event":"backup.target","container":"app","volume":"logs","outcome":"skipped","reason":"source unreadable"}
{"time":"2026-06-07T20:11:05Z","level":"info","event":"backup.summary","succeeded":3,"skipped":1,"failed":0}
```

## Dispatching with Ansible

Run the backup cycle as a one-shot `--rm` container. `community.docker` reports
a non-zero exit as a task failure, so the exit codes above drive your handlers:

```yaml
- name: Run conba backup cycle
  community.docker.docker_container:
    name: conba-run
    image: ghcr.io/lazybytez/conba:edge
    command: run
    detach: false
    cleanup: true
    env:
      CONBA_OUTPUT_FORMAT: json
      CONBA_RESTIC_REPOSITORY: "{{ restic_repository }}"
      CONBA_RESTIC_PASSWORD: "{{ restic_password }}"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - /var/lib/docker/volumes:/var/lib/docker/volumes:ro
  register: conba_run
  failed_when: conba_run.status not in [0, 2]   # tolerate partial, fail on total
```

The container's combined stdout/stderr is captured by your log driver; point
your monitor at it and alert on `level: "error"` events or on exit codes `2`
and `3`.

## Monitoring tips

- Parse stdout as NDJSON; alert on any object with `"level":"error"`.
- A completed cycle always ends with a `*.summary` event — its absence means
  the process died mid-run.
- Treat exit `1` as "operator action required" (config), `2`/`3` as "backup
  health" alerts.
