package client

import "time"

const (
	// AuthTokenTimelife - Time duration after which we assume auth token expired
	AuthTokenTimelife = 10 * time.Minute
)
