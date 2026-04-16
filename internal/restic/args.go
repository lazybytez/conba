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

// BuildSnapshotArgs returns the argument slice for listing snapshots
// with optional tag filtering.
func BuildSnapshotArgs(tags []string) []string {
	args := []string{"snapshots", "--json"}
	args = appendTags(args, tags)

	return args
}

// BuildForgetArgs returns the argument slice for a forget-and-prune operation
// with optional tags and retention policy. Only non-zero policy values produce
// the corresponding --keep-* flag.
func BuildForgetArgs(tags []string, policy ForgetPolicy) []string {
	args := []string{"forget", "--prune", "--json"}
	args = appendTags(args, tags)
	args = appendKeep(args, "--keep-daily", policy.KeepDaily)
	args = appendKeep(args, "--keep-weekly", policy.KeepWeekly)
	args = appendKeep(args, "--keep-monthly", policy.KeepMonthly)
	args = appendKeep(args, "--keep-yearly", policy.KeepYearly)

	return args
}

// BuildUnlockArgs returns the argument slice for unlocking a restic repository.
func BuildUnlockArgs() []string {
	return []string{"unlock"}
}

// BuildStatsArgs returns the argument slice for retrieving repository statistics.
func BuildStatsArgs() []string {
	return []string{"stats", "--json"}
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
