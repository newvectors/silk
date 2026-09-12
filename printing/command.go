package printing

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type commandRunner interface {
	LookPath(string) (string, error)
	Run(context.Context, string, []string) (string, error)
}
type systemRunner struct{}

func (systemRunner) LookPath(name string) (string, error) { return exec.LookPath(name) }
func (systemRunner) Run(ctx context.Context, name string, args []string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	// Parse CUPS output independently of the desktop locale. Do not change
	// LPDEST/PRINTER: the user's configured default destination is respected.
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C")
	cmd.WaitDelay = 2 * time.Second
	out := &boundedOutput{}
	cmd.Stdout, cmd.Stderr = out, out
	err := cmd.Run()
	return out.String(), err
}

type boundedOutput struct {
	mu   sync.Mutex
	text strings.Builder
}

func (b *boundedOutput) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(p)
	if remaining := 8192 - b.text.Len(); remaining > 0 {
		if len(p) > remaining {
			p = p[:remaining]
		}
		b.text.Write(p)
	}
	return n, nil
}
func (b *boundedOutput) String() string { b.mu.Lock(); defer b.mu.Unlock(); return b.text.String() }

func findCommand(r commandRunner, name string) (string, error) {
	path, err := r.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("%w: %s: %v", ErrBackendUnavailable, name, err)
	}
	return path, nil
}

func commandError(ctx context.Context, operation, output string, err error, submitting bool) error {
	if ctx.Err() != nil {
		kind := ErrCanceled
		if submitting {
			kind = ErrSubmissionUnknown
		}
		return errors.Join(kind, ctx.Err())
	}
	return fmt.Errorf("printing: %s: %w (%s)", operation, err, strings.TrimSpace(output))
}
