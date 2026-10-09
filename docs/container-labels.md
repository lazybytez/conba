# Container labels

Conba reads Docker labels off each container to drive per-container behaviour.
Set them in your `compose.yaml` (`labels:`) or `docker run --label`.

## Filtering & retention labels

These work regardless of configuration.

| Label | Values | Default | Description |
|-------|--------|---------|-------------|
| `conba.enabled` | `true`, `false` | — | Force include/exclude this container. Required to be `true` for a container to be backed up when `discovery.opt_in_only` is on. |
| `conba.retention` | `Nd,Nw,Nm,Ny` | global | Per-container retention override. Suffix-tagged, comma-separated, order-agnostic, case-insensitive. Suffixes: `d` daily, `w` weekly, `m` monthly, `y` yearly; missing components default to 0. Example: `7d,4w,6m,2y`. |
| `conba.exclude-volumes` | comma-separated | — | Volume names (`Mount.Name`) to exclude. For bind mounts this is the host source path (rarely portable — prefer `conba.exclude-mount-destinations`). |
| `conba.exclude-bind-mounts` | `true`, `false` | `false` | Exclude all of the container's bind mounts. Named volumes are unaffected. |
| `conba.exclude-mount-destinations` | comma-separated | — | Container-side destination paths to exclude (bind or volume). Matched exactly. Example: `/var/log,/etc/app/cache`. |

## Pre-backup / restore labels

These are **only honoured when `pre_backup_commands.enabled: true`** in the
config (off by default — see [Configuration](configuration.md#pre_backup_commands)).
They let conba run a command inside the container at backup/restore time. See
[Database dumps](guides/database-dumps.md) and [Restoring](guides/restore.md).

| Label | Values | Default | Description |
|-------|--------|---------|-------------|
| `conba.pre-backup.command` | shell command | — | Required to enable streaming for the container. Runs inside the container (`sh -c "<cmd>"`); its stdout is streamed into restic as a `kind=stream` snapshot. |
| `conba.pre-backup.mode` | `replace`, `alongside` | `replace` | `replace`: the stream snapshot substitutes the container's volume snapshots. `alongside`: produce the stream snapshot **and** the volume snapshots. |
| `conba.pre-backup.filename` | filename | labeled container name | The `--stdin-filename` restic records for the stream (e.g. `mysql.sql`). Used as the dump filename on restore. |
| `conba.pre-backup.restore-command` | shell command | — | Command `conba restore` pipes the dump into when restoring a stream snapshot and `--to-command` is not given. Runs inside the labeled container. |

### Trust note

Label-driven commands are a meaningful trust-surface change: anyone who can set
labels on a container can make conba execute shell strings inside it. That is
why the feature is opt-in via `pre_backup_commands.enabled`. The command is only
ever interpreted by the **in-container** shell — conba runs it through the Docker
API exec, never by building a host shell string.

## Example

```yaml
# compose.yaml
services:
  mysql:
    image: mysql:8
    volumes:
      - mysql-data:/var/lib/mysql
    labels:
      conba.pre-backup.command: "mysqldump --all-databases -uroot"
      conba.pre-backup.mode: "replace"
      conba.pre-backup.filename: "mysql.sql"
      conba.pre-backup.restore-command: "mysql -uroot"

volumes:
  mysql-data:
```
