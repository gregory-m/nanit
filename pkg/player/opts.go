package player

import (
	"io"
	"os/exec"

	"gitlab.com/adam.stanek/nanit/pkg/baby"
)

// Opts - player options
type Opts struct {
	BabyUID          string
	URL              string
	BabyStateManager *baby.StateManager
	Executor         func(string, ...string) Command
}

func (opts Opts) applyDefaults() Opts {
	result := Opts{
		BabyUID:          opts.BabyUID,
		URL:              opts.URL,
		BabyStateManager: opts.BabyStateManager,
	}

	if opts.Executor == nil {
		result.Executor = realExecutor
	} else {
		result.Executor = opts.Executor
	}

	return result
}

// -------------------------------------------------

// Command - used exec.Command subset for easier mocking in tests
type Command interface {
	StderrPipe() (io.ReadCloser, error)
	StdoutPipe() (io.ReadCloser, error)
	Start() error
	Wait() error

	Kill() error
	ExitCode() int
}

// -------------------------------------------------

type realPlayerCommand struct {
	cmd *exec.Cmd
}

func (e *realPlayerCommand) StderrPipe() (io.ReadCloser, error) {
	return e.cmd.StderrPipe()
}

func (e *realPlayerCommand) StdoutPipe() (io.ReadCloser, error) {
	return e.cmd.StdoutPipe()
}

func (e *realPlayerCommand) Start() error {
	return e.cmd.Start()
}

func (e *realPlayerCommand) Wait() error {
	return e.cmd.Wait()
}

func (e *realPlayerCommand) ExitCode() int {
	return e.cmd.ProcessState.ExitCode()
}

func (e *realPlayerCommand) Kill() error {
	return e.cmd.Process.Kill()
}

func realExecutor(cmd string, args ...string) Command {
	return &realPlayerCommand{
		cmd: exec.Command(cmd, args...),
	}
}
