package player_test

import (
	"io"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gitlab.com/adam.stanek/nanit/pkg/baby"
	"gitlab.com/adam.stanek/nanit/pkg/player"
	"gitlab.com/adam.stanek/nanit/pkg/utils"
)

// ---------------------

func TestPlayer(t *testing.T) {
	utils.RunWithGracefulCancel(func(ctx utils.GracefulContext) {
		mockedExecutor := newCommandMock(func(mock *commandMock) error {
			go func() {
				time.Sleep(2 * time.Second)
				mock.Finish(0)
			}()

			return nil
		})

		opts := player.Opts{
			BabyUID:          "xxxxxxxx",
			URL:              "rtmp://127.0.0.1:1935/live/local",
			BabyStateManager: baby.NewStateManager(),
			Executor:         mockedExecutor,
		}

		player.Run(opts, ctx)
	}).Wait()
}

func init() {
	zerolog.SetGlobalLevel(zerolog.TraceLevel)
	consoleWriter := zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC822}
	log.Logger = log.Output(consoleWriter)
}

// ---------------------

type commandMock struct {
	stdoutReader *io.PipeReader
	stdoutWriter *io.PipeWriter
	stderrReader *io.PipeReader
	stderrWriter *io.PipeWriter

	exitC    chan struct{}
	exitCode int

	startCallback func(*commandMock) error
}

func newCommandMock(startCallback func(*commandMock) error) func(cmd string, args ...string) player.Command {
	mockedCommand := &commandMock{
		startCallback: startCallback,
		exitC:         make(chan struct{}, 1),
	}

	mockedCommand.stderrReader, mockedCommand.stderrWriter = io.Pipe()
	mockedCommand.stdoutReader, mockedCommand.stdoutWriter = io.Pipe()

	return func(cmd string, args ...string) player.Command { return mockedCommand }
}

func (e *commandMock) StderrPipe() (io.ReadCloser, error) {
	return e.stderrReader, nil
}

func (e *commandMock) StdoutPipe() (io.ReadCloser, error) {
	return e.stdoutReader, nil
}

func (e *commandMock) Start() error {
	return e.startCallback(e)
}

func (e *commandMock) Wait() error {
	<-e.exitC
	return nil
}

func (e *commandMock) ExitCode() int {
	return e.exitCode
}

func (e *commandMock) Kill() error {
	e.Finish(-1)
	return nil
}

func (e *commandMock) Finish(exitCode int) {
	e.stderrWriter.Close()
	e.stdoutWriter.Close()
	e.exitCode = exitCode
	e.exitC <- struct{}{}
}

func (e *commandMock) WriteStderrLine(line string) {
	io.WriteString(e.stderrWriter, line+"\n")
}
