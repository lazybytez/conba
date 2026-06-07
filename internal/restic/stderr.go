package restic

import (
	"bytes"

	"github.com/lazybytez/conba/internal/support/redact"
)

// maxStderrTail bounds how much of restic's stderr is surfaced in an error
// or log. restic stderr is normally short, but a misbehaving backend can
// produce a large volume; only the tail is useful for diagnosis.
const maxStderrTail = 4 * 1024

// truncationMarker prefixes a stderr tail that was cut to maxStderrTail.
const truncationMarker = "...(truncated) "

// sanitizeStderr prepares restic's stderr for surfacing: it trims
// surrounding whitespace, masks any credentials embedded in repository URLs
// restic echoed back, and bounds the result to the trailing maxStderrTail
// bytes so a runaway backend cannot flood output or logs.
func sanitizeStderr(stderr []byte) string {
	text := redact.Credentials(string(bytes.TrimSpace(stderr)))

	if len(text) > maxStderrTail {
		text = truncationMarker + text[len(text)-maxStderrTail:]
	}

	return text
}
