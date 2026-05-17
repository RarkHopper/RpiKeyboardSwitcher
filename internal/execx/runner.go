package execx

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
)

type Runner interface {
	Run(ctx context.Context, stdin io.Reader, stdout io.Writer, stderr io.Writer, name string, args ...string) error
}

type OSRunner struct{}

func (OSRunner) Run(ctx context.Context, stdin io.Reader, stdout io.Writer, stderr io.Writer, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run %s: %w", name, err)
	}

	return nil
}

type exitCoder interface {
	ExitCode() int
}

func ExitCode(err error) int {
	var coder exitCoder
	if errors.As(err, &coder) {
		return coder.ExitCode()
	}

	return 1
}
