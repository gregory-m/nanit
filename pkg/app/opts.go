package app

import (
	"gitlab.com/adam.stanek/nanit/pkg/mqtt"
	"gitlab.com/adam.stanek/nanit/pkg/session"
)

// RunOpts - application run options
type RunOpts struct {
	NanitCredentials NanitCredentials
	SessionStore     *session.Store
	DataDirectories  DataDirectories
	HTTPEnabled      bool
	MQTT             *mqtt.Opts
	StreamProcessor  *StreamProcessorOpts
	LocalStreaming   *LocalStreamingOpts
}

// NanitCredentials - user credentials for Nanit account
type NanitCredentials struct {
	Email    string
	Password string
}

// DataDirectories - dictionary of dir paths
type DataDirectories struct {
	BaseDir  string
	VideoDir string
	LogDir   string
}

// StreamProcessorOpts - options to run stream processor
type StreamProcessorOpts struct {
	CommandTemplate string
}

// LocalStreamingOpts - options for local streaming
type LocalStreamingOpts struct {
	PushTargetURLTemplate string
}
