package main

import (
	"os"
	"os/signal"

	"gitlab.com/adam.stanek/nanit/pkg/app"
	"gitlab.com/adam.stanek/nanit/pkg/mqtt"
	"gitlab.com/adam.stanek/nanit/pkg/session"
	"gitlab.com/adam.stanek/nanit/pkg/utils"
)

func main() {
	initLogger()
	logAppVersion()
	utils.LoadDotEnvFile()
	setLogLevel()

	runOpts := app.RunOpts{
		NanitCredentials: app.NanitCredentials{
			Email:    utils.EnvVarReqStr("NANIT_EMAIL"),
			Password: utils.EnvVarReqStr("NANIT_PASSWORD"),
		},
		SessionStore:    session.InitSessionStore(utils.EnvVarStr("NANIT_SESSION_FILE", "")),
		DataDirectories: ensureDataDirectories(),
		HTTPEnabled:     utils.EnvVarBool("NANIT_HTTP_ENABLED", true),
	}

	if utils.EnvVarBool("NANIT_REMOTE_STREAM_ENABLED", true) {
		runOpts.StreamProcessor = &app.StreamProcessorOpts{
			CommandTemplate: utils.EnvVarStr(
				"NANIT_REMOTE_STREAM_CMD",
				"ffmpeg -i {sourceUrl} -codec copy -hls_time 1 -hls_wrap 10 -hls_flags delete_segments -hls_segment_filename {babyUid}-%02d.ts {babyUid}.m3u8",
			),
		}
	}

	if utils.EnvVarBool("NANIT_MQTT_ENABLED", false) {
		runOpts.MQTT = &mqtt.Opts{
			BrokerURL: utils.EnvVarReqStr("NANIT_MQTT_BROKER_URL"),
			Username:  utils.EnvVarStr("NANIT_MQTT_USERNAME", ""),
			Password:  utils.EnvVarStr("NANIT_MQTT_PASSWORD", ""),
		}
	}

	if utils.EnvVarBool("NANIT_LOCAL_STREAM_ENABLED", false) {
		runOpts.LocalStreaming = &app.LocalStreamingOpts{
			PushTargetURLTemplate: utils.EnvVarReqStr("NANIT_LOCAL_STREAM_PUSH_TARGET"),
		}
	}

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	stopRunning := app.Run(runOpts)

	<-interrupt
	stopRunning()
}
