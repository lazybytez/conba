package docker_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/lazybytez/conba/internal/runtime/docker"
)

// TestTailWriter_KeepsLastBytesUnderCap proves a write smaller than the cap is
// retained in full.
func TestTailWriter_KeepsLastBytesUnderCap(t *testing.T) {
	t.Parallel()

	writer := docker.NewTailWriter(16)

	gotN, err := writer.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("Write returned error: %v", err)
	}

	if gotN != 5 {
		t.Errorf("want n 5, got %d", gotN)
	}

	if got := string(writer.Bytes()); got != "hello" {
		t.Errorf("want %q, got %q", "hello", got)
	}
}

// TestTailWriter_TruncatesToCap proves input larger than the cap keeps only the
// last N bytes and never grows the buffer beyond the cap.
func TestTailWriter_TruncatesToCap(t *testing.T) {
	t.Parallel()

	const limit = 4

	writer := docker.NewTailWriter(limit)

	input := bytes.Repeat([]byte("a"), 1000)
	input = append(input, []byte("WXYZ")...)

	gotN, err := writer.Write(input)
	if err != nil {
		t.Fatalf("Write returned error: %v", err)
	}

	if gotN != len(input) {
		t.Errorf("want reported n %d (full input length), got %d", len(input), gotN)
	}

	got := writer.Bytes()
	if len(got) != limit {
		t.Fatalf("want buffer length %d (bounded), got %d", limit, len(got))
	}

	if string(got) != "WXYZ" {
		t.Errorf("want last %d bytes %q, got %q", limit, "WXYZ", string(got))
	}
}

// TestTailWriter_MultipleWritesStayBounded proves the buffer stays at the cap
// across many appends, retaining the most recent bytes.
func TestTailWriter_MultipleWritesStayBounded(t *testing.T) {
	t.Parallel()

	const limit = 3

	writer := docker.NewTailWriter(limit)

	for i := range 100 {
		_, err := writer.Write([]byte{byte('0' + i%10)})
		if err != nil {
			t.Fatalf("Write returned error: %v", err)
		}
	}

	got := writer.Bytes()
	if len(got) != limit {
		t.Fatalf("want bounded length %d, got %d", limit, len(got))
	}

	// Last three written bytes for i = 97, 98, 99 are '7', '8', '9'.
	if string(got) != "789" {
		t.Errorf("want %q, got %q", "789", string(got))
	}
}

// TestExecExitError_ZeroIsNil proves exit code 0 yields a nil error.
func TestExecExitError_ZeroIsNil(t *testing.T) {
	t.Parallel()

	err := docker.ExecExitError(0, []byte("anything on stderr"))
	if err != nil {
		t.Errorf("want nil error for exit 0, got %v", err)
	}
}

// TestExecExitError_NonZeroIncludesCodeAndTail proves a non-zero exit yields an
// error mentioning both the exit code and the captured stderr tail.
func TestExecExitError_NonZeroIncludesCodeAndTail(t *testing.T) {
	t.Parallel()

	err := docker.ExecExitError(137, []byte("permission denied"))
	if err == nil {
		t.Fatal("want non-nil error for non-zero exit")
	}

	msg := err.Error()
	if !strings.Contains(msg, "137") {
		t.Errorf("want error to contain exit code %q, got %q", "137", msg)
	}

	if !strings.Contains(msg, "permission denied") {
		t.Errorf("want error to contain stderr tail %q, got %q", "permission denied", msg)
	}
}

// TestExecExitError_NonZeroEmptyTail proves a non-zero exit with no captured
// stderr still reports the exit code.
func TestExecExitError_NonZeroEmptyTail(t *testing.T) {
	t.Parallel()

	err := docker.ExecExitError(1, nil)
	if err == nil {
		t.Fatal("want non-nil error for non-zero exit")
	}

	if !strings.Contains(err.Error(), "1") {
		t.Errorf("want error to contain exit code %q, got %q", "1", err.Error())
	}
}

// TestExecExitError_IsSentinel proves callers can match the failure with
// errors.Is against the package sentinel.
func TestExecExitError_IsSentinel(t *testing.T) {
	t.Parallel()

	err := docker.ExecExitError(2, []byte("boom"))
	if !errors.Is(err, docker.ErrExecNonZeroExit) {
		t.Errorf("want errors.Is(err, ErrExecNonZeroExit) to be true, got %v", err)
	}
}
