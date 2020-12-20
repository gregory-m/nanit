package app

import (
	"strings"

	"github.com/rs/zerolog/log"
	"gitlab.com/adam.stanek/nanit/pkg/baby"
	"gitlab.com/adam.stanek/nanit/pkg/client"
	"gitlab.com/adam.stanek/nanit/pkg/mqtt"
)

// Run - application main loop
func Run(opts RunOpts) func() {
	interruptC := make(chan bool, 1)

	go func() {
		api := &client.NanitClient{
			Email:        opts.NanitCredentials.Email,
			Password:     opts.NanitCredentials.Password,
			SessionStore: opts.SessionStore,
		}

		// Reauthorize if we don't have a token or we assume it is invalid
		api.MaybeAuthorize(false)

		// Fetches babies info if they are not present in session
		api.EnsureBabies()

		// State manager
		stateManager := baby.NewStateManager()

		// MQTT
		var mqttConn *mqtt.Connection
		if opts.MQTT != nil {
			mqttConn = mqtt.NewConnection(*opts.MQTT)
			mqttConn.Start(stateManager)
		}

		babyClosers := make([]func(), len(opts.SessionStore.Session.Babies))

		// Start reading the data from the stream
		for i, baby := range opts.SessionStore.Session.Babies {
			babyClosers[i] = func() {
				log.Info().Str("babyuid", baby.UID).Msg("Closing baby")
			}

			// Remote stream processing
			if opts.StreamProcessor != nil {
				sp := NewStreamProcess(opts.StreamProcessor.CommandTemplate, baby.UID, opts.SessionStore.Session, api, opts.DataDirectories)
				sp.Start()

				prev := babyClosers[i]
				babyClosers[i] = func() {
					prev()
					sp.Stop()
				}
			}

			// Local stream
			localStreamURL := ""
			if opts.LocalStreaming != nil {
				r := strings.NewReplacer("{babyUid}", baby.UID)
				localStreamURL = r.Replace(opts.LocalStreaming.PushTargetURLTemplate)
			}

			// Websocket connection
			if opts.LocalStreaming != nil || mqttConn != nil {
				// Websocket connection
				ws := client.NewWebsocketConnection(baby.CameraUID, opts.SessionStore.Session, api)
				registerWebsocketHandlers(baby.UID, ws, localStreamURL, stateManager)
				ws.Start()

				prev := babyClosers[i]
				babyClosers[i] = func() {
					prev()
					ws.Stop()
				}
			}
		}

		// Start serving content over HTTP
		if opts.HTTPEnabled {
			go serve(opts.SessionStore.Session.Babies, opts.DataDirectories)
		}

		<-interruptC
	}()

	return func() {
		close(interruptC)
	}
}
