package filter

import (
	"errors"
	"fmt"

	"github.com/lazybytez/conba/internal/discovery"
)

// Container label keys controlling pre-backup command execution.
const (
	LabelPreBackupCommand   = "conba.pre-backup.command"
	LabelPreBackupMode      = "conba.pre-backup.mode"
	LabelPreBackupContainer = "conba.pre-backup.container"
	LabelPreBackupFilename  = "conba.pre-backup.filename"
)

// Mode is the execution mode for a pre-backup command.
type Mode string

// Supported pre-backup modes.
const (
	ModeReplace   Mode = "replace"
	ModeAlongside Mode = "alongside"
)

// ErrInvalidPreBackupMode signals that the pre-backup mode label carries a
// value that is neither "replace" nor "alongside".
var ErrInvalidPreBackupMode = errors.New("invalid pre-backup mode")

// Spec holds the parsed pre-backup configuration for a target.
//
// An empty Container means "run the command in the labeled container itself".
// An empty Filename means "use the labeled container's name as the filename".
type Spec struct {
	Command   string
	Mode      Mode
	Container string
	Filename  string
}

// PreBackup parses the conba.pre-backup.* labels from the target's container.
//
// Returns (zero, false, nil) when the command label is absent or empty. An
// empty value is treated as "no spec" because an empty command would produce
// a useless empty stream snapshot, never the user's intent.
// Returns (populated, true, nil) when the command label is set; an empty mode
// label defaults to ModeReplace.
// Returns (zero, false, error) when the mode label has an unsupported value.
func PreBackup(target discovery.Target) (Spec, bool, error) {
	labels := target.Container.Labels

	command, ok := labels[LabelPreBackupCommand]
	if !ok || command == "" {
		return Spec{Command: "", Mode: "", Container: "", Filename: ""}, false, nil
	}

	mode, err := parseMode(labels[LabelPreBackupMode])
	if err != nil {
		return Spec{Command: "", Mode: "", Container: "", Filename: ""}, false, err
	}

	return Spec{
		Command:   command,
		Mode:      mode,
		Container: labels[LabelPreBackupContainer],
		Filename:  labels[LabelPreBackupFilename],
	}, true, nil
}

func parseMode(raw string) (Mode, error) {
	switch raw {
	case "":
		return ModeReplace, nil
	case string(ModeReplace):
		return ModeReplace, nil
	case string(ModeAlongside):
		return ModeAlongside, nil
	default:
		return "", fmt.Errorf("%w: %q", ErrInvalidPreBackupMode, raw)
	}
}
