package restic_test

import (
	"context"
	"errors"
	"testing"

	"github.com/lazybytez/conba/internal/restic"
)

func TestForget_Success(t *testing.T) {
	t.Parallel()

	client := newHelperClient(t, 0, "", "")

	err := client.Forget(context.Background(), []string{"container=app"}, restic.ForgetPolicy{
		KeepDaily:   7,
		KeepWeekly:  4,
		KeepMonthly: 12,
		KeepYearly:  3,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestForget_Failure(t *testing.T) {
	t.Parallel()

	client := newHelperClient(t, 1, "", "unable to prune")

	err := client.Forget(context.Background(), nil, restic.ForgetPolicy{
		KeepDaily:   0,
		KeepWeekly:  0,
		KeepMonthly: 0,
		KeepYearly:  0,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, restic.ErrResticFailed) {
		t.Errorf("expected error wrapping ErrResticFailed, got %v", err)
	}
}
