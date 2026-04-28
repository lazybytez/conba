package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"sort"
	"strings"

	"github.com/lazybytez/conba/internal/config"
	"github.com/lazybytez/conba/internal/discovery"
	"github.com/lazybytez/conba/internal/filter"
	"github.com/lazybytez/conba/internal/logging"
	"github.com/lazybytez/conba/internal/restic"
	"github.com/lazybytez/conba/internal/restore"
	"github.com/lazybytez/conba/internal/runtime"
	"github.com/lazybytez/conba/internal/runtime/docker"
	"github.com/spf13/cobra"
)

// errNoRestoreCommand is returned when stream-mode restore has no command
// available from either --to-command or the restore-command label.
var errNoRestoreCommand = errors.New("no restore command available")

// Pre-resolution flag combination errors.
var (
	errVolumeAndToCommandExclusive = errors.New(
		"--volume and --to-command are mutually exclusive",
	)
	errToAndToCommandExclusive = errors.New(
		"--to and --to-command are mutually exclusive",
	)
	errForceRequiresTo = errors.New("--force requires --to")
)

// Mode-mismatch errors raised once a snapshot has been resolved.
var (
	errToCommandOnVolumeSnapshot = errors.New(
		"--to-command applies to stream snapshots only; volume snapshots use --to",
	)
	errVolumeRestoreNeedsTo = errors.New("volume restore requires --to <path>")
	errAmbiguousVolume      = errors.New(
		"multiple volumes match filter; specify --volume",
	)
	errToOnStreamSnapshot = errors.New(
		"stream snapshot restore uses --to-command, not --to",
	)
	errVolumeOnStreamSnapshot = errors.New(
		"--volume applies to volume snapshots only; stream snapshots use --to-command",
	)
)

// Production-deps execution errors.
var (
	errEmptyExecArgv     = errors.New("empty argv for docker exec")
	errContainerNotFound = errors.New("container not found")
)

// kindStreamTag identifies a stream-kind snapshot via the kind=stream tag
// emitted at backup time.
const kindStreamTag = "kind=stream"

// RestoreCoreOptions bundles all parsed CLI flags plus the runtime
// configuration the core restore logic needs. It is decoupled from cobra
// so the orchestration can be exercised directly from tests.
type RestoreCoreOptions struct {
	// Container is the restic container=<name> tag to restrict to.
	Container string
	// Volume is the optional restic volume=<name> tag to restrict to.
	Volume string
	// Snapshot is the optional explicit restic snapshot ID.
	Snapshot string
	// To is the host directory to restore a volume snapshot into.
	To string
	// ToCommand is the in-container shell command to pipe a stream snapshot into.
	ToCommand string
	// Force overrides the non-empty destination guard for volume mode.
	Force bool
	// AllHosts drops the hostname=<host> filter from snapshot resolution.
	AllHosts bool
	// DryRun prints planned actions without performing them.
	DryRun bool
	// PreBackupEnabled mirrors config.PreBackupCommands.Enabled.
	PreBackupEnabled bool
	// Out receives human-readable progress lines.
	Out io.Writer
}

// RestoreCoreDeps abstracts the side-effects RunRestoreCore performs so
// tests can inject stubs.
type RestoreCoreDeps interface {
	// Snapshots lists snapshots matching the supplied tag filters.
	Snapshots(ctx context.Context, tags []string) ([]restic.Snapshot, error)
	// Restore extracts snapshotID into targetPath; dryRun=true reports
	// without writing files.
	Restore(ctx context.Context, snapshotID, targetPath string, dryRun bool) error
	// Dump streams filename inside snapshotID to stdout.
	Dump(ctx context.Context, snapshotID, filename string, stdout io.Writer) error
	// ContainerRunning reports whether a container is currently running.
	ContainerRunning(ctx context.Context, name string) (bool, error)
	// Exec runs argv inside the named container with stdin attached.
	Exec(ctx context.Context, name string, argv []string, stdin io.Reader) error
	// Hostname returns the host's name for use in the hostname=<host> tag.
	Hostname() (string, error)
	// LookupContainer returns metadata for the named container, including
	// its labels (used for the optional restore-command label).
	LookupContainer(ctx context.Context, name string) (runtime.ContainerInfo, error)
}

