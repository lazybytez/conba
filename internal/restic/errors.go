package restic

import (
	"errors"
	"strings"
)

// Sentinel errors for classifiable restic failure conditions.
var (
	ErrRepoNotInitialized = errors.New("repository not initialized")
	ErrRepoLocked         = errors.New("repository is locked")
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
	case containsAny(msg, msgRepoNotFound, msgConfigMissing, msgRepoUnset):
		return ErrRepoNotInitialized
	case containsAny(msg, msgLockFailed, msgAlreadyLocked):
		return ErrRepoLocked
	default:
		return err
	}
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}

	return false
}
