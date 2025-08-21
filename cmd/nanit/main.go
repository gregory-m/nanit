package main

import (
	"flag"
	"os"
	"os/signal"
	"regexp"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/gregory-m/nanit/pkg/app"
	"github.com/gregory-m/nanit/pkg/mqtt"
	"github.com/gregory-m/nanit/pkg/utils"
)

var doLogin = flag.Bool("l", false, "Do login")

func main() {
	flag.Parse()
	initLogger()
	logAppVersion()
	utils.LoadDotEnvFile()
	setLogLevel()

	var refresh_token = ""

	if *doLogin {
		refresh_token = Login("", "", "sms")
	}

	opts := app.Opts{
		NanitCredentials: app.NanitCredentials{
			RefreshToken: utils.EnvVarStr("NANIT_REFRESH_TOKEN", refresh_token),
		},
		SessionFile:     utils.EnvVarStr("NANIT_SESSION_FILE", "data/session.json"),
		DataDirectories: ensureDataDirectories(),
		HTTPEnabled:     false,
		EventPolling: app.EventPollingOpts{
			// Event message polling disabled by default
			Enabled: utils.EnvVarBool("NANIT_EVENTS_POLLING", false),
			// 30 second default polling interval
			PollingInterval: utils.EnvVarSeconds("NANIT_EVENTS_POLLING_INTERVAL", 30*time.Second),
			// 300 second (5 min) default message timeout (unseen messages are ignored once they are this old)
			MessageTimeout: utils.EnvVarSeconds("NANIT_EVENTS_MESSAGE_TIMEOUT", 300*time.Second),
		},
	}

	if utils.EnvVarBool("NANIT_RTMP_ENABLED", true) {
		publicAddr := utils.EnvVarReqStr("NANIT_RTMP_ADDR")
		addrM := regexp.MustCompile("(:[0-9]+)$").FindStringSubmatch(publicAddr)
		if len(addrM) != 2 {
			log.Fatal().Msg("Invalid NANIT_RTMP_ADDR. Unable to parse port.")
		}

		path := utils.EnvVarStr("NANIT_RTMP_PATH", "/local")
		pathM := regexp.MustCompile("^(/.+)$").FindStringSubmatch(path)
		if len(pathM) != 2 {
			log.Fatal().Msg("Invalid NANIT_RTMP_PATH. Unable to parse path.")
		}

		key := utils.EnvVarStr("NANIT_RTMP_KEY", "")
		keyM := regexp.MustCompile("^([a-zA-Z0-9]+)?$").FindStringSubmatch(key)
		if len(keyM) != 2 {
			log.Fatal().Msg("Invalid NANIT_RTMP_KEY. Unable to parse key.")
		}

		opts.RTMP = &app.RTMPOpts{
			ListenAddr: addrM[1],
			PublicAddr: publicAddr,
			Path:       path,
			Key:        key,
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

	if opts.EventPolling.Enabled {
		log.Info().Msgf("Event polling enabled with an interval of %v", opts.EventPolling.PollingInterval)
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