// NewRestoreCommand creates the restore subcommand that materialises a
// volume snapshot back to a host path or pipes a stream snapshot into a
// container command.
func NewRestoreCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restore",
		Short: "Restore a container volume or stream snapshot from restic",
		RunE:  runRestore,
	}

	cmd.Flags().String("container", "", "container name to restore (required)")
	cmd.Flags().String("volume", "", "volume name to restore (volume snapshots only)")
	cmd.Flags().String("snapshot", "", "explicit restic snapshot ID")
	cmd.Flags().String("to", "", "host directory to restore a volume snapshot into")
	cmd.Flags().String("to-command", "", "in-container command to pipe a stream snapshot into")
	cmd.Flags().Bool("force", false, "overwrite a non-empty restore destination")
	cmd.Flags().Bool("all-hosts", false, "consider snapshots from any hostname")
	cmd.Flags().Bool("dry-run", false, "print planned actions without performing them")

	err := cmd.MarkFlagRequired("container")
	if err != nil {
		// MarkFlagRequired only errors when the flag does not exist; we
		// just declared it above, so this branch is unreachable.
		panic(fmt.Sprintf("marking container flag required: %v", err))
	}

	return cmd
}

// runRestore is the cobra RunE that wires production dependencies and
// invokes RunRestoreCore.
func runRestore(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	cfg := config.FromContext(ctx)
	logger := logging.FromContext(ctx)

	if cfg == nil {
		return errMissingConfig
	}

	client, err := restic.New(cfg.Restic, logger)
	if err != nil {
		return fmt.Errorf("create restic client: %w", err)
	}

	dockerRT, err := docker.New(ctx, cfg.Runtime.Docker.Host)
	if err != nil {
		return fmt.Errorf("connect to docker: %w", err)
	}

	defer func() { _ = dockerRT.Close() }()

	deps := &productionRestoreDeps{client: client, runtime: dockerRT}

	opts := RestoreCoreOptions{
		Container:        flagString(cmd.Flags(), "container"),
		Volume:           flagString(cmd.Flags(), "volume"),
		Snapshot:         flagString(cmd.Flags(), "snapshot"),
		To:               flagString(cmd.Flags(), "to"),
		ToCommand:        flagString(cmd.Flags(), "to-command"),
		Force:            flagBool(cmd.Flags(), "force"),
		AllHosts:         flagBool(cmd.Flags(), "all-hosts"),
		DryRun:           flagBool(cmd.Flags(), "dry-run"),
		PreBackupEnabled: cfg.PreBackupCommands.Enabled,
		Out:              cmd.OutOrStdout(),
	}

	return RunRestoreCore(ctx, opts, deps)
}

// RunRestoreCore performs the validation, snapshot resolution, and mode
// dispatch for a restore. It is decoupled from cobra so tests can call
// it directly with stubbed dependencies.
func RunRestoreCore(ctx context.Context, opts RestoreCoreOptions, deps RestoreCoreDeps) error {
	err := validatePreResolution(opts)
	if err != nil {
		return err
	}

	tags, err := buildRestoreTags(opts, deps)
	if err != nil {
		return err
	}

	snapshots, err := deps.Snapshots(ctx, tags)
	if err != nil {
		return fmt.Errorf("list snapshots: %w", err)
	}

	snap, err := resolveRestoreSnapshot(snapshots, tags, opts)
	if err != nil {
		return err
	}

	if isStreamSnapshot(snap) {
		return runStreamRestore(ctx, opts, deps, snap)
	}

	return runVolumeRestore(ctx, opts, deps, snap, snapshots, tags)
}

// validatePreResolution rejects flag combinations that are invalid
// regardless of which kind of snapshot is selected.
func validatePreResolution(opts RestoreCoreOptions) error {
	if opts.Volume != "" && opts.ToCommand != "" {
		return fmt.Errorf("validate flags: %w", errVolumeAndToCommandExclusive)
	}

	if opts.To != "" && opts.ToCommand != "" {
		return fmt.Errorf("validate flags: %w", errToAndToCommandExclusive)
	}

	if opts.Force && opts.To == "" {
		return fmt.Errorf("validate flags: %w", errForceRequiresTo)
	}

	return nil
}

