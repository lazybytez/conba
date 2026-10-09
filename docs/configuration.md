# Configuration

Conba reads a YAML config file (default: `conba.yaml` in the working directory;
override with `-c/--config <path>`). Every value can also be supplied via an
environment variable.

## Environment overrides

Any config key maps to an environment variable by upper-casing it, replacing
dots with underscores, and prefixing `CONBA_`:

| Config key | Environment variable |
|------------|----------------------|
| `restic.repository` | `CONBA_RESTIC_REPOSITORY` |
| `restic.password` | `CONBA_RESTIC_PASSWORD` |
| `logging.level` | `CONBA_LOGGING_LEVEL` |
| `discovery.opt_in_only` | `CONBA_DISCOVERY_OPT_IN_ONLY` |
| `pre_backup_commands.enabled` | `CONBA_PRE_BACKUP_COMMANDS_ENABLED` |

Environment variables take precedence over the file. They are convenient for
secrets (`CONBA_RESTIC_PASSWORD`) and for the container image.

## Full example

```yaml
restic:
  repository: "s3:s3.amazonaws.com/my-bucket"
  password_file: "/run/secrets/restic-password"

runtime:
  type: docker
  docker:
    host: "unix:///var/run/docker.sock"

discovery:
  opt_in_only: false

retention:
  keep_daily: 7
  keep_weekly: 4
  keep_monthly: 6
  keep_yearly: 0

pre_backup_commands:
  enabled: false

logging:
  level: "info"
  format: "human"
```

## Reference

### `restic`

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `repository` | string | — (**required**) | restic repository URL (`s3:…`, `/path`, `sftp:…`, etc.). |
| `password` | string | — | Repository password. Provide this **or** `password_file`. |
| `password_file` | string | — | Path to a file containing the repository password. |
| `binary` | string | `restic` | Path/name of the restic binary to invoke. |
| `extra_args` | list | — | Extra arguments appended to every restic invocation. |
| `environment` | map | — | Extra environment variables for restic (e.g. cloud credentials, `RESTIC_CACHE_DIR`). |

One of `password` / `password_file` is required, as is `repository`. Missing
either fails fast at startup.

> Pre-backup (stream) commands require restic to spawn a child process, which
> needs a cache directory. When using that feature, set `RESTIC_CACHE_DIR` (and
> `HOME`) under `restic.environment`, and ensure `PATH` is set so restic can
> find the runtime. See [Database dumps](guides/database-dumps.md).

### `runtime`

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `type` | string | `docker` | Container runtime. Docker is the only supported type. |
| `docker.host` | string | `unix:///var/run/docker.sock` | Docker daemon address. |

### `discovery`

Controls which containers/volumes become backup targets. See
[Retention and filtering](guides/retention-and-filtering.md) for worked examples.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `opt_in_only` | bool | `false` | When `true`, only containers labelled `conba.enabled=true` are considered. |
| `include` | filter list | — | Force-include matching containers. |
| `exclude` | filter list | — | Exclude matching containers. |

A **filter list** has four fields, each a list of strings:

| Field | Matches on |
|-------|-----------|
| `names` | exact container name |
| `name_patterns` | container name (regular expression) |
| `ids` | exact container ID |
| `id_patterns` | container ID (regular expression) |

### `retention`

Global retention policy applied by `conba forget` (and the forget phase of
`conba run`). All default to `0` (= keep, i.e. that dimension is not enforced).
Per-container overrides are possible with the `conba.retention` label.

| Key | Type | Default |
|-----|------|---------|
| `keep_daily` | int | `0` |
| `keep_weekly` | int | `0` |
| `keep_monthly` | int | `0` |
| `keep_yearly` | int | `0` |

### `pre_backup_commands`

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `enabled` | bool | `false` | Master switch for label-driven pre-backup/restore commands. **Off by default** because it lets anyone who can set container labels make conba run shell commands inside those containers. |

When disabled (the default), all `conba.pre-backup.*` labels are ignored and
volume backups proceed as usual.

### `logging`

| Key | Type | Default | Values |
|-----|------|---------|--------|
| `level` | string | `info` | `debug`, `info`, `warn`, `error` |
| `format` | string | `human` | `human`, `json` |
