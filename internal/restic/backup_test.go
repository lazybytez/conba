package restic_test

import (
	"context"
	"errors"
	"testing"

	"github.com/lazybytez/conba/internal/restic"
)

func TestBackup_Success(t *testing.T) {
	t.Parallel()

	client := newHelperClient(t, 0, "{}", "")

	err := client.Backup(context.Background(), "/data", []string{"daily"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestBackup_Failure(t *testing.T) {
	t.Parallel()

	client := newHelperClient(t, 1, "", "repository does not exist")

	err := client.Backup(context.Background(), "/data", []string{"daily"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, restic.ErrResticFailed) {
		t.Errorf("expected error wrapping ErrResticFailed, got %v", err)
	}
}
