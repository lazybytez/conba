# Database dumps (streaming backups)

Stateful services like databases produce inconsistent on-disk files unless they
are quiesced or exported through the engine's own tool. Instead of snapshotting
raw volume files, conba can run a command **inside** the container at backup time
and stream its stdout straight into a restic snapshot — for example `mysqldump`
piped directly into a snapshot tagged for the MySQL container.

## Enable the feature

Label-driven command execution is **off by default** because it lets anyone who
can set container labels make conba run shell commands inside the container. Turn
it on explicitly:

```yaml
# conba.yaml
pre_backup_commands:
  enabled: true
```

restic spawns the dump as a child process, which needs a cache directory. When
this feature is on, make sure restic has one:

```yaml
restic:
  environment:
    RESTIC_CACHE_DIR: /var/cache/restic
    HOME: /root
    PATH: /usr/local/bin:/usr/bin:/bin
```

## Label the container

```yaml
# compose.yaml
services:
  mysql:
    image: mysql:8
    environment:
      MYSQL_ROOT_PASSWORD: example
    volumes:
      - mysql-data:/var/lib/mysql
    labels:
      conba.pre-backup.command: 'mysqldump --all-databases -uroot -p"$MYSQL_ROOT_PASSWORD"'
      conba.pre-backup.mode: "replace"
      conba.pre-backup.filename: "mysql.sql"
      conba.pre-backup.restore-command: 'mysql -uroot -p"$MYSQL_ROOT_PASSWORD"'

volumes:
  mysql-data:
```

The command runs as `sh -c "<command>"` inside the container, so shell features
(env-var expansion, pipes) work and are evaluated in the container's environment.

## Modes

| Mode | Result |
|------|--------|
| `replace` (default) | Only the **stream** snapshot is produced; the container's volume snapshots are skipped. Use this for databases where the raw files are not worth backing up. |
| `alongside` | Produce the stream snapshot **and** the normal volume snapshots. |

## Run it

```sh
conba backup --dry-run    # shows "would run: <cmd> in <container>"
conba backup
conba snapshots --container mysql   # you'll see a kind=stream snapshot
```

The same applies to any tool: `pg_dumpall` for PostgreSQL, `mongodump --archive`
for MongoDB, `redis-cli --rdb -` for Redis, etc. Any command that writes the
backup to stdout works.

## Failure safety

A stream snapshot is only finalised if the command exits 0. If the dump command
fails (non-zero exit) — even after emitting partial output — conba aborts the
restic side so **no truncated snapshot is stored**, and the overall backup cycle
reports the failure while continuing with the remaining targets.

## Restoring a dump

See [Restoring → stream snapshots](restore.md#stream-snapshots). In short, conba
pipes `restic dump` back into an in-container command (`--to-command` or the
`conba.pre-backup.restore-command` label), e.g. `mysql -uroot`.
