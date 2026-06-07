# Getting started

This walks through a first backup of your running containers' volumes to a local
restic repository. See [Installation](../installation.md) to get the binary or
image first.

## 1. Write a config

Create `conba.yaml` next to where you'll run conba:

```yaml
restic:
  repository: "/srv/conba-repo"        # any restic repo URL works (s3:, sftp:, …)
  password: "change-me"                # or password_file: /run/secrets/...

retention:
  keep_daily: 7
  keep_weekly: 4

logging:
  level: info

output:
  format: auto   # text on a terminal, json when piped/containerized
```

Prefer to keep the password out of the file? Drop `password` and export
`CONBA_RESTIC_PASSWORD` instead (see [Configuration](../configuration.md#environment-overrides)).

## 2. Initialise the repository

```sh
conba init
```

## 3. Preview what will be backed up

```sh
conba inspect        # lists Included / Excluded containers and volumes
conba backup --dry-run
```

`inspect` is the fastest way to confirm your [filters](retention-and-filtering.md)
select the right targets before you write any data.

## 4. Back up

```sh
conba backup
```

Each eligible volume becomes a snapshot tagged with its container, volume, and
hostname.

## 5. List what you have

```sh
conba snapshots
conba snapshots --container mysql      # filter
```

## 6. Run it on a schedule (on demand)

Conba has **no built-in scheduler** — it runs when invoked. The `run` command
does init + backup + forget in one shot, which is ideal for cron, a systemd
timer, or CI:

```sh
conba run                # init (if needed) + backup + apply retention
conba run --dry-run      # see the whole cycle first
```

Example cron entry (daily at 02:00):

```cron
0 2 * * * cd /srv/conba && /usr/local/bin/conba run >> /var/log/conba.log 2>&1
```

## Next steps

- [Database dumps](database-dumps.md) — consistent DB backups via `mysqldump`.
- [Restoring](restore.md) — get your data back.
- [Retention and filtering](retention-and-filtering.md) — fine-tune scope and
  snapshot lifetime.
- [Verify](../commands.md#verify) the repository periodically: `conba verify`.
