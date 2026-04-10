package format_test

import (
	"testing"

	"github.com/lazybytez/conba/internal/format"
)

func TestBytes_Zero(t *testing.T) {
	t.Parallel()

	got := format.Bytes(0)
	if got != "0 B" {
		t.Errorf("Bytes(0) = %q, want %q", got, "0 B")
	}
}

func TestBytes_Bytes(t *testing.T) {
	t.Parallel()

	got := format.Bytes(500)
	if got != "500 B" {
		t.Errorf("Bytes(500) = %q, want %q", got, "500 B")
	}
}

func TestBytes_KiB(t *testing.T) {
	t.Parallel()

	got := format.Bytes(2048)
	if got != "2.00 KiB" {
		t.Errorf("Bytes(2048) = %q, want %q", got, "2.00 KiB")
	}
}

func TestBytes_MiB(t *testing.T) {
	t.Parallel()

	got := format.Bytes(5 * 1024 * 1024)
	if got != "5.00 MiB" {
		t.Errorf("Bytes(5*MiB) = %q, want %q", got, "5.00 MiB")
	}
}

func TestBytes_GiB(t *testing.T) {
	t.Parallel()

	got := format.Bytes(3 * 1024 * 1024 * 1024)
	if got != "3.00 GiB" {
		t.Errorf("Bytes(3*GiB) = %q, want %q", got, "3.00 GiB")
	}
}

func TestBytes_TiB(t *testing.T) {
	t.Parallel()

	got := format.Bytes(2 * 1024 * 1024 * 1024 * 1024)
	if got != "2.00 TiB" {
		t.Errorf("Bytes(2*TiB) = %q, want %q", got, "2.00 TiB")
	}
}
