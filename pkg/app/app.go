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

	// Watchdog
	if app.Opts.LocalStreaming != nil {
		app.startWatchDog(ctx)
	}

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
			utils.RunWithPerseverance(func(attempt utils.AttemptContext) {
				// Reauthorize if it is not a first try or we assume we don't have a valid token
				app.RestClient.MaybeAuthorize(attempt.GetTry() > 1)

				app.runStreamProcess(baby.UID, "hls-capture", app.Opts.StreamProcessor.CommandTemplate, attempt)
			}, childCtx, utils.PerseverenceOpts{
				RunnerID:       fmt.Sprintf("hls-capture-%v", baby.UID),
				ResetThreshold: 2 * time.Second,
				Cooldown: []time.Duration{
					2 * time.Second,
					30 * time.Second,
					2 * time.Minute,
					15 * time.Minute,
					1 * time.Hour,
				},
			})
		})
	}

	// Websocket connection
	if app.Opts.LocalStreaming != nil || app.MQTTConnection != nil {
		// Websocket connection
		ws := client.NewWebsocketConnectionManager(baby.CameraUID, app.SessionStore.Session, app.RestClient)

		ws.WithReadyConnection(func(conn *client.WebsocketConnection, childCtx utils.GracefulContext) {
			app.runWebsocket(baby.UID, conn, childCtx)
		})

		ctx.RunAsChild(func(childCtx utils.GracefulContext) {
			ws.RunWithinContext(childCtx)
		})
	}

	<-ctx.Done()
}

func (app *App) runStreamProcess(babyUID string, name string, commandTemplate string, attempt utils.GracefulContext) {
	sublog := log.With().Str("processor", name).Logger()

	logFilename := filepath.Join(app.Opts.DataDirectories.LogDir, fmt.Sprintf("%v-%v-%v.log", name, babyUID, time.Now().Format(time.RFC3339)))

	r := strings.NewReplacer("{remoteStreamUrl}", app.getRemoteStreamURL(babyUID), "{localStreamUrl}", app.getLocalStreamURL(babyUID), "{babyUid}", babyUID)
	cmdTokens := strings.Split(r.Replace(commandTemplate), " ")

	logFile, fileErr := os.Create(logFilename)
	if fileErr != nil {
		sublog.Fatal().Str("filename", logFilename).Err(fileErr).Msg("Unable to create log file")
	}

	defer logFile.Close()

	sublog.Info().Str("cmd", strings.Join(cmdTokens, " ")).Str("logfile", logFilename).Msg("Starting stream processor")

	cmd := exec.Command(cmdTokens[0], cmdTokens[1:]...)
	cmd.Stderr = logFile
	cmd.Stdout = logFile
	cmd.Dir = app.Opts.DataDirectories.VideoDir

	err := cmd.Start()
	if err != nil {
		sublog.Fatal().Err(err).Msg("Unable to start stream processor")
	}

	done := make(chan error, 1)

	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			sublog.Error().Err(err).Msg("Stream processor exited")
			attempt.Fail(err)
			return
		}

		sublog.Warn().Msg("Stream processor exited with status 0")
		attempt.Fail(errors.New("Stream processor exited with status 0"))
		return

	case <-attempt.Done():
		sublog.Info().Msg("Terminating stream processor")
		if err := cmd.Process.Kill(); err != nil {
			sublog.Error().Err(err).Msg("Unable to kill process")
		}
	}
}

func (app *App) runWebsocket(babyUID string, conn *client.WebsocketConnection, childCtx utils.GracefulContext) {
	// Reading sensor data
	conn.RegisterMessageHandler(func(m *client.Message, conn *client.WebsocketConnection) {
		// Sensor request initiated by us on start (or some other client, we don't care)
		if *m.Type == client.Message_RESPONSE && m.Response != nil {
			if *m.Response.RequestType == client.RequestType_GET_SENSOR_DATA && len(m.Response.SensorData) > 0 {
				processSensorData(babyUID, m.Response.SensorData, app.BabyStateManager)
			}
		} else

		// Communication initiated from a cam
		// Note: it sends the updates periodically on its own + whenever some significant change occurs
		if *m.Type == client.Message_REQUEST && m.Request != nil {
			if *m.Request.Type == client.RequestType_PUT_SENSOR_DATA && len(m.Request.SensorData_) > 0 {
				processSensorData(babyUID, m.Request.SensorData_, app.BabyStateManager)
			}
		}
	})

	// Ask for sensor data (initial request)
	conn.SendRequest(client.RequestType_GET_SENSOR_DATA, &client.Request{
		GetSensorData: &client.GetSensorData{
			All: utils.ConstRefBool(true),
		},
	})

	// Ask for logs
	// conn.SendRequest(client.RequestType_GET_LOGS, &client.Request{
	// 	GetLogs: &client.GetLogs{
	// 		Url: utils.ConstRefStr("http://192.168.3.234:8080/log"),
	// 	},
	// })

	initializeLocalStreaming := func() {
		requestLocalStreaming(babyUID, app.getLocalStreamURL(babyUID), conn, app.BabyStateManager)
	}

	// Watch for local streaming state change
	unsubscribe := app.BabyStateManager.Subscribe(func(updatedBabyUID string, stateUpdate baby.State) {
		if updatedBabyUID == babyUID && stateUpdate.LocalStreamingInitiated != nil && *stateUpdate.LocalStreamingInitiated == false {
			time.Sleep(1 * time.Second)
			go initializeLocalStreaming()
		}
	})

	// Initialize local streaming
	if app.Opts.LocalStreaming != nil {
		babyState := app.BabyStateManager.GetBabyState(babyUID)

		if !babyState.GetLocalStreamingInitiated() {
			go initializeLocalStreaming()
		}
	}

	<-childCtx.Done()
	unsubscribe()
}

func (app *App) startWatchDog(ctx utils.GracefulContext) {
	app.BabyStateManager.Subscribe(func(babyUID string, stateUpdate baby.State) {
		// If local streaming has been just initiated
		if stateUpdate.LocalStreamingInitiated != nil && *stateUpdate.LocalStreamingInitiated == true {
			ctx.RunAsChild(func(childCtx utils.GracefulContext) {
				// Give a chance to stream to properly initiate
				select {
				case <-time.After(5 * time.Second):
				case <-childCtx.Done():
					return
				}

				log.Info().Str("baby_uid", babyUID).Msg("Starting local stream watch dog")
				app.runStreamProcess(babyUID, "watchdog", "ffmpeg -i {localStreamUrl} -f null -", childCtx)
			}).Wait()

			log.Warn().Str("baby_uid", babyUID).Msg("Watchdog exited")
			streamingStoppedUpdate := baby.State{}
			streamingStoppedUpdate.SetLocalStreamingInitiated(false)
			app.BabyStateManager.Update(babyUID, streamingStoppedUpdate)
		}
	})
}

func (app *App) getRemoteStreamURL(babyUID string) string {
	return fmt.Sprintf("rtmps://media-secured.nanit.com/nanit/%v.%v", babyUID, app.SessionStore.Session.AuthToken)
}

func (app *App) getLocalStreamURL(babyUID string) string {
	if app.Opts.LocalStreaming != nil {
		tpl := app.Opts.LocalStreaming.PushTargetURLTemplate
		return strings.NewReplacer("{babyUid}", babyUID).Replace(tpl)
	}

	return ""
}