// buildRestoreTags assembles the tag filter slice handed to deps.Snapshots,
// honouring --all-hosts and --volume.
func buildRestoreTags(opts RestoreCoreOptions, deps RestoreCoreDeps) ([]string, error) {
	tags := []string{tagPrefixContainer + opts.Container}

	if opts.Volume != "" {
		tags = append(tags, tagPrefixVolume+opts.Volume)
	}

	if !opts.AllHosts {
		host, err := deps.Hostname()
		if err != nil {
			return nil, fmt.Errorf("get hostname: %w", err)
		}

		tags = append(tags, tagPrefixHostname+host)
	}

	return tags, nil
}

// resolveRestoreSnapshot picks the snapshot to restore. When --snapshot is
// set we resolve by ID and verify the tags; otherwise the latest snapshot
// matching the filter wins. A "no snapshot" outcome is rendered with the
// filter context so the user can diagnose why nothing matched.
func resolveRestoreSnapshot(
	snapshots []restic.Snapshot,
	tags []string,
	opts RestoreCoreOptions,
) (restic.Snapshot, error) {
	snap, err := restic.ResolveSnapshot(snapshots, tags, opts.Snapshot)
	if err != nil {
		if errors.Is(err, restic.ErrSnapshotNotFound) {
			return restic.Snapshot{}, fmt.Errorf(
				"no snapshot matched filters %s: %w",
				strings.Join(tags, ", "), err,
			)
		}

		return restic.Snapshot{}, fmt.Errorf("resolve snapshot: %w", err)
	}

	return snap, nil
}

// isStreamSnapshot reports whether a snapshot carries the kind=stream tag
// emitted at backup time for stream-kind backups.
func isStreamSnapshot(snap restic.Snapshot) bool {
	return slices.Contains(snap.Tags, kindStreamTag)
}

// runVolumeRestore handles a resolved volume snapshot: it enforces volume
// mode invariants, picks the right snapshot when multiple volumes match
// the filter, and delegates to restore.RunVolume.
func runVolumeRestore(
	ctx context.Context,
	opts RestoreCoreOptions,
	deps RestoreCoreDeps,
	snap restic.Snapshot,
	allSnapshots []restic.Snapshot,
	tags []string,
) error {
	if opts.ToCommand != "" {
		return fmt.Errorf("volume mode: %w", errToCommandOnVolumeSnapshot)
	}

	if opts.To == "" {
		return fmt.Errorf("volume mode: %w", errVolumeRestoreNeedsTo)
	}

	if opts.Volume == "" {
		volumes := distinctVolumes(allSnapshots, tags)
		if len(volumes) > 1 {
			return fmt.Errorf(
				"%w (candidates: %s)",
				errAmbiguousVolume, strings.Join(volumes, ", "),
			)
		}
	}

	runOpts := restore.Options{
		SnapshotID: snap.ID,
		Filename:   "",
		Container:  opts.Container,
		TargetPath: opts.To,
		Command:    "",
		DryRun:     opts.DryRun,
		Force:      opts.Force,
		Out:        opts.Out,
	}

	err := restore.RunVolume(ctx, runOpts, deps.Restore)
	if err != nil {
		return mapVolumeError(err, opts.To)
	}

	return nil
}

// distinctVolumes returns the sorted set of volume=<name> values across
// snapshots that satisfy every tag in tags. The result drives the "specify
// --volume" error when more than one candidate exists.
func distinctVolumes(snapshots []restic.Snapshot, tags []string) []string {
	seen := make(map[string]struct{})

	for _, snap := range snapshots {
		if !snapshotHasAllTags(snap, tags) {
			continue
		}

		for _, tag := range snap.Tags {
			if !strings.HasPrefix(tag, tagPrefixVolume) {
				continue
			}

			seen[tag[len(tagPrefixVolume):]] = struct{}{}
		}
	}

	out := make([]string, 0, len(seen))
	for v := range seen {
		out = append(out, v)
	}

	sort.Strings(out)

	return out
}

// snapshotHasAllTags reports whether snap carries every tag in required.
func snapshotHasAllTags(snap restic.Snapshot, required []string) bool {
	for _, want := range required {
		if !slices.Contains(snap.Tags, want) {
			return false
		}
	}

	return true
}

// mapVolumeError rewrites restore.ErrDestinationNotEmpty with a friendly
// hint about --force while preserving the original error chain.
func mapVolumeError(err error, target string) error {
	if errors.Is(err, restore.ErrDestinationNotEmpty) {
		return fmt.Errorf(
			"%w: %s is not empty (re-run with --force to overwrite)",
			err, target,
		)
	}

	return fmt.Errorf("volume restore: %w", err)
}

