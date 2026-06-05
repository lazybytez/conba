package restic

import (
	"strconv"
	"strings"
)

// BuildInitArgs returns the argument slice for initialising a restic repository.
func BuildInitArgs() []string {
	return []string{"init"}
}

// BuildBackupArgs returns the argument slice for backing up the given path
// with optional tags.
func BuildBackupArgs(path string, tags []string) []string {
	args := []string{"backup", path, "--json"}
	args = appendTags(args, tags)

	return args
}

// BuildBackupFromStdinArgs returns the argument slice for backing up data
// piped to restic's stdin via --stdin, naming the snapshot after filename
// and applying optional tags.
//
// restic does not spawn the source process: conba attaches the reader as
// the restic subprocess stdin.
func BuildBackupFromStdinArgs(filename string, tags []string) []string {
	args := []string{
		"backup",
		"--stdin",
		"--stdin-filename=" + filename,
	}
	args = appendTags(args, tags)

	return args
}

// BuildSnapshotArgs returns the argument slice for listing snapshots
// with optional tag filtering.
func BuildSnapshotArgs(tags []string) []string {
	args := []string{"snapshots", "--json"}
	args = appendTags(args, tags)

	return args
}

// BuildForgetArgs returns the argument slice for a forget operation with
// optional tags and retention policy. Only non-zero policy values produce the
// corresponding --keep-* flag. When opts.Prune is true, --prune is appended so
// restic reclaims disk space; when opts.DryRun is true, --dry-run is appended
// so restic reports what would be forgotten without applying changes.
func BuildForgetArgs(tags []string, policy ForgetPolicy, opts ForgetOptions) []string {
	args := []string{"forget", "--json"}
	args = appendTags(args, tags)
	args = appendKeep(args, "--keep-daily", policy.KeepDaily)
	args = appendKeep(args, "--keep-weekly", policy.KeepWeekly)
	args = appendKeep(args, "--keep-monthly", policy.KeepMonthly)
	args = appendKeep(args, "--keep-yearly", policy.KeepYearly)

	if opts.Prune {
		args = append(args, "--prune")
	}

	if opts.DryRun {
		args = append(args, "--dry-run")
	}

	return args
}

// BuildRestoreArgs returns the argument slice for restoring snapshotID into
// targetPath. When dryRun is true, --dry-run is appended so restic reports
// what it would restore without writing files.
func BuildRestoreArgs(snapshotID string, targetPath string, dryRun bool) []string {
	args := []string{"restore", snapshotID, "--target", targetPath}
	if dryRun {
		args = append(args, "--dry-run")
	}

	return args
}

// BuildDumpArgs returns the argument slice for dumping a single file from
// snapshotID to stdout. The filename is the path inside the snapshot to dump.
func BuildDumpArgs(snapshotID string, filename string) []string {
	return []string{"dump", snapshotID, filename}
}

// BuildUnlockArgs returns the argument slice for unlocking a restic repository.
func BuildUnlockArgs() []string {
	return []string{"unlock"}
}

// BuildCheckArgs returns the argument slice for verifying repository
// integrity. When readData is true, all data blobs are read and verified
// (slow but exhaustive); otherwise only repository structure is checked.
func BuildCheckArgs(readData bool) []string {
	args := []string{"check"}
	if readData {
		args = append(args, "--read-data")
	}

	return args
}

// BuildStatsArgs returns the argument slice for retrieving repository statistics.
func BuildStatsArgs() []string {
	return []string{"stats", "--json"}
}

// BuildDiffArgs returns the argument slice for diffing two snapshots.
// Snapshot identifiers may be full IDs, short IDs, or the literal "latest".
func BuildDiffArgs(snapA, snapB string) []string {
	return []string{"diff", snapA, snapB}
}

// appendTags joins all tags into a single --tag flag. Restic treats values
// within one --tag (comma-separated) as AND, and repeated --tag flags as OR;
// conba always wants AND semantics for filtering, and backup accepts the
// same comma form as additive tags.
func appendTags(args []string, tags []string) []string {
	if len(tags) == 0 {
		return args
	}

	return append(args, "--tag", strings.Join(tags, ","))
}

func appendKeep(args []string, flag string, value int) []string {
	if value <= 0 {
		return args
	}

	return append(args, flag, strconv.Itoa(value))
}
