package restic_test

import (
	"testing"

	"github.com/lazybytez/conba/internal/restic"
)

// Shape captured from restic 0.18.1 `diff --json`; the statistics record is
// trimmed to the fields the parser reads (restic emits more, which we ignore).
const sampleDiffJSON = `{"message_type":"change","path":"/tmp/d/b.txt","modifier":"+"}
{"message_type":"change","path":"/tmp/d/a.txt","modifier":"-"}
{"message_type":"statistics","changed_files":0,"added":{"files":1,"bytes":1390},"removed":{"files":0,"bytes":1047}}
`

func TestParseDiffJSON(t *testing.T) {
	t.Parallel()

	changes, stats, err := restic.ParseDiffJSON([]byte(sampleDiffJSON))
	if err != nil {
		t.Fatalf("ParseDiffJSON: %v", err)
	}

	if len(changes) != 2 {
		t.Fatalf("changes = %d, want 2", len(changes))
	}

	if changes[0].Path != "/tmp/d/b.txt" || changes[0].Modifier != "+" {
		t.Errorf("change[0] = %+v, want +/b.txt", changes[0])
	}

	if changes[1].Modifier != "-" {
		t.Errorf("change[1] modifier = %q, want -", changes[1].Modifier)
	}

	if stats.Added.Files != 1 || stats.Added.Bytes != 1390 {
		t.Errorf("added = %+v, want files=1 bytes=1390", stats.Added)
	}

	if stats.Removed.Bytes != 1047 {
		t.Errorf("removed bytes = %d, want 1047", stats.Removed.Bytes)
	}
}

func TestParseDiffJSON_EmptyAndBlankLines(t *testing.T) {
	t.Parallel()

	changes, _, err := restic.ParseDiffJSON([]byte("\n\n"))
	if err != nil {
		t.Fatalf("ParseDiffJSON on blank input: %v", err)
	}

	if len(changes) != 0 {
		t.Errorf("changes = %d, want 0", len(changes))
	}
}
