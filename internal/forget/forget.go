package forget

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/lazybytez/conba/internal/backup"
	"github.com/lazybytez/conba/internal/config"
	"github.com/lazybytez/conba/internal/discovery"
	"github.com/lazybytez/conba/internal/filter"
	"github.com/lazybytez/conba/internal/report"
	"github.com/lazybytez/conba/internal/restic"
)

// Func is the signature for a forget operation on a tag set with a
// policy and options. Mirrors restic.Client.Forget so tests can inject
// a stub.
type Func func(
	ctx context.Context,
	tags []string,
	policy restic.ForgetPolicy,
	opts restic.ForgetOptions,
) error

// Options bundles the per-run flags that don't belong in the per-target
// loop.
type Options struct {
	Hostname string
	AllHosts bool
	DryRun   bool
	Prune    bool
}

// ErrTargetsFailed is returned by Run when at least one target failed
// (parse error or restic error). Mirrors backup.ErrTargetsFailed.
var ErrTargetsFailed = errors.New("forget targets failed")

// RunError reports how a forget cycle ended, carrying per-outcome counts so
// callers can distinguish a partial failure from a total one. It satisfies
// errors.Is(err, ErrTargetsFailed).
type RunError struct {
	Succeeded int
	Skipped   int
	Failed    int
}

// Error implements error.
func (e *RunError) Error() string {
	total := e.Succeeded + e.Skipped + e.Failed

	return fmt.Sprintf("%d of %d target(s) failed", e.Failed, total)
}

// Is reports that a RunError matches the ErrTargetsFailed sentinel.
func (e *RunError) Is(target error) bool {
	return target == ErrTargetsFailed
}

// targetOutcome classifies the result of forgetting on a single target.
type targetOutcome int

const (
	outcomeSucceeded targetOutcome = iota
	outcomeSkipped
	outcomeFailed
)

// Run iterates targets, resolves effective retention per target, and
// calls forgetFn once per target with the target's tags (plus host
// scoping unless opts.AllHosts), emitting progress and a summary through
// reporter. Outcomes split into succeeded / skipped / failed buckets.
func Run(
	ctx context.Context,
	targets []discovery.Target,
	forgetFn Func,
	globalRetention config.RetentionConfig,
	opts Options,
	reporter report.Reporter,
) error {
	if len(targets) == 0 {
		return nil
	}

	totals := struct{ succeeded, skipped, failed int }{succeeded: 0, skipped: 0, failed: 0}

	for _, target := range targets {
		switch runTarget(ctx, target, forgetFn, globalRetention, opts, reporter) {
		case outcomeSucceeded:
			totals.succeeded++
		case outcomeSkipped:
			totals.skipped++
		case outcomeFailed:
			totals.failed++
		}
	}

	emitSummary(reporter, opts.DryRun, totals.succeeded, totals.skipped, totals.failed)

	if totals.failed > 0 {
		return &RunError{
			Succeeded: totals.succeeded,
			Skipped:   totals.skipped,
			Failed:    totals.failed,
		}
	}

	return nil
}

func runTarget(
	ctx context.Context,
	target discovery.Target,
	forgetFn Func,
	globalRetention config.RetentionConfig,
	opts Options,
	reporter report.Reporter,
) targetOutcome {
	policy, source, err := Resolve(target, globalRetention)
	if err != nil {
		raw := target.Container.Labels[filter.LabelRetention]
		reporter.Emit(report.Event{
			Level: report.LevelError,
			Style: report.StyleError,
			Name:  "forget.target",
			Message: fmt.Sprintf(
				"Failed %s/%s: invalid retention label %q: %v",
				target.Container.Name, target.Mount.Name, raw, err,
			),
			Fields: []report.Field{
				report.F("container", target.Container.Name),
				report.F("volume", target.Mount.Name),
				report.F("outcome", "failed"),
				report.F("error", err.Error()),
			},
		})

		return outcomeFailed
	}

	if source == ResolutionNone {
		reporter.Emit(report.Event{
			Level: report.LevelInfo,
			Name:  "forget.target",
			Message: fmt.Sprintf(
				"Skipped %s/%s: no retention policy configured (label empty, global empty)",
				target.Container.Name, target.Mount.Name,
			),
			Fields: []report.Field{
				report.F("container", target.Container.Name),
				report.F("volume", target.Mount.Name),
				report.F("outcome", "skipped"),
				report.F("reason", "no retention policy configured"),
			},
		})

		return outcomeSkipped
	}

	return applyForget(ctx, target, forgetFn, policy, opts, reporter)
}

