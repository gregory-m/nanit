package main

import (
	"os"
	"os/signal"
	"regexp"

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

	if utils.EnvVarBool("NANIT_RTMP_ENABLED", true) {
		publicAddr := utils.EnvVarReqStr("NANIT_RTMP_ADDR")
		m := regexp.MustCompile("(:[0-9]+)$").FindStringSubmatch(publicAddr)
		if len(m) != 2 {
			log.Fatal().Msg("Invalid NANIT_RTMP_ADDR. Unable to parse port.")
		}

		opts.RTMP = &app.RTMPOpts{
			ListenAddr: m[1],
			PublicAddr: publicAddr,
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
