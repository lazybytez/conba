package forget

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/lazybytez/conba/internal/backup"
	"github.com/lazybytez/conba/internal/config"
	"github.com/lazybytez/conba/internal/discovery"
	"github.com/lazybytez/conba/internal/filter"
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

// hostTagPrefix is the prefix on the per-host tag emitted by
// backup.BuildTags. The forget loop strips tags with this prefix when
// opts.AllHosts is true so retention applies across every host that
// shares the repository.
const hostTagPrefix = "hostname="

// targetOutcome classifies the result of forgetting on a single target.
type targetOutcome int

const (
	outcomeSucceeded targetOutcome = iota
	outcomeSkipped
	outcomeFailed
)

// Run iterates targets, resolves effective retention per target, and
// calls forgetFn once per target with the target's tags (plus host
// scoping unless opts.AllHosts). Outcomes split into succeeded /
// skipped / failed buckets.
func Run(
	ctx context.Context,
	targets []discovery.Target,
	forgetFn Func,
	globalRetention config.RetentionConfig,
	opts Options,
	out io.Writer,
) error {
	if len(targets) == 0 {
		return nil
	}

	succeeded := 0
	skipped := 0
	failed := 0

	for _, target := range targets {
		switch runTarget(ctx, target, forgetFn, globalRetention, opts, out) {
		case outcomeSucceeded:
			succeeded++
		case outcomeSkipped:
			skipped++
		case outcomeFailed:
			failed++
		}
	}

	writeSummary(out, opts.DryRun, succeeded, skipped, failed)

	if failed > 0 {
		return fmt.Errorf("%d target(s) failed: %w", failed, ErrTargetsFailed)
	}

	return nil
}

func runTarget(
	ctx context.Context,
	target discovery.Target,
	forgetFn Func,
	globalRetention config.RetentionConfig,
	opts Options,
	out io.Writer,
) targetOutcome {
	policy, source, err := Resolve(target, globalRetention)
	if err != nil {
		raw := target.Container.Labels[filter.LabelRetention]
		_, _ = fmt.Fprintf(
			out,
			"Failed %s/%s: invalid retention label %q: %v\n",
			target.Container.Name,
			target.Mount.Name,
			raw,
			err,
		)

		return outcomeFailed
	}

	if source == ResolutionNone {
		_, _ = fmt.Fprintf(
			out,
			"Skipped %s/%s: no retention policy configured (label empty, global empty)\n",
			target.Container.Name,
			target.Mount.Name,
		)

		return outcomeSkipped
	}

	tags := buildTags(target, opts)
	resticPolicy := toResticPolicy(policy)
	resticOpts := restic.ForgetOptions{Prune: opts.Prune, DryRun: opts.DryRun}

	err = forgetFn(ctx, tags, resticPolicy, resticOpts)
	if err != nil {
		_, _ = fmt.Fprintf(
			out,
			"Failed %s/%s: %v\n",
			target.Container.Name,
			target.Mount.Name,
			err,
		)

		return outcomeFailed
	}

	verb := "Forgot from"
	if opts.DryRun {
		verb = "Would forget from"
	}

	_, _ = fmt.Fprintf(out, "%s %s/%s\n", verb, target.Container.Name, target.Mount.Name)

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
		if strings.HasPrefix(tag, hostTagPrefix) {
			continue
		}

		filtered = append(filtered, tag)
	}

	return filtered
}

func toResticPolicy(c config.RetentionConfig) restic.ForgetPolicy {
	return restic.ForgetPolicy{
		KeepDaily:   c.KeepDaily,
		KeepWeekly:  c.KeepWeekly,
		KeepMonthly: c.KeepMonthly,
		KeepYearly:  c.KeepYearly,
	}
}

func writeSummary(out io.Writer, dryRun bool, succeeded, skipped, failed int) {
	if dryRun {
		_, _ = fmt.Fprintf(
			out,
			"Forget complete (dry-run): %d would succeed, %d skipped, %d failed.\n",
			succeeded,
			skipped,
			failed,
		)

		return
	}

	_, _ = fmt.Fprintf(
		out,
		"Forget complete: %d succeeded, %d skipped, %d failed.\n",
		succeeded,
		skipped,
		failed,
	)
}
