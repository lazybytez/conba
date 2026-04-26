package forget_test

import (
	"errors"
	"testing"

	"github.com/lazybytez/conba/internal/config"
	"github.com/lazybytez/conba/internal/discovery"
	"github.com/lazybytez/conba/internal/filter"
	"github.com/lazybytez/conba/internal/forget"
	"github.com/lazybytez/conba/internal/runtime"
)

func makeTargetWithLabels(labels map[string]string) discovery.Target {
	return discovery.Target{
		Container: runtime.ContainerInfo{
			ID:     "c-test",
			Name:   "test",
			Labels: labels,
			Mounts: nil,
		},
		Mount: runtime.MountInfo{
			Type:        runtime.MountTypeVolume,
			Name:        "data",
			Source:      "/src/data",
			Destination: "/data",
			ReadOnly:    false,
		},
	}
}

type parseCase struct {
	name    string
	input   string
	want    config.RetentionConfig
	wantErr bool
}

// policy is a brief constructor that fills all RetentionConfig fields
// explicitly. Tests use it instead of partial struct literals so the
// project's exhaustruct rule is satisfied without 100-char-long lines.
func policy(d, w, m, y int) config.RetentionConfig {
	return config.RetentionConfig{
		KeepDaily:   d,
		KeepWeekly:  w,
		KeepMonthly: m,
		KeepYearly:  y,
	}
}

func parseSuccessCases() []parseCase {
	return []parseCase{
		{
			name:    "all four dimensions",
			input:   "7d,4w,6m,2y",
			want:    policy(7, 4, 6, 2),
			wantErr: false,
		},
		{
			name:    "single daily",
			input:   "30d",
			want:    policy(30, 0, 0, 0),
			wantErr: false,
		},
		{
			name:    "uppercase suffixes",
			input:   "7D,4W",
			want:    policy(7, 4, 0, 0),
			wantErr: false,
		},
		{
			name:    "whitespace tolerant",
			input:   " 7d , 4w ",
			want:    policy(7, 4, 0, 0),
			wantErr: false,
		},
		{
			name:    "empty string",
			input:   "",
			want:    policy(0, 0, 0, 0),
			wantErr: false,
		},
		{
			name:    "comma-only input is empty",
			input:   ",",
			want:    policy(0, 0, 0, 0),
			wantErr: false,
		},
	}
}

func parseErrorCases() []parseCase {
	return []parseCase{
		{
			name:    "missing suffix",
			input:   "7",
			want:    policy(0, 0, 0, 0),
			wantErr: true,
		},
		{
			name:    "unknown suffix",
			input:   "7x",
			want:    policy(0, 0, 0, 0),
			wantErr: true,
		},
		{
			name:    "negative number",
			input:   "-1d",
			want:    policy(0, 0, 0, 0),
			wantErr: true,
		},
		{
			name:    "non-numeric prefix",
			input:   "abcd",
			want:    policy(0, 0, 0, 0),
			wantErr: true,
		},
		{
			name:    "duplicate suffix",
			input:   "7d,7d",
			want:    policy(0, 0, 0, 0),
			wantErr: true,
		},
	}
}

func runParseCase(t *testing.T, testCase parseCase) {
	t.Helper()

	got, err := forget.ParseRetentionLabel(testCase.input)

	if testCase.wantErr {
		if err == nil {
			t.Fatalf("want error, got nil (parsed %+v)", got)
		}

		if !errors.Is(err, forget.ErrInvalidRetentionLabel) {
			t.Errorf("want error wrapping ErrInvalidRetentionLabel, got %v", err)
		}

		return
	}

	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}

	if got != testCase.want {
		t.Errorf("got %+v, want %+v", got, testCase.want)
	}
}

func TestParseRetentionLabel(t *testing.T) {
	t.Parallel()

	cases := append(parseSuccessCases(), parseErrorCases()...)

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			runParseCase(t, testCase)
		})
	}
}

type resolveCase struct {
	name           string
	labels         map[string]string
	global         config.RetentionConfig
	wantPolicy     config.RetentionConfig
	wantResolution forget.Resolution
	wantErr        bool
}

func resolveCases() []resolveCase {
	return []resolveCase{
		{
			name:           "label set, global zero, returns label policy",
			labels:         map[string]string{filter.LabelRetention: "7d"},
			global:         policy(0, 0, 0, 0),
			wantPolicy:     policy(7, 0, 0, 0),
			wantResolution: forget.ResolutionLabel,
			wantErr:        false,
		},
		{
			name:           "no label, global non-zero, returns global",
			labels:         map[string]string{},
			global:         policy(0, 4, 0, 0),
			wantPolicy:     policy(0, 4, 0, 0),
			wantResolution: forget.ResolutionGlobal,
			wantErr:        false,
		},
		{
			name:           "no label, global zero, returns none",
			labels:         map[string]string{},
			global:         policy(0, 0, 0, 0),
			wantPolicy:     policy(0, 0, 0, 0),
			wantResolution: forget.ResolutionNone,
			wantErr:        false,
		},
		{
			name:           "label invalid, returns error and ResolutionNone",
			labels:         map[string]string{filter.LabelRetention: "garbage"},
			global:         policy(0, 0, 0, 0),
			wantPolicy:     policy(0, 0, 0, 0),
			wantResolution: forget.ResolutionNone,
			wantErr:        true,
		},
		{
			name:           "label parses to zero, falls back to global",
			labels:         map[string]string{filter.LabelRetention: "0d,0w,0m,0y"},
			global:         policy(1, 0, 0, 0),
			wantPolicy:     policy(1, 0, 0, 0),
			wantResolution: forget.ResolutionGlobal,
			wantErr:        false,
		},
		{
			name:           "label parses to zero, global zero, returns none",
			labels:         map[string]string{filter.LabelRetention: "0d,0w,0m,0y"},
			global:         policy(0, 0, 0, 0),
			wantPolicy:     policy(0, 0, 0, 0),
			wantResolution: forget.ResolutionNone,
			wantErr:        false,
		},
		{
			name:           "empty label string, global non-zero, falls back to global",
			labels:         map[string]string{filter.LabelRetention: ""},
			global:         policy(3, 0, 0, 0),
			wantPolicy:     policy(3, 0, 0, 0),
			wantResolution: forget.ResolutionGlobal,
			wantErr:        false,
		},
	}
}

func runResolveCase(t *testing.T, testCase resolveCase) {
	t.Helper()

	target := makeTargetWithLabels(testCase.labels)

	policy, resolution, err := forget.Resolve(target, testCase.global)

	if testCase.wantErr {
		if err == nil {
			t.Fatalf("want error, got nil (policy %+v, resolution %s)", policy, resolution)
		}

		if !errors.Is(err, forget.ErrInvalidRetentionLabel) {
			t.Errorf("want error wrapping ErrInvalidRetentionLabel, got %v", err)
		}

		if resolution != testCase.wantResolution {
			t.Errorf("want resolution %q, got %q", testCase.wantResolution, resolution)
		}

		return
	}

	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}

	if policy != testCase.wantPolicy {
		t.Errorf("policy: got %+v, want %+v", policy, testCase.wantPolicy)
	}

	if resolution != testCase.wantResolution {
		t.Errorf("resolution: got %q, want %q", resolution, testCase.wantResolution)
	}
}

func TestResolve(t *testing.T) {
	t.Parallel()

	for _, testCase := range resolveCases() {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			runResolveCase(t, testCase)
		})
	}
}
