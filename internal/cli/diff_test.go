package cli_test

import (
	"testing"

	"github.com/lazybytez/conba/internal/cli"
)

func TestNewDiffCommand_Use(t *testing.T) {
	t.Parallel()

	cmd := cli.NewDiffCommand()
	if cmd.Use != "diff <snapshot-a> <snapshot-b>" {
		t.Errorf("Use = %q, want %q", cmd.Use, "diff <snapshot-a> <snapshot-b>")
	}
}

func TestNewDiffCommand_Short(t *testing.T) {
	t.Parallel()

	cmd := cli.NewDiffCommand()
	if cmd.Short == "" {
		t.Error("Short description must not be empty")
	}
}

func TestNewDiffCommand_RequiresExactlyTwoArgs(t *testing.T) {
	t.Parallel()

	cmd := cli.NewDiffCommand()
	if cmd.Args == nil {
		t.Fatal("Args validator must be set")
	}

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "zero args", args: nil, wantErr: true},
		{name: "one arg", args: []string{"abc"}, wantErr: true},
		{name: "two args", args: []string{"abc", "def"}, wantErr: false},
		{name: "three args", args: []string{"abc", "def", "ghi"}, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := cmd.Args(cmd, test.args)
			if test.wantErr && err == nil {
				t.Errorf("Args(%v) returned nil error, want error", test.args)
			}

			if !test.wantErr && err != nil {
				t.Errorf("Args(%v) returned %v, want nil", test.args, err)
			}
		})
	}
}

func TestRunDiff_NilConfig(t *testing.T) {
	t.Parallel()
	assertRunEFailsWithoutConfig(t, cli.NewDiffCommand)
}

func TestRunDiff_MissingRepository(t *testing.T) {
	t.Parallel()
	assertRunEFailsWithMissingRepo(t, cli.NewDiffCommand)
}
