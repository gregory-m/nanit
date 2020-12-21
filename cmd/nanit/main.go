package main

import (
	"os"
	"os/signal"

	"github.com/rs/zerolog/log"
	"gitlab.com/adam.stanek/nanit/pkg/app"
	"gitlab.com/adam.stanek/nanit/pkg/mqtt"
	"gitlab.com/adam.stanek/nanit/pkg/utils"
)

func main() {
	initLogger()
	logAppVersion()
	utils.LoadDotEnvFile()
	setLogLevel()

	opts := app.Opts{
		NanitCredentials: app.NanitCredentials{
			Email:    utils.EnvVarReqStr("NANIT_EMAIL"),
			Password: utils.EnvVarReqStr("NANIT_PASSWORD"),
		},
		SessionFile:     utils.EnvVarStr("NANIT_SESSION_FILE", ""),
		DataDirectories: ensureDataDirectories(),
		HTTPEnabled:     false,
	}

	if utils.EnvVarBool("NANIT_HLS_ENABLED", true) {
		opts.HTTPEnabled = true
		opts.StreamProcessor = &app.StreamProcessorOpts{
			CommandTemplate: utils.EnvVarStr(
				"NANIT_HLS_CMD",
				"ffmpeg -i {remoteStreamUrl} -codec copy -hls_time 1 -hls_wrap 10 -hls_flags delete_segments -hls_segment_filename {babyUid}-%02d.ts {babyUid}.m3u8",
			),
		}
	}

	if utils.EnvVarBool("NANIT_MQTT_ENABLED", false) {
		opts.MQTT = &mqtt.Opts{
			BrokerURL:   utils.EnvVarReqStr("NANIT_MQTT_BROKER_URL"),
			ClientID:    utils.EnvVarStr("NANIT_MQTT_CLIENT_ID", "nanit"),
			Username:    utils.EnvVarStr("NANIT_MQTT_USERNAME", ""),
			Password:    utils.EnvVarStr("NANIT_MQTT_PASSWORD", ""),
			TopicPrefix: utils.EnvVarStr("NANIT_MQTT_PREFIX", "nanit"),
		}
	}

	if utils.EnvVarBool("NANIT_LOCAL_STREAM_ENABLED", false) {
		opts.LocalStreaming = &app.LocalStreamingOpts{
			PushTargetURLTemplate: utils.EnvVarReqStr("NANIT_LOCAL_STREAM_PUSH_TARGET"),
		}
	}

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	instance := app.NewApp(opts)

	runner := utils.RunWithGracefulCancel(instance.Run)

	<-interrupt
	log.Warn().Msg("Received interrupt signal, terminating")

	waitForCleanup := make(chan struct{}, 1)

	go func() {
		runner.Cancel()
		close(waitForCleanup)
	}()

	select {
	case <-interrupt:
		log.Fatal().Msg("Received another interrupt signal, forcing termination without clean up")
	case <-waitForCleanup:
		log.Info().Msg("Clean exit")
		return
	}
}
