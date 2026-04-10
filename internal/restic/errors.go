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

// ClassifyError inspects a restic error and returns a sentinel error
// if the failure matches a known condition. If the error is not
// recognizable, the original error is returned unchanged.
func ClassifyError(err error) error {
	if err == nil {
		return nil
	}

	msg := err.Error()

	if containsAny(msg,
		"Is there a repository at the following location?",
		"unable to open config file",
		"Please specify repository location",
	) {
		return ErrRepoNotInitialized
	}

	if containsAny(msg,
		"unable to create lock",
		"repository is already locked",
	) {
		return ErrRepoLocked
	}

	return err
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}

	return false
}
