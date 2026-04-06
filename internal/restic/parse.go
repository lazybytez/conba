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
