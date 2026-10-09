# Restoring

`conba restore` recovers data from an existing snapshot. One command handles both
kinds of snapshot: it inspects the resolved snapshot's `kind` tag and picks the
right restic primitive (`restic restore` for volume snapshots, `restic dump`
piped into an in-container command for stream snapshots — through the Docker API,
no `docker` CLI required).

You describe **what** to restore via flags; conba selects the **latest** matching
snapshot by default. Use `conba snapshots` to enumerate candidates and
`--snapshot <id>` for a point-in-time restore. `--all-hosts` drops the hostname
filter. `--container` is always required. If multiple volume snapshots match,
conba asks you to disambiguate with `--volume`.

> The operator owns the container lifecycle. Conba never stops or starts
> containers. For a volume restore that overwrites a live volume, stop the
> container first.

## Volume snapshots

Restore the latest `mysql-data` volume snapshot into a host directory for
inspection:

```sh
conba restore --container mysql --volume mysql-data --to /tmp/recovered
```

- `--to <dir>` is required for volume restores.
- A non-empty destination is refused unless you pass `--force`.

## Stream snapshots

A stream snapshot (e.g. a `mysqldump`) is restored by piping the dump back into a
command **inside a running container**. The target container must be running
(you cannot exec into a stopped container); conba refuses with a clear error
otherwise.

The restore command comes from either source — the CLI flag wins if both are set:

**Via flag** (always available):

```sh
conba restore --container mysql \
  --to-command 'mysql -uroot -p"$MYSQL_ROOT_PASSWORD"'
```

**Via label** (needs `pre_backup_commands.enabled: true`): if the container
carries `conba.pre-backup.restore-command`, you can omit `--to-command`:

```sh
conba restore --container mysql
```

If a stream snapshot is selected and neither `--to-command` nor the label is
available, conba errors and names both options.

## Dry run

`--dry-run` prints the planned action and invokes neither restic nor the
in-container command (for stream mode it still checks the container is running):

```sh
conba restore --container mysql --volume mysql-data --to /tmp/recovered --dry-run
```

## Flag summary

See [`restore` in the command reference](../commands.md#restore) for the full
flag list.
