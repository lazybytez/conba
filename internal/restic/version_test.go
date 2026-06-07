package restic_test

import (
	"context"
	"testing"

	"github.com/lazybytez/conba/internal/restic"
)

func TestParseResticVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"standard line", "restic 0.18.1 compiled with go1.25.1 on linux/arm64\n", "0.18.1"},
		{"extra spaces", "  restic   0.17.3  compiled\n", "0.17.3"},
		{"unrecognized", "some other tool 1.2.3", ""},
		{"empty", "", ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := restic.ParseResticVersion(test.in)
			if got != test.want {
				t.Errorf("ParseResticVersion(%q) = %q, want %q", test.in, got, test.want)
			}
		})
	}
}

func TestVersionsCompatible(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		a, b      string
		wantMatch bool
		wantOK    bool
	}{
		{"same version", "0.18.1", "0.18.1", true, true},
		{"patch differs is compatible", "0.18.1", "0.18.5", true, true},
		{"minor differs", "0.18.1", "0.17.3", false, true},
		{"major differs", "1.0.0", "0.18.1", false, true},
		{"unparseable left", "unknown", "0.18.1", false, false},
		{"unparseable right", "0.18.1", "v0.18", false, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			match, ok := restic.VersionsCompatible(test.a, test.b)
			if match != test.wantMatch || ok != test.wantOK {
				t.Errorf("VersionsCompatible(%q, %q) = (%v, %v), want (%v, %v)",
					test.a, test.b, match, ok, test.wantMatch, test.wantOK)
			}
		})
	}
}

// TestDetectVersion exercises the real restic binary; it skips when restic is
// not installed, matching the rest of the package's integration tests.
func TestDetectVersion(t *testing.T) {
	t.Parallel()

	version, err := restic.DetectVersion(context.Background(), "restic")
	if err != nil {
		t.Skipf("restic not available: %v", err)
	}

	if _, ok := restic.VersionsCompatible(version, version); !ok {
		t.Errorf("detected version %q does not parse", version)
	}
}