// runStreamRestore handles a resolved stream snapshot: it picks the
// command (flag wins over label), looks up the container labels when the
// pre-backup feature flag is enabled, and dispatches to restore.RunStream
// for live runs or to a dry-run path that prints the planned action
// without invoking restic dump or docker exec.
func runStreamRestore(
	ctx context.Context,
	opts RestoreCoreOptions,
	deps RestoreCoreDeps,
	snap restic.Snapshot,
) error {
	if opts.To != "" {
		return fmt.Errorf("stream mode: %w", errToOnStreamSnapshot)
	}

	if opts.Volume != "" {
		return fmt.Errorf("stream mode: %w", errVolumeOnStreamSnapshot)
	}

	command, err := resolveStreamCommand(ctx, opts, deps)
	if err != nil {
		return err
	}

	filename := snap.Paths[0]

	runOpts := restore.Options{
		SnapshotID: snap.ID,
		Filename:   filename,
		Container:  opts.Container,
		TargetPath: "",
		Command:    command,
		DryRun:     opts.DryRun,
		Force:      false,
		Out:        opts.Out,
	}

	if opts.DryRun {
		return runStreamDryRun(ctx, runOpts, deps)
	}

	dockerAdapter := &restoreDockerAdapter{deps: deps}

	err = restore.RunStream(ctx, runOpts, deps.Dump, dockerAdapter)
	if err != nil {
		return mapStreamError(err, opts.Container)
	}

	return nil
}

// resolveStreamCommand picks the command to pipe into the container.
// --to-command always wins. When the flag is unset and the pre-backup
// feature flag is enabled, the restore-command label is consulted. A
// missing command produces errNoRestoreCommand.
func resolveStreamCommand(
	ctx context.Context,
	opts RestoreCoreOptions,
	deps RestoreCoreDeps,
) (string, error) {
	if opts.ToCommand != "" {
		return opts.ToCommand, nil
	}

	if !opts.PreBackupEnabled {
		return "", fmt.Errorf(
			"%w: pass --to-command to pipe the stream into a command",
			errNoRestoreCommand,
		)
	}

	info, err := deps.LookupContainer(ctx, opts.Container)
	if err != nil {
		return "", fmt.Errorf("lookup container %q: %w", opts.Container, err)
	}

	target := discovery.Target{
		Container: info,
		Mount: runtime.MountInfo{
			Type:        "",
			Name:        "",
			Source:      "",
			Destination: "",
			ReadOnly:    false,
		},
	}

	spec, hasSpec, err := filter.PreBackup(target)
	if err != nil {
		return "", fmt.Errorf("parse pre-backup labels: %w", err)
	}

	if !hasSpec || spec.RestoreCommand == "" {
		return "", fmt.Errorf(
			"%w: container %q has no conba.pre-backup.restore-command "+
				"label; pass --to-command explicitly",
			errNoRestoreCommand, opts.Container,
		)
	}

	return spec.RestoreCommand, nil
}

// runStreamDryRun emits the planned action and does NOT invoke restic
// dump or docker exec. ContainerRunning is checked first so a stopped
// container fails just as it would in a live run.
func runStreamDryRun(
	ctx context.Context,
	runOpts restore.Options,
	deps RestoreCoreDeps,
) error {
	running, err := deps.ContainerRunning(ctx, runOpts.Container)
	if err != nil {
		return fmt.Errorf("check container running: %w", err)
	}

	if !running {
		return fmt.Errorf("%w: %s", restore.ErrContainerNotRunning, runOpts.Container)
	}

	_, _ = fmt.Fprintf(
		runOpts.Out,
		"would restore snapshot %s by piping %s into %s in container %s\n",
		runOpts.SnapshotID,
		runOpts.Filename,
		runOpts.Command,
		runOpts.Container,
	)

	return nil
}

// mapStreamError keeps ErrContainerNotRunning's chain intact while adding
// the container name for the user-facing message.
func mapStreamError(err error, container string) error {
	if errors.Is(err, restore.ErrContainerNotRunning) {
		return fmt.Errorf(
			"%w: container %q is not running",
			err, container,
		)
	}

	return fmt.Errorf("stream restore: %w", err)
}

// flagBool returns the bool flag value, defaulting to false on lookup error.
func flagBool(flags interface {
	GetBool(name string) (bool, error)
}, name string,
) bool {
	value, _ := flags.GetBool(name)

	return value
}

