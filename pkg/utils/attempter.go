package utils

import (
	"time"

	"github.com/rs/zerolog/log"
)

// AttemptContext - attempt context container
type AttemptContext interface {
	GracefulContext

	// GetTry - returns assigned number of tries
	GetTry() int
}

// PerseverenceOpts - options container for RunWithPerseverance
type PerseverenceOpts struct {
	// Cooldown - List of cooldown periods for failed attempts.
	// If execution fails more times than length of this array, last item is used.
	Cooldown []time.Duration

	// ResetThreshold - After this time failed attempts are counted as first failure
	ResetThreshold time.Duration
}

// RunWithPerseverance - runs handler and tries it again if it fails
func RunWithPerseverance(handler func(AttemptContext), ctx GracefulContext, opts PerseverenceOpts) {
	try := 1
	timer := time.NewTimer(0)

	for {
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case timeScheduled := <-timer.C:
			err := ctx.RunAsChild(func(childGracefulCtx GracefulContext) {
				handler(newAttemptCtx(try, childGracefulCtx))
			}).Wait()

			timeTaken := time.Since(timeScheduled)

			if err == nil {
				return
			} else if opts.ResetThreshold > 0 && timeTaken > opts.ResetThreshold {
				log.Trace().Msgf("Previous attempt was %v ago, resetting tries", timeTaken)
				try = 1
			} else {
				cooldown := opts.Cooldown[MinInt(try, len(opts.Cooldown))-1]
				try++

				if cooldown > timeTaken {
					timer.Reset(cooldown - timeTaken)
				} else {
					timer.Reset(0)
				}
			}
		}
	}
}

type attemptCtx struct {
	try         int
	gracefulCtx GracefulContext
}

func newAttemptCtx(try int, gracefulCtx GracefulContext) *attemptCtx {
	return &attemptCtx{try, gracefulCtx}
}

func (ctx *attemptCtx) GetTry() int           { return ctx.try }
func (ctx *attemptCtx) Done() <-chan struct{} { return ctx.gracefulCtx.Done() }
func (ctx *attemptCtx) Fail(err error)        { ctx.gracefulCtx.Fail(err) }
func (ctx *attemptCtx) RunAsChild(callback func(GracefulContext)) GracefulRunner {
	return ctx.gracefulCtx.RunAsChild(callback)
}
