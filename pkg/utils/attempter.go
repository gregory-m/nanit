package utils

import (
	"time"

	"github.com/rs/zerolog/log"
)

// Attempt - holds state of a single attempt
type Attempt struct {
	Number      int
	ScheduledAt time.Time
	InterruptC  chan bool
	DoneC       chan bool
	Running     bool
}

// NewAttempt - constructor
func NewAttempt(number int, scheduledAt time.Time) *Attempt {
	return &Attempt{
		Number:      number,
		ScheduledAt: scheduledAt,
		InterruptC:  make(chan bool, 1),
	}
}

// Attempter - holds configuration and current state of attempter
type Attempter struct {
	Handler func(*Attempt) error

	AttemptC   chan *Attempt
	InterruptC chan func()

	CurrentAttempt *Attempt

	// Cooldown - List of cooldown periods for failed attempts.
	// If execution fails more times than length of this array, last item is used.
	Cooldown []time.Duration

	// ResetThreshold - After this time failed attempts are counted as first failure
	ResetThreshold time.Duration

	HasFinished bool
}

// NewAttempter - constructor
func NewAttempter(handler func(*Attempt) error, cooldown []time.Duration, resetThreshold time.Duration) *Attempter {
	attempter := &Attempter{
		AttemptC:       make(chan *Attempt, 1),
		InterruptC:     make(chan func(), 1),
		Cooldown:       cooldown,
		Handler:        handler,
		ResetThreshold: resetThreshold,
		HasFinished:    false,
	}

	return attempter
}

func failAttempt(attempter *Attempter, attempt *Attempt, err error) {
	now := time.Now()
	timeAgo := now.Sub(attempt.ScheduledAt)

	log.Trace().Int("attempt", attempt.Number).Err(err).Msg("Attempt failed")

	var nextTryNumber int
	if attempter.ResetThreshold > 0 && timeAgo > attempter.ResetThreshold {
		log.Trace().Msgf("Previous attempt was %v ago, resetting tries", timeAgo)

		nextTryNumber = 1
	} else {
		nextTryNumber = attempt.Number + 1
	}

	if nextTryNumber == 1 || len(attempter.Cooldown) == 0 {
		attempter.AttemptC <- NewAttempt(1, now)
	} else {
		cooldown := attempter.Cooldown[MinInt(nextTryNumber-1, len(attempter.Cooldown))-1]
		if cooldown > timeAgo {
			attempter.AttemptC <- NewAttempt(nextTryNumber, now.Add(cooldown-timeAgo))
		} else {
			attempter.AttemptC <- NewAttempt(nextTryNumber, now)
		}
	}
}

// Run - triggers attempter main loop
func (attempter *Attempter) Run() {
	timer := time.NewTimer(0)
	attempt := NewAttempt(1, time.Now())

	for {
		select {
		case done := <-attempter.InterruptC:
			timer.Stop()
			done()
			return

		case <-timer.C:
			log.Trace().Msg("Starting attempt")
			attempter.CurrentAttempt = attempt
			err := attempter.Handler(attempt)
			attempter.CurrentAttempt = nil

			if err != nil {
				failAttempt(attempter, attempt, err)
			} else {
				attempter.HasFinished = true

				select {
				case done := <-attempter.InterruptC:
					done()
					return
				default:
					log.Trace().Msg("Attempt handler finished")
					return
				}
			}

		case attempt = <-attempter.AttemptC:
			now := time.Now()
			cooldownDuration := attempt.ScheduledAt.Sub(now)
			if cooldownDuration > 0 {
				log.Debug().Time("at", now.Add(cooldownDuration)).Msgf("Next attempt scheduled %v from now", cooldownDuration)
				timer.Reset(cooldownDuration)
			} else {
				timer.Reset(0)
			}
		}
	}
}

// Stop - notifies attempter to stop the current run and prevents him from scheduling the next one.
// This will cause Run() to return.
func (attempter *Attempter) Stop() {
	if !attempter.HasFinished {
		attempt := attempter.CurrentAttempt
		if attempt != nil {
			attempt.InterruptC <- true
		}

		waitC := make(chan bool, 1)
		attempter.InterruptC <- func() { waitC <- true }
		<-waitC
	}
}
