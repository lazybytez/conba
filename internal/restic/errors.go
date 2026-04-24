package restic

import (
	"errors"

	"github.com/lazybytez/conba/internal/support/stringutil"
)

// Sentinel errors for classifiable restic failure conditions.
var (
	// ErrRepoNotInitialized indicates the configured restic repository
	// does not exist (or has not been initialised yet) at its location.
	ErrRepoNotInitialized = errors.New("repository not initialized")

	// ErrRepoLocked indicates restic refused to run because another
	// process holds the repository lock.
	ErrRepoLocked = errors.New("repository is locked")

	// ErrSourceUnreadable indicates a backup source path could not be read
	// (does not exist, permission denied) and the target should be skipped
	// rather than counted as a failure.
	ErrSourceUnreadable = errors.New("backup source unreadable")
)

// Known restic stderr patterns used to classify errors.
const (
	msgRepoNotFound  = "Is there a repository at the following location?"
	msgConfigMissing = "unable to open config file"
	msgRepoUnset     = "Please specify repository location"
	msgLockFailed    = "unable to create lock"
	msgAlreadyLocked = "repository is already locked"
)

// ClassifyError inspects a restic error and returns a sentinel error
// if the failure matches a known condition. If the error is not
// recognizable, the original error is returned unchanged.
func ClassifyError(err error) error {
	if err == nil {
		return nil
	}

	msg := err.Error()

	switch {
	case stringutil.ContainsAny(msg, msgRepoNotFound, msgConfigMissing, msgRepoUnset):
		return ErrRepoNotInitialized
	case stringutil.ContainsAny(msg, msgLockFailed, msgAlreadyLocked):
		return ErrRepoLocked
	default:
		return err
	}
}
