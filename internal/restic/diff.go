package restic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
)

// restic diff --json message_type values.
const (
	diffMsgChange     = "change"
	diffMsgStatistics = "statistics"
)

// Scanner buffer sizing for diff JSON lines (paths can be long).
const (
	diffScanInitial = 64 * 1024
	diffScanMax     = 1024 * 1024
)

// DiffChange is a single path-level change between two snapshots. Modifier
// is restic's change marker ("+", "-", "M", "U", "T").
type DiffChange struct {
	Path     string
	Modifier string
}

// DiffSide aggregates one direction (added or removed) of a diff.
type DiffSide struct {
	Files int64 `json:"files"`
	Dirs  int64 `json:"dirs"`
	Bytes int64 `json:"bytes"`
}

// DiffStats is the summary record restic emits at the end of a diff.
type DiffStats struct {
	ChangedFiles int64
	Added        DiffSide
	Removed      DiffSide
}

type diffRecord struct {
	MessageType  string   `json:"message_type"`
	Path         string   `json:"path"`
	Modifier     string   `json:"modifier"`
	ChangedFiles int64    `json:"changed_files"`
	Added        DiffSide `json:"added"`
	Removed      DiffSide `json:"removed"`
}

// Diff compares two snapshots and returns restic's human diff output as
// bytes. snapA and snapB may be full IDs, short IDs, or the literal
// "latest".
func (c *Client) Diff(ctx context.Context, snapA, snapB string) ([]byte, error) {
	out, err := c.run(ctx, BuildDiffArgs(snapA, snapB, false))
	if err != nil {
		return nil, fmt.Errorf("restic diff: %w", err)
	}

	return out, nil
}

// DiffChanges compares two snapshots and returns the structured changes and
// summary statistics, parsed from restic's --json output.
func (c *Client) DiffChanges(
	ctx context.Context, snapA, snapB string,
) ([]DiffChange, DiffStats, error) {
	out, err := c.run(ctx, BuildDiffArgs(snapA, snapB, true))
	if err != nil {
		return nil, DiffStats{}, fmt.Errorf("restic diff: %w", err)
	}

	changes, stats, err := parseDiffJSON(out)
	if err != nil {
		return nil, DiffStats{}, fmt.Errorf("parse restic diff json: %w", err)
	}

	return changes, stats, nil
}

func parseDiffJSON(data []byte) ([]DiffChange, DiffStats, error) {
	var (
		changes []DiffChange
		stats   DiffStats
	)

	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, diffScanInitial), diffScanMax)

	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}

		var record diffRecord

		err := json.Unmarshal(line, &record)
		if err != nil {
			return nil, DiffStats{}, fmt.Errorf("decode diff line: %w", err)
		}

		switch record.MessageType {
		case diffMsgChange:
			changes = append(changes, DiffChange{
				Path:     record.Path,
				Modifier: record.Modifier,
			})
		case diffMsgStatistics:
			stats = DiffStats{
				ChangedFiles: record.ChangedFiles,
				Added:        record.Added,
				Removed:      record.Removed,
			}
		}
	}

	err := scanner.Err()
	if err != nil {
		return nil, DiffStats{}, fmt.Errorf("scan diff output: %w", err)
	}

	return changes, stats, nil
}
