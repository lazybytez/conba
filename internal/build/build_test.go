package build_test

import (
	"runtime"
	"testing"

	"github.com/lazybytez/conba/internal/build"
)

func TestFormatVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		version   string
		commitSHA string
		want      string
	}{
		{
			name:      "edge version includes commit SHA",
			version:   "edge",
			commitSHA: "abc1234",
			want:      "edge<abc1234>",
		},
		{
			name:      "release version returned as-is",
			version:   "v1.2.3",
			commitSHA: "abc1234",
			want:      "v1.2.3",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := build.FormatVersion(test.version, test.commitSHA)
			if got != test.want {
				t.Errorf("FormatVersion() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestComputeVersionString(t *testing.T) {
	t.Parallel()

	got := build.ComputeVersionString()
	if got == "" {
		t.Fatal("ComputeVersionString() returned empty string")
	}
}

func TestResticVersion(t *testing.T) {
	t.Parallel()

	if build.ResticVersion == "" {
		t.Fatal("ResticVersion is empty")
	}
}

func TestGoVersion(t *testing.T) {
	t.Parallel()

	got := build.GoVersion()
	if got == "" {
		t.Fatal("GoVersion() returned empty string")
	}

	if got != runtime.Version() {
		t.Errorf("GoVersion() = %q, want %q", got, runtime.Version())
	}
}
