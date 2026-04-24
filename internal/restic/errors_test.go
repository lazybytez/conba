package restic_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/lazybytez/conba/internal/restic"
)

var errClassifyStub = errors.New("stub")

func TestClassifyError_Nil(t *testing.T) {
	t.Parallel()

	err := restic.ClassifyError(nil)
	if err != nil {
		t.Errorf("want nil, got %v", err)
	}
}

func TestClassifyError_NotInitialized(t *testing.T) {
	t.Parallel()

	err := restic.ClassifyError(
		fmt.Errorf("Is there a repository at the following location?: %w", errClassifyStub),
	)

	if !errors.Is(err, restic.ErrRepoNotInitialized) {
		t.Errorf("want ErrRepoNotInitialized, got %v", err)
	}
}

func TestClassifyError_ConfigFileMissing(t *testing.T) {
	t.Parallel()

	err := restic.ClassifyError(
		fmt.Errorf("unable to open config file: %w", errClassifyStub),
	)

	if !errors.Is(err, restic.ErrRepoNotInitialized) {
		t.Errorf("want ErrRepoNotInitialized, got %v", err)
	}
}

func TestClassifyError_SpecifyRepo(t *testing.T) {
	t.Parallel()

	err := restic.ClassifyError(
		fmt.Errorf("Please specify repository location: %w", errClassifyStub),
	)

	if !errors.Is(err, restic.ErrRepoNotInitialized) {
		t.Errorf("want ErrRepoNotInitialized, got %v", err)
	}
}

func TestClassifyError_Locked(t *testing.T) {
	t.Parallel()

	err := restic.ClassifyError(
		fmt.Errorf("unable to create lock: %w", errClassifyStub),
	)

	if !errors.Is(err, restic.ErrRepoLocked) {
		t.Errorf("want ErrRepoLocked, got %v", err)
	}
}

func TestClassifyError_AlreadyLocked(t *testing.T) {
	t.Parallel()

	err := restic.ClassifyError(
		fmt.Errorf("repository is already locked: %w", errClassifyStub),
	)

	if !errors.Is(err, restic.ErrRepoLocked) {
		t.Errorf("want ErrRepoLocked, got %v", err)
	}
}

func TestClassifyError_Unknown(t *testing.T) {
	t.Parallel()

	original := fmt.Errorf("some other error: %w", errClassifyStub)
	err := restic.ClassifyError(original)

	if !errors.Is(err, errClassifyStub) {
		t.Errorf("want original error returned, got %v", err)
	}
}

func TestErrSourceUnreadable_WrappedPreservesIdentity(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("wrapped: %w", restic.ErrSourceUnreadable)

	if !errors.Is(err, restic.ErrSourceUnreadable) {
		t.Errorf("want ErrSourceUnreadable, got %v", err)
	}
}
