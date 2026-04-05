package restic

import (
	"sort"
	"strings"

	"github.com/lazybytez/conba/internal/config"
)

// BuildEnv returns a slice of KEY=VALUE strings representing the environment
// variables required by restic. Extra environment entries are applied first,
// then explicit config fields override them. PasswordFile takes priority
// over Password.
func BuildEnv(cfg config.ResticConfig) []string {
	env := make(map[string]string)

	for key, value := range cfg.Environment {
		env[strings.ToUpper(key)] = value
	}

	env["RESTIC_REPOSITORY"] = cfg.Repository

	switch {
	case cfg.PasswordFile != "":
		env["RESTIC_PASSWORD_FILE"] = cfg.PasswordFile
	case cfg.Password != "":
		env["RESTIC_PASSWORD"] = cfg.Password
	}

	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, key+"="+env[key])
	}

	return result
}
