package step

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// ExecResult holds the result of a command execution.
type ExecResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
	Duration time.Duration
}

// RunCommand executes a ShellCommand with the given context and environment.
// It uses os/exec directly — NO shell interpolation.
//
// The env parameter provides step-level env vars (resolved from cfg.Env).
// ShellCommand.Env provides command-level overrides.
// Merge order: inherited process env + stepEnv + cmd.Env.
//
// If the context is cancelled or times out, SIGTERM is sent to the process group,
// followed by SIGKILL after PostSIGTERMGrace (5s).
//
// Returns an error only for infrastructure failures (binary not found, etc.).
// Non-zero exit codes are returned in ExecResult.ExitCode, not as errors.
func RunCommand(ctx context.Context, cmd *ShellCommand, stepEnv []string, stepName string, phase string) (ExecResult, error) {
	if cmd == nil {
		return ExecResult{ExitCode: -1}, fmt.Errorf("nil command")
	}
	if len(cmd.Args) == 0 {
		return ExecResult{ExitCode: -1}, fmt.Errorf("empty command args")
	}

	start := time.Now()

	c := exec.CommandContext(ctx, cmd.Args[0], cmd.Args[1:]...)

	// Set process group so we can signal the whole group on timeout.
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// Build environment: inherited + step-level + command-level.
	env := os.Environ()
	env = append(env, stepEnv...)
	for k, v := range cmd.Env {
		env = append(env, k+"="+v)
	}
	c.Env = env

	var stdout, stderr bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = &stderr

	err := c.Start()
	if err != nil {
		return ExecResult{
			ExitCode: -1,
			Duration: time.Since(start),
		}, fmt.Errorf("failed to start command %q: %w", cmd.Args[0], err)
	}

	// Wait for the process to exit or context cancellation.
	waitDone := make(chan error, 1)
	go func() {
		waitDone <- c.Wait()
	}()

	select {
	case err := <-waitDone:
		elapsed := time.Since(start)
		result := ExecResult{
			Stdout:   stdout.String(),
			Stderr:   stderr.String(),
			Duration: elapsed,
		}
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				result.ExitCode = exitErr.ExitCode()
				return result, nil
			}
			return result, fmt.Errorf("command %q failed: %w", cmd.Args[0], err)
		}
		result.ExitCode = 0
		return result, nil

	case <-ctx.Done():
		// Context cancelled/timed out — send SIGTERM to process group.
		elapsed := time.Since(start)
		log.Printf("[step:%s][%s] timed out after %s, sending SIGTERM", stepName, phase, elapsed)

		pgid, err := syscall.Getpgid(c.Process.Pid)
		if err == nil {
			_ = syscall.Kill(-pgid, syscall.SIGTERM)
		} else {
			_ = c.Process.Signal(syscall.SIGTERM)
		}

		// Wait for graceful shutdown.
		graceTimer := time.NewTimer(PostSIGTERMGrace)
		defer graceTimer.Stop()

		select {
		case <-waitDone:
			// Process exited after SIGTERM.
		case <-graceTimer.C:
			// Grace period expired — send SIGKILL.
			log.Printf("[step:%s][%s] grace period expired, sending SIGKILL", stepName, phase)
			if pgid, err := syscall.Getpgid(c.Process.Pid); err == nil {
				_ = syscall.Kill(-pgid, syscall.SIGKILL)
			} else {
				_ = c.Process.Kill()
			}
			<-waitDone
		}

		return ExecResult{
			ExitCode: -1,
			Stdout:   stdout.String(),
			Stderr:   stderr.String(),
			Duration: time.Since(start),
		}, nil // timeout is NOT an infra error; caller checks context
	}
}
