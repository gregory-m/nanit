package mqtt

import (
	"fmt"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/rs/zerolog/log"
	"gitlab.com/adam.stanek/nanit/pkg/baby"
	"gitlab.com/adam.stanek/nanit/pkg/utils"
)

// Connection - MQTT context
type Connection struct {
	Opts         Opts
	StateManager *baby.StateManager
}

// NewConnection - constructor
func NewConnection(opts Opts) *Connection {
	return &Connection{
		Opts: opts,
	}
}

// Run - runs the mqtt connection handler
func (conn *Connection) Run(manager *baby.StateManager, ctx utils.GracefulContext) {
	conn.StateManager = manager

	utils.AttempterRunWithinContext(
		func(attempt *utils.Attempt) error {
			return runMqtt(conn, attempt)
		},
		[]time.Duration{
			2 * time.Second,
			10 * time.Second,
			1 * time.Minute,
		},
		2*time.Second,
		ctx,
	)
}

func runMqtt(conn *Connection, attempt *utils.Attempt) error {
	opts := MQTT.NewClientOptions()
	opts.AddBroker(conn.Opts.BrokerURL)
	opts.SetClientID("nanit")
	opts.SetUsername(conn.Opts.Username)
	opts.SetPassword(conn.Opts.Password)
	opts.SetCleanSession(false)

	client := MQTT.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Error().Str("broker_url", conn.Opts.BrokerURL).Err(token.Error()).Msg("Unable to connect to MQTT broker")
		return token.Error()
	}

	log.Info().Str("broker_url", conn.Opts.BrokerURL).Msg("Successfully connected to MQTT broker")

	unsubscribe := conn.StateManager.Subscribe(func(babyUID string, state baby.State) {
		if state.TemperatureMilli != nil {
			token := client.Publish(fmt.Sprintf("nanit/babies/%v/temperature", babyUID), 0, false, fmt.Sprintf("%v", state.GetTemperature()))
			if token.Wait(); token.Error() != nil {
				log.Error().Msg("Unable to publish temperature update")
			}
		}

		if state.HumidityMilli != nil {
			token := client.Publish(fmt.Sprintf("nanit/babies/%v/humidity", babyUID), 0, false, fmt.Sprintf("%v", state.GetHumidity()))
			if token.Wait(); token.Error() != nil {
				log.Error().Msg("Unable to publish humidity update")
			}
		}
	})

	// Wait until interrupt signal is received
	<-attempt.InterruptC

	log.Debug().Msg("Closing MQTT connection on interrupt")
	unsubscribe()
	client.Disconnect(250)
	return nil
}