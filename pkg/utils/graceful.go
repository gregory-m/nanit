package utils

import "sync"

// GracefulContext - a context carries channel factory for cancelation
type GracefulContext interface {
	Done() <-chan struct{}
	RunAsChild(callback func(GracefulContext))
}

// RunWithGracefulCancel - runs callback as a go routine and returns cancel routine
// This is inspired by context but with the key difference that the cancel function waits until
// the handler finishes all the cleanup
// @see https://blog.golang.org/context
func RunWithGracefulCancel(callback func(GracefulContext)) func() {
	ctx := newGracefulCtx()

	go func() {
		ctx.wg.Add(1)

		// Note: we are passing it down as & because type has only * methods
		// The context module is using similar trick in order to ensure that the object
		// is going to be always passed as a reference.
		// Everthing is hidden by interface
		callback(&ctx)
		ctx.wg.Done()
	}()

	return func() {
		close(ctx.cancelC)
		ctx.wg.Wait()
	}
}

type gracefulCtx struct {
	cancelC chan struct{}
	wg      sync.WaitGroup
}

func newGracefulCtx() gracefulCtx {
	return gracefulCtx{
		cancelC: make(chan struct{}),
	}
}

func (c *gracefulCtx) Done() <-chan struct{} {
	return c.cancelC
}

func (c *gracefulCtx) RunAsChild(callback func(GracefulContext)) {
	cancel := RunWithGracefulCancel(func(childCtx GracefulContext) {
		c.wg.Add(1)
		callback(childCtx)
		c.wg.Done()
	})

	go func() {
		<-c.Done()
		cancel()
	}()
}
