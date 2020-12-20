package utils

import (
	"errors"
	"sync"
)

// GracefulContext - a context carries channel factory for cancelation
type GracefulContext interface {
	// Done - blocks until cancelled
	Done() <-chan struct{}

	// RunAsChild - runs handler within child context
	RunAsChild(callback func(GracefulContext))

	// Fail - cancels run from the inside and propagates cancel to all children
	// Does not await the cancellation (obviously)
	Fail(err error)
}

// GracefulRunner - outter API for controlling gracefully run handlers
type GracefulRunner interface {
	// Wait - blocks until finishes execution
	Wait() error

	// Cancel - notifies handler to cancel the execution and awaits graceful return (clean up)
	Cancel()
}

type gracefulRunner struct {
	ctx *gracefulCtx
}

func newGracefulRunner(ctx *gracefulCtx) *gracefulRunner {
	return &gracefulRunner{ctx}
}

func (runner *gracefulRunner) Wait() error {
	runner.ctx.wg.Wait()
	return runner.ctx.err
}

func (runner *gracefulRunner) Cancel() {
	runner.ctx.Fail(errors.New("cancelled execution"))
	runner.ctx.wg.Wait()
}

// RunWithGracefulCancel - runs callback as a go routine and returns cancel routine
// This is inspired by context but with the key difference that the cancel function waits until
// the handler finishes all the cleanup
// @see https://blog.golang.org/context
func RunWithGracefulCancel(callback func(GracefulContext)) GracefulRunner {
	ctx := newGracefulCtx()
	ctx.wg.Add(1)

	go func() {
		callback(ctx)
		ctx.wg.Done()
	}()

	return newGracefulRunner(ctx)
}

type gracefulCtx struct {
	cancelC         chan struct{}
	wg              sync.WaitGroup
	mutex           sync.Mutex
	hasBeenCanceled bool
	err             error
}

func newGracefulCtx() *gracefulCtx {
	return &gracefulCtx{
		cancelC:         make(chan struct{}),
		hasBeenCanceled: false,
	}
}

func (c *gracefulCtx) Done() <-chan struct{} {
	return c.cancelC
}

func (c *gracefulCtx) RunAsChild(callback func(GracefulContext)) {
	c.wg.Add(1)
	runner := RunWithGracefulCancel(func(childCtx GracefulContext) {
		callback(childCtx)
		c.wg.Done()
	})

	go func() {
		<-c.Done()
		runner.Cancel()
	}()
}

func (c *gracefulCtx) Fail(err error) {
	c.mutex.Lock()
	if c.hasBeenCanceled {
		c.mutex.Unlock()
		return
	}

	c.hasBeenCanceled = true
	c.err = err
	close(c.cancelC)
	c.mutex.Unlock()
}
