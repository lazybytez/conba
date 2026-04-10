package restic

import "time"

// Snapshot represents a single restic snapshot as returned by `restic snapshots --json`.
type Snapshot struct {
	ID       string    `json:"short_id"`
	Time     time.Time `json:"time"`
	Paths    []string  `json:"paths"`
	Tags     []string  `json:"tags"`
	Hostname string    `json:"hostname"`
}

// RepoStats holds repository statistics as returned by `restic stats --json`.
type RepoStats struct {
	TotalSize      uint64 `json:"total_size"`
	TotalFileCount uint64 `json:"total_file_count"`
}

// ForgetPolicy defines retention rules for restic forget operations.
type ForgetPolicy struct {
	KeepDaily   int
	KeepWeekly  int
	KeepMonthly int
	KeepYearly  int
}
