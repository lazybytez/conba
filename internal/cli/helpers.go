package cli

import (
	"errors"
	"fmt"

	"github.com/lazybytez/conba/internal/config"
)

var (
	errMissingConfig = errors.New("config not available in context")

	errMissingRepository = errors.New(
		"restic.repository is required but not configured",
	)
	errMissingPassword = errors.New(
		"restic.password or restic.password_file is required but not configured",
	)
)

// requireResticConfig validates that the restic configuration has the
// minimum required fields to interact with a repository.
func requireResticConfig(cfg config.ResticConfig) error {
	if cfg.Repository == "" {
		return fmt.Errorf(
			"%w: set it in the config file or via CONBA_RESTIC_REPOSITORY",
			errMissingRepository,
		)
	}

	if cfg.Password == "" && cfg.PasswordFile == "" {
		return fmt.Errorf(
			"%w: set it in the config file or via CONBA_RESTIC_PASSWORD",
			errMissingPassword,
		)
	}

	return nil
}
