package restic

import (
	"encoding/json"
	"fmt"
)

// ParseSnapshots unmarshals a JSON array of restic snapshots.
func ParseSnapshots(data []byte) ([]Snapshot, error) {
	var snapshots []Snapshot

	err := json.Unmarshal(data, &snapshots)
	if err != nil {
		return nil, fmt.Errorf("parsing snapshots: %w", err)
	}

	return snapshots, nil
}

// ParseStats deserializes the JSON output of `restic stats --json`.
func ParseStats(data []byte) (RepoStats, error) {
	var stats RepoStats

	err := json.Unmarshal(data, &stats)
	if err != nil {
		return RepoStats{}, fmt.Errorf("parsing stats: %w", err)
	}

	return stats, nil
}
