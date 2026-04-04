package main

import (
	"testing"
)

//nolint:paralleltest // subtests mutate package-level version var
func TestVersionString(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
	}{
		{
			name:    "default dev version",
			version: "dev",
			want:    "conba vdev",
		},
		{
			name:    "release version",
			version: "1.2.3",
			want:    "conba v1.2.3",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			original := version
			version = test.version

			t.Cleanup(func() { version = original })

			got := versionString()
			if got != test.want {
				t.Errorf("versionString() = %q, want %q", got, test.want)
			}
		})
	}
}