// applyForget runs forgetFn for a target with a resolved policy and reports
// the outcome.
func applyForget(
	ctx context.Context,
	target discovery.Target,
	forgetFn Func,
	policy config.RetentionConfig,
	opts Options,
	reporter report.Reporter,
) targetOutcome {
	tags := buildTags(target, opts)
	resticOpts := restic.ForgetOptions{Prune: opts.Prune, DryRun: opts.DryRun}

	err := forgetFn(ctx, tags, ToResticPolicy(policy), resticOpts)
	if err != nil {
		reporter.Emit(report.Event{
			Level:   report.LevelError,
			Style:   report.StyleError,
			Name:    "forget.target",
			Message: fmt.Sprintf("Failed %s/%s: %v", target.Container.Name, target.Mount.Name, err),
			Fields: []report.Field{
				report.F("container", target.Container.Name),
				report.F("volume", target.Mount.Name),
				report.F("outcome", "failed"),
				report.F("error", err.Error()),
			},
		})

		return outcomeFailed
	}

	verb := "Forgot from"
	if opts.DryRun {
		verb = "Would forget from"
	}

	reporter.Emit(report.Event{
		Level:   report.LevelInfo,
		Style:   report.StyleSuccess,
		Name:    "forget.target",
		Message: fmt.Sprintf("%s %s/%s", verb, target.Container.Name, target.Mount.Name),
		Fields: []report.Field{
			report.F("container", target.Container.Name),
			report.F("volume", target.Mount.Name),
			report.F("outcome", "success"),
			report.F("dry_run", opts.DryRun),
		},
	})

	return outcomeSucceeded
}

// buildTags returns the tag set passed to forgetFn for a single target.
// When opts.AllHosts is true the host-scope tag is removed so retention
// applies across every host that writes to the repository.
func buildTags(target discovery.Target, opts Options) []string {
	tags := backup.BuildTags(target, opts.Hostname)

	if !opts.AllHosts {
		return tags
	}

	filtered := make([]string, 0, len(tags))

	for _, tag := range tags {
		if strings.HasPrefix(tag, backup.HostTagPrefix) {
			continue
		}

		filtered = append(filtered, tag)
	}

	return filtered
}

// ToResticPolicy projects a RetentionConfig onto restic's ForgetPolicy
// shape. Both the discovery loop and the surgical CLI path use this
// helper so the field mapping has a single source of truth.
func ToResticPolicy(c config.RetentionConfig) restic.ForgetPolicy {
	return restic.ForgetPolicy{
		KeepDaily:   c.KeepDaily,
		KeepWeekly:  c.KeepWeekly,
		KeepMonthly: c.KeepMonthly,
		KeepYearly:  c.KeepYearly,
	}
}

func emitSummary(reporter report.Reporter, dryRun bool, succeeded, skipped, failed int) {
	message := fmt.Sprintf(
		"Forget complete: %d succeeded, %d skipped, %d failed.", succeeded, skipped, failed,
	)
	if dryRun {
		message = fmt.Sprintf(
			"Forget complete (dry-run): %d would succeed, %d skipped, %d failed.",
			succeeded, skipped, failed,
		)
	}

	reporter.Emit(report.Event{
		Level:   report.LevelInfo,
		Name:    "forget.summary",
		Message: message,
		Fields: []report.Field{
			report.F("succeeded", succeeded),
			report.F("skipped", skipped),
			report.F("failed", failed),
			report.F("dry_run", dryRun),
		},
	})
}
