package apply

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/mono0x/tamis/internal/plan"
)

func runShell(ctx context.Context, source string, command plan.Command) error {
	source, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/bin/sh", "-eu", "-c", command.Run)
	cmd.Env = append(os.Environ(), "TAMIS_FILE="+source)
	for _, name := range slices.Sorted(maps.Keys(command.Env)) {
		cmd.Env = append(cmd.Env, name+"="+command.Env[name])
	}
	cmd.Stdout = io.Discard
	var output shellOutput
	cmd.Stderr = &output
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	cmd.WaitDelay = time.Second
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start shell: %w", err)
	}
	// End background children too, before accepting the source's new identity.
	defer func() { _ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }()
	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		return fmt.Errorf("shell failed for %s: %w: %s", source, err, strings.TrimSpace(output.String()))
	}
	return ctx.Err()
}

type shellOutput struct{ strings.Builder }

func (o *shellOutput) Write(p []byte) (int, error) {
	n := len(p)
	remaining := 16*1024 - o.Len()
	if remaining > 0 {
		_, _ = o.Builder.Write(p[:min(len(p), remaining)])
	}
	return n, nil
}
