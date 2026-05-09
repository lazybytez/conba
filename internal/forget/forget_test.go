package forget_test

import (
	"bytes"
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/lazybytez/conba/internal/discovery"
	"github.com/lazybytez/conba/internal/filter"
	"github.com/lazybytez/conba/internal/forget"
	"github.com/lazybytez/conba/internal/restic"
	"github.com/lazybytez/conba/internal/runtime"
)

var errForget = errors.New("forget failed")

type capturedCall struct {
	tags   []string
	policy restic.ForgetPolicy
	opts   restic.ForgetOptions
}

func stubForgetFn(errs ...error) (forget.Func, *[]capturedCall) {
	var calls []capturedCall

	callIndex := 0

	forgetFn := func(
		_ context.Context,
		tags []string,
		policy restic.ForgetPolicy,
		opts restic.ForgetOptions,
	) error {
		calls = append(calls, capturedCall{
			tags:   append([]string(nil), tags...),
			policy: policy,
			opts:   opts,
		})

		var err error
		if callIndex < len(errs) {
			err = errs[callIndex]
		}

		callIndex++

		return err
	}

	return forgetFn, &calls
}

func makeForgetTarget(name, mountName string, labels map[string]string) discovery.Target {
	return discovery.Target{
		Container: runtime.ContainerInfo{
			ID:     "c-" + name,
			Name:   name,
			Labels: labels,
			Mounts: nil,
		},
		Mount: runtime.MountInfo{
			Type:        runtime.MountTypeVolume,
			Name:        mountName,
			Source:      "/src/" + name,
			Destination: "/" + mountName,
			ReadOnly:    false,
		},
	}
}

func defaultOpts() forget.Options {
	return forget.Options{
		Hostname: "host1",
		AllHosts: false,
		DryRun:   false,
		Prune:    false,
	}
}

func TestRun_AllSucceed(t *testing.T) {
	t.Parallel()

	targets := []discovery.Target{
		makeForgetTarget("app", "data", map[string]string{filter.LabelRetention: "7d"}),
		makeForgetTarget("db", "pgdata", map[string]string{filter.LabelRetention: "4w"}),
	}

	forgetFn, _ := stubForgetFn(nil, nil)

	var buf bytes.Buffer

	err := forget.Run(
		context.Background(),
		targets,
		forgetFn,
		policy(0, 0, 0, 0),
		defaultOpts(),
		&buf,
	)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "2 succeeded, 0 skipped, 0 failed") {
		t.Errorf("want summary '2 succeeded, 0 skipped, 0 failed', got:\n%s", output)
	}

	if strings.Count(output, "Forgot from") != 2 {
		t.Errorf("want 2 'Forgot from' lines, got:\n%s", output)
	}

	summaryRe := regexp.MustCompile(
		`(?m)^Forget complete: \d+ succeeded, \d+ skipped, \d+ failed\.$`,
	)
	if !summaryRe.MatchString(output) {
		t.Errorf("summary line does not match expected format, got:\n%s", output)
	}
}

