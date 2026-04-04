package main

import (
	"testing"
)

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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := version
			version = tt.version
			t.Cleanup(func() { version = original })

			got := versionString()
			if got != tt.want {
				t.Errorf("versionString() = %q, want %q", got, tt.want)
			}
		})
	}
}
