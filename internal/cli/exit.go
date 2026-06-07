package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/lazybytez/conba/internal/backup"
	"github.com/lazybytez/conba/internal/config"
	"github.com/lazybytez/conba/internal/forget"
	"github.com/lazybytez/conba/internal/report"
	"github.com/lazybytez/conba/internal/restic"
)

// Process exit codes. They let automation distinguish a misconfiguration
// from a partial backup failure from a total failure.
const (
	exitOK          = 0
	exitUsage       = 1
	exitPartialFail = 2
	exitFatal       = 3
)

// Main runs the conba CLI and returns the process exit code. It is the
// single entry point called by package main.
func Main() int {
	err := Execute()
	if err == nil {
		return exitOK
	}

	reportFatal(err)

	return exitCode(err)
}

// reportFatal emits the terminal error: a fatal event on stdout in json
// mode (resolved from CONBA_OUTPUT_FORMAT and the stdout terminal), or an
// "error:" line on stderr otherwise. The reporter built in the command
// context is gone by the time Execute returns, so the mode is re-resolved
// here without the --output flag.
func reportFatal(err error) {
	mode, _ := report.Resolve("", os.Getenv("CONBA_OUTPUT_FORMAT"), os.Stdout)
	if mode == report.ModeJSON {
		report.New(report.ModeJSON, os.Stdout, false).Emit(report.Event{
			Level:   report.LevelError,
			Name:    "fatal",
			Message: err.Error(),
			Fields:  []report.Field{report.F("error", err.Error())},
		})

		return
	}

	_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
}

// exitCode maps err to a process exit code: exitUsage for
// config/precondition errors, exitPartialFail when a backup/forget cycle
// had at least one success alongside failures, and exitFatal for a total
// failure or any unclassified error.
func exitCode(err error) int {
	var (
		backupErr *backup.RunError
		forgetErr *forget.RunError
	)

	switch {
	case errors.As(err, &backupErr):
		return targetsExitCode(backupErr.Succeeded)
	case errors.As(err, &forgetErr):
		return targetsExitCode(forgetErr.Succeeded)
	case isPreconditionError(err):
		return exitUsage
	default:
		return exitFatal
	}
}

func targetsExitCode(succeeded int) int {
	if succeeded > 0 {
		return exitPartialFail
	}

	return exitFatal
}

func isPreconditionError(err error) bool {
	preconditions := []error{
		errMissingConfig,
		config.ErrMissingRepository,
		config.ErrMissingPassword,
		config.ErrInvalidLogLevel,
		config.ErrInvalidOutputFormat,
		config.ErrInvalidRuntimeType,
		config.ErrInvalidFilterPattern,
		restic.ErrRepoLocked,
		restic.ErrRepoNotInitialized,
	}

	for _, sentinel := range preconditions {
		if errors.Is(err, sentinel) {
			return true
		}
	}

	return false
}