func TestRun_AllFail(t *testing.T) {
	t.Parallel()

	targets := []discovery.Target{
		makeForgetTarget("app", "data", map[string]string{filter.LabelRetention: "7d"}),
		makeForgetTarget("db", "pgdata", map[string]string{filter.LabelRetention: "4w"}),
	}

	forgetFn, _ := stubForgetFn(errForget, errForget)

	var buf bytes.Buffer

	err := forget.Run(
		context.Background(),
		targets,
		forgetFn,
		policy(0, 0, 0, 0),
		defaultOpts(),
		&buf,
	)
	if err == nil {
		t.Fatal("want error, got nil")
	}

	if !errors.Is(err, forget.ErrTargetsFailed) {
		t.Errorf("want error wrapping ErrTargetsFailed, got %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "0 succeeded, 0 skipped, 2 failed") {
		t.Errorf("want summary '0 succeeded, 0 skipped, 2 failed', got:\n%s", output)
	}

	if strings.Count(output, "Failed") != 2 {
		t.Errorf("want 2 'Failed' lines, got:\n%s", output)
	}
}

func TestRun_MixedOutcomes(t *testing.T) {
	t.Parallel()

	targets := []discovery.Target{
		makeForgetTarget("ok", "okvol", map[string]string{filter.LabelRetention: "7d"}),
		makeForgetTarget("bad", "badvol", map[string]string{filter.LabelRetention: "garbage"}),
		makeForgetTarget("none", "nonevol", map[string]string{}),
	}

	forgetFn, calls := stubForgetFn(nil)

	var buf bytes.Buffer

	err := forget.Run(
		context.Background(),
		targets,
		forgetFn,
		policy(0, 0, 0, 0),
		defaultOpts(),
		&buf,
	)
	if err == nil {
		t.Fatal("want error because failed > 0, got nil")
	}

	if !errors.Is(err, forget.ErrTargetsFailed) {
		t.Errorf("want error wrapping ErrTargetsFailed, got %v", err)
	}

	if len(*calls) != 1 {
		t.Errorf("want 1 forgetFn call (only the success target), got %d", len(*calls))
	}

	output := buf.String()

	if !strings.Contains(output, "1 succeeded, 1 skipped, 1 failed") {
		t.Errorf("want summary '1 succeeded, 1 skipped, 1 failed', got:\n%s", output)
	}

	if !strings.Contains(output, "Failed bad/badvol") {
		t.Errorf("want 'Failed bad/badvol' line, got:\n%s", output)
	}

	if !strings.Contains(output, "Skipped none/nonevol") {
		t.Errorf("want 'Skipped none/nonevol' line, got:\n%s", output)
	}

	if !strings.Contains(output, "no retention policy configured") {
		t.Errorf("want 'no retention policy configured' message, got:\n%s", output)
	}
}

func TestRun_DryRunSummary(t *testing.T) {
	t.Parallel()

	targets := []discovery.Target{
		makeForgetTarget("app", "data", map[string]string{filter.LabelRetention: "7d"}),
		makeForgetTarget("db", "pgdata", map[string]string{filter.LabelRetention: "4w"}),
	}

	forgetFn, _ := stubForgetFn(nil, nil)

	opts := defaultOpts()
	opts.DryRun = true

	var buf bytes.Buffer

	err := forget.Run(context.Background(), targets, forgetFn, policy(0, 0, 0, 0), opts, &buf)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "(dry-run)") {
		t.Errorf("want '(dry-run)' in output, got:\n%s", output)
	}

	if !strings.Contains(output, "would succeed") {
		t.Errorf("want 'would succeed' in output, got:\n%s", output)
	}

	if strings.Count(output, "Would forget from") != 2 {
		t.Errorf("want 2 'Would forget from' lines, got:\n%s", output)
	}
}

func TestRun_AllHostsStripsHostTag(t *testing.T) {
	t.Parallel()

	targets := []discovery.Target{
		makeForgetTarget("app", "data", map[string]string{filter.LabelRetention: "7d"}),
	}

	forgetFn, calls := stubForgetFn(nil)

	opts := defaultOpts()
	opts.Hostname = "myhost"
	opts.AllHosts = true

	var buf bytes.Buffer

	err := forget.Run(context.Background(), targets, forgetFn, policy(0, 0, 0, 0), opts, &buf)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}

	if len(*calls) != 1 {
		t.Fatalf("want 1 call, got %d", len(*calls))
	}

	for _, tag := range (*calls)[0].tags {
		if strings.HasPrefix(tag, "hostname=") {
			t.Errorf("want no tag with prefix 'hostname=', got %q", tag)
		}
	}
}

func TestRun_DefaultIncludesHostTag(t *testing.T) {
	t.Parallel()

	targets := []discovery.Target{
		makeForgetTarget("app", "data", map[string]string{filter.LabelRetention: "7d"}),
	}

	forgetFn, calls := stubForgetFn(nil)

	opts := defaultOpts()
	opts.Hostname = "myhost"
	opts.AllHosts = false

	var buf bytes.Buffer

	err := forget.Run(context.Background(), targets, forgetFn, policy(0, 0, 0, 0), opts, &buf)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}

	if len(*calls) != 1 {
		t.Fatalf("want 1 call, got %d", len(*calls))
	}

	hostTagCount := 0

	for _, tag := range (*calls)[0].tags {
		if strings.HasPrefix(tag, "hostname=") {
			hostTagCount++

			if tag != "hostname=myhost" {
				t.Errorf("want tag 'hostname=myhost', got %q", tag)
			}
		}
	}

	if hostTagCount != 1 {
		t.Errorf("want exactly 1 hostname= tag, got %d", hostTagCount)
	}
}

