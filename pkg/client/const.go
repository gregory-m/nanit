package client

import "time"

const (
	// AuthTokenTimelife - Time duration after which we assume auth token expired
	AuthTokenTimelife = 10 * time.Minute
	// SoundEventMessageType is for working with sound event messages
	SoundEventMessageType = "SOUND"
	// MotionEventMessageType is for working with motion event messages
	MotionEventMessageType = "MOTION"
)
