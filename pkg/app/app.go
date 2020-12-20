package app

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"gitlab.com/adam.stanek/nanit/pkg/baby"
	"gitlab.com/adam.stanek/nanit/pkg/client"
	"gitlab.com/adam.stanek/nanit/pkg/mqtt"
	"gitlab.com/adam.stanek/nanit/pkg/session"
	"gitlab.com/adam.stanek/nanit/pkg/utils"
)

// App - application container
type App struct {
	Opts             Opts
	SessionStore     *session.Store
	BabyStateManager *baby.StateManager
	RestClient       *client.NanitClient
	MQTTConnection   *mqtt.Connection
}

// NewApp - constructor
func NewApp(opts Opts) *App {
	sessionStore := session.InitSessionStore(opts.SessionFile)

	instance := &App{
		Opts:             opts,
		BabyStateManager: baby.NewStateManager(),
		SessionStore:     sessionStore,
		RestClient: &client.NanitClient{
			Email:        opts.NanitCredentials.Email,
			Password:     opts.NanitCredentials.Password,
			SessionStore: sessionStore,
		},
	}

	if opts.MQTT != nil {
		instance.MQTTConnection = mqtt.NewConnection(*opts.MQTT)
	}

	return instance
}

// Run - application main loop
func (app *App) Run(ctx utils.GracefulContext) {
	// Reauthorize if we don't have a token or we assume it is invalid
	app.RestClient.MaybeAuthorize(false)

	// Fetches babies info if they are not present in session
	app.RestClient.EnsureBabies()

	// MQTT
	if app.MQTTConnection != nil {
		ctx.RunAsChild(func(childCtx utils.GracefulContext) {
			app.MQTTConnection.Run(app.BabyStateManager, childCtx)
		})
	}

	// Start reading the data from the stream
	for _, babyInfo := range app.SessionStore.Session.Babies {
		ctx.RunAsChild(func(childCtx utils.GracefulContext) {
			app.handleBaby(babyInfo, childCtx)
		})
	}

	// Start serving content over HTTP
	if app.Opts.HTTPEnabled {
		go serve(app.SessionStore.Session.Babies, app.Opts.DataDirectories)
	}

	<-ctx.Done()
}

func (app *App) handleBaby(baby baby.Baby, ctx utils.GracefulContext) {
	// Remote stream processing
	if app.Opts.StreamProcessor != nil {
		ctx.RunAsChild(func(childCtx utils.GracefulContext) {
			utils.AttempterRunWithinContext(
				func(attempt *utils.Attempt) error {
					return app.runStreamProcess(baby, attempt)
				},
				[]time.Duration{
					2 * time.Second,
					30 * time.Second,
					2 * time.Minute,
					15 * time.Minute,
					1 * time.Hour,
				},
				2*time.Second,
				ctx,
			)
		})
	}

	// Local stream
	localStreamURL := ""
	if app.Opts.LocalStreaming != nil {
		r := strings.NewReplacer("{babyUid}", baby.UID)
		localStreamURL = r.Replace(app.Opts.LocalStreaming.PushTargetURLTemplate)
	}

	// Websocket connection
	if app.Opts.LocalStreaming != nil || app.MQTTConnection != nil {
		// Websocket connection
		ws := client.NewWebsocketConnection(baby.CameraUID, app.SessionStore.Session, app.RestClient)
		registerWebsocketHandlers(baby.UID, ws, localStreamURL, app.BabyStateManager)
		ctx.RunAsChild(func(childCtx utils.GracefulContext) {
			ws.RunWithinContext(childCtx)
		})
	}

	<-ctx.Done()
}

func (app *App) runStreamProcess(baby baby.Baby, attempt *utils.Attempt) error {
	// Reauthorize if it is not a first try or we assume we don't have a valid token
	app.RestClient.MaybeAuthorize(attempt.Number > 1)

	logFilename := filepath.Join(app.Opts.DataDirectories.LogDir, fmt.Sprintf("process-%v-%v.log", baby.UID, time.Now().Format(time.RFC3339)))
	url := fmt.Sprintf("rtmps://media-secured.nanit.com/nanit/%v.%v", baby.UID, app.SessionStore.Session.AuthToken)

	r := strings.NewReplacer("{sourceUrl}", url, "{babyUid}", baby.UID)
	cmdTokens := strings.Split(r.Replace(app.Opts.StreamProcessor.CommandTemplate), " ")

	logFile, fileErr := os.Create(logFilename)
	if fileErr != nil {
		log.Fatal().Str("filename", logFilename).Err(fileErr).Msg("Unable to create log file")
	}

	defer logFile.Close()

	log.Info().Str("cmd", strings.Join(cmdTokens, " ")).Str("logfile", logFilename).Msg("Starting stream processor")

	cmd := exec.Command(cmdTokens[0], cmdTokens[1:]...)
	cmd.Stderr = logFile
	cmd.Stdout = logFile
	cmd.Dir = app.Opts.DataDirectories.VideoDir

	err := cmd.Start()
	if err != nil {
		log.Fatal().Err(err).Msg("Unable to start stream processor")
	}

	done := make(chan error, 1)

	go func() {
		done <- cmd.Wait()
	}()

	for {
		select {
		case err := <-done:
			if err != nil {
				log.Error().Err(err).Msg("Stream processor exited")
				return err
			}

			log.Warn().Msg("Stream processor exited with status 0")
			return errors.New("Stream processor exited with status 0")

		case <-attempt.InterruptC:
			log.Info().Msg("Terminating stream processor")
			if err := cmd.Process.Kill(); err != nil {
				log.Error().Err(err).Msg("Unable to kill process")
			}

			return nil
		}
	}
}