// restoreDockerAdapter wraps a RestoreCoreDeps so it satisfies the
// restore.DockerRuntime interface expected by restore.RunStream.
type restoreDockerAdapter struct {
	deps RestoreCoreDeps
}

// ContainerRunning implements restore.DockerRuntime.
func (a *restoreDockerAdapter) ContainerRunning(ctx context.Context, name string) (bool, error) {
	running, err := a.deps.ContainerRunning(ctx, name)
	if err != nil {
		return false, fmt.Errorf("container running: %w", err)
	}

	return running, nil
}

// Exec implements restore.DockerRuntime.
func (a *restoreDockerAdapter) Exec(
	ctx context.Context,
	name string,
	argv []string,
	stdin io.Reader,
) error {
	err := a.deps.Exec(ctx, name, argv, stdin)
	if err != nil {
		return fmt.Errorf("exec: %w", err)
	}

	return nil
}

// productionRestoreDeps wires the real restic client and docker runtime
// behind RestoreCoreDeps. ContainerRunning and Exec shell out to the
// docker CLI binary because the orchestration shape (argv carries
// "docker exec ...") is fixed by tests.
type productionRestoreDeps struct {
	client  *restic.Client
	runtime *docker.Client
}

// Snapshots implements RestoreCoreDeps.
func (p *productionRestoreDeps) Snapshots(
	ctx context.Context, tags []string,
) ([]restic.Snapshot, error) {
	snaps, err := p.client.Snapshots(ctx, tags)
	if err != nil {
		return nil, fmt.Errorf("restic snapshots: %w", err)
	}

	return snaps, nil
}

// Restore implements RestoreCoreDeps.
func (p *productionRestoreDeps) Restore(
	ctx context.Context, snapshotID, targetPath string, dryRun bool,
) error {
	err := p.client.Restore(ctx, snapshotID, targetPath, dryRun)
	if err != nil {
		return fmt.Errorf("restic restore: %w", err)
	}

	return nil
}

// Dump implements RestoreCoreDeps.
func (p *productionRestoreDeps) Dump(
	ctx context.Context, snapshotID, filename string, stdout io.Writer,
) error {
	err := p.client.Dump(ctx, snapshotID, filename, stdout)
	if err != nil {
		return fmt.Errorf("restic dump: %w", err)
	}

	return nil
}

// ContainerRunning implements RestoreCoreDeps. It walks the runtime's
// container list because the docker package only exposes ListContainers.
func (p *productionRestoreDeps) ContainerRunning(ctx context.Context, name string) (bool, error) {
	containers, err := p.runtime.ListContainers(ctx)
	if err != nil {
		return false, fmt.Errorf("list containers: %w", err)
	}

	for _, ctr := range containers {
		if ctr.Name == name {
			return true, nil
		}
	}

	return false, nil
}

// Exec implements RestoreCoreDeps by shelling out to the docker CLI. The
// argv passed in already starts with "docker", so we use the rest as
// arguments to the docker binary on PATH.
func (p *productionRestoreDeps) Exec(
	ctx context.Context, _ string, argv []string, stdin io.Reader,
) error {
	if len(argv) == 0 {
		return fmt.Errorf("docker exec: %w", errEmptyExecArgv)
	}

	//nolint:gosec // argv[0] is always the literal "docker"; tail is constructed by us
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Stdin = stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("docker exec: %w", err)
	}

	return nil
}

// Hostname implements RestoreCoreDeps.
func (p *productionRestoreDeps) Hostname() (string, error) {
	host, err := os.Hostname()
	if err != nil {
		return "", fmt.Errorf("get hostname: %w", err)
	}

	return host, nil
}

// LookupContainer implements RestoreCoreDeps. It lists running containers
// and returns the first matching name, mirroring discovery's lookup style.
func (p *productionRestoreDeps) LookupContainer(
	ctx context.Context, name string,
) (runtime.ContainerInfo, error) {
	containers, err := p.runtime.ListContainers(ctx)
	if err != nil {
		return runtime.ContainerInfo{}, fmt.Errorf("list containers: %w", err)
	}

	for _, ctr := range containers {
		if ctr.Name == name {
			return ctr, nil
		}
	}

	return runtime.ContainerInfo{}, fmt.Errorf("%w: %q", errContainerNotFound, name)
}