func TestRun_PassesPruneAndDryRun(t *testing.T) {
	t.Parallel()

	targets := []discovery.Target{
		makeForgetTarget("app", "data", map[string]string{filter.LabelRetention: "7d"}),
	}

	forgetFn, calls := stubForgetFn(nil)

	opts := defaultOpts()
	opts.Prune = true
	opts.DryRun = true

	var buf bytes.Buffer

	err := forget.Run(context.Background(), targets, forgetFn, policy(0, 0, 0, 0), opts, &buf)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}

	if len(*calls) != 1 {
		t.Fatalf("want 1 call, got %d", len(*calls))
	}

	gotOpts := (*calls)[0].opts
	if !gotOpts.Prune {
		t.Errorf("want Prune=true, got Prune=%v", gotOpts.Prune)
	}

	if !gotOpts.DryRun {
		t.Errorf("want DryRun=true, got DryRun=%v", gotOpts.DryRun)
	}
}

func TestRun_NoTargets_NoOutput_NoError(t *testing.T) {
	t.Parallel()

	forgetFn, _ := stubForgetFn()

	var buf bytes.Buffer

	err := forget.Run(
		context.Background(),
		nil,
		forgetFn,
		policy(0, 0, 0, 0),
		defaultOpts(),
		&buf,
	)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}

	if buf.Len() != 0 {
		t.Errorf("want no output, got %q", buf.String())
	}
}

func TestRun_LabelOverrideUsesParsedPolicy(t *testing.T) {
	t.Parallel()

	targets := []discovery.Target{
		makeForgetTarget("app", "data", map[string]string{filter.LabelRetention: "7d"}),
	}

	forgetFn, calls := stubForgetFn(nil)

	var buf bytes.Buffer

	err := forget.Run(
		context.Background(),
		targets,
		forgetFn,
		policy(0, 1, 0, 0),
		defaultOpts(),
		&buf,
	)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}

	if len(*calls) != 1 {
		t.Fatalf("want 1 call, got %d", len(*calls))
	}

	gotPolicy := (*calls)[0].policy
	if gotPolicy.KeepDaily != 7 {
		t.Errorf("want KeepDaily=7, got %d", gotPolicy.KeepDaily)
	}

	if gotPolicy.KeepWeekly != 0 {
		t.Errorf("want KeepWeekly=0 (label takes precedence), got %d", gotPolicy.KeepWeekly)
	}

	if gotPolicy.KeepMonthly != 0 {
		t.Errorf("want KeepMonthly=0, got %d", gotPolicy.KeepMonthly)
	}

	if gotPolicy.KeepYearly != 0 {
		t.Errorf("want KeepYearly=0, got %d", gotPolicy.KeepYearly)
	}
}

func TestRun_GlobalUsedWhenLabelAbsent(t *testing.T) {
	t.Parallel()

	targets := []discovery.Target{
		makeForgetTarget("app", "data", map[string]string{}),
	}

	forgetFn, calls := stubForgetFn(nil)

	var buf bytes.Buffer

	err := forget.Run(
		context.Background(),
		targets,
		forgetFn,
		policy(0, 1, 0, 0),
		defaultOpts(),
		&buf,
	)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}

	if len(*calls) != 1 {
		t.Fatalf("want 1 call, got %d", len(*calls))
	}

	gotPolicy := (*calls)[0].policy
	if gotPolicy.KeepWeekly != 1 {
		t.Errorf("want KeepWeekly=1, got %d", gotPolicy.KeepWeekly)
	}

	if gotPolicy.KeepDaily != 0 {
		t.Errorf("want KeepDaily=0, got %d", gotPolicy.KeepDaily)
	}
}
