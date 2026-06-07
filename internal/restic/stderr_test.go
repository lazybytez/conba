package restic_test

import (
	"strings"
	"testing"

	"github.com/lazybytez/conba/internal/restic"
)

func TestSanitizeStderr_RedactsCredentials(t *testing.T) {
	t.Parallel()

	in := []byte("  Fatal: repo at s3:https://KEY:SECRET@host/bucket unreachable\n")

	got := restic.SanitizeStderr(in)

	if strings.Contains(got, "SECRET") {
		t.Errorf("sanitized stderr still contains the secret: %q", got)
	}

	if !strings.Contains(got, "s3:https://KEY:***@host/bucket") {
		t.Errorf("sanitized stderr lost the redacted url: %q", got)
	}

	if strings.HasPrefix(got, " ") || strings.HasSuffix(got, "\n") {
		t.Errorf("sanitized stderr not trimmed: %q", got)
	}
}

func TestSanitizeStderr_BoundsLongOutput(t *testing.T) {
	t.Parallel()

	in := []byte(strings.Repeat("x", 10*1024) + "TAIL")

	got := restic.SanitizeStderr(in)

	const maxWithMarker = 4*1024 + 64
	if len(got) > maxWithMarker {
		t.Errorf("sanitized stderr length = %d, want <= %d", len(got), maxWithMarker)
	}

	if !strings.HasSuffix(got, "TAIL") {
		t.Errorf("sanitized stderr should keep the tail, got suffix %q", got[len(got)-8:])
	}

	if !strings.Contains(got, "truncated") {
		t.Errorf("truncated output should be marked, got %q", got[:20])
	}
}
