package main

import (
	"fmt"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/rs/zerolog/log"
)

// MQTTConnection MQTT context
type MQTTConnection struct {
	BrokerURL    string
	Username     string
	Password     string
	Attempter    *Attempter
	StateManager *StateManager
}

// NewMQTTConnection constructor
func NewMQTTConnection(url string, username string, password string) *MQTTConnection {
	return &MQTTConnection{
		BrokerURL: url,
		Username:  username,
		Password:  password,
	}
}

// Start runs the mqtt connection
func (conn *MQTTConnection) Start(manager *StateManager) {
	conn.StateManager = manager
	conn.Attempter = NewAttempter(
		func(attempt *Attempt) error {
			return runMqtt(conn, attempt)
		},
		[]time.Duration{
			2 * time.Second,
			10 * time.Second,
			1 * time.Minute,
		},
		2*time.Second,
	)

	go conn.Attempter.Run()
}

// Stop closes existing connection and stops attempting to reopen it
func (conn *MQTTConnection) Stop() {
	conn.Attempter.Stop()
}

func runMqtt(conn *MQTTConnection, attempt *Attempt) error {
	opts := MQTT.NewClientOptions()
	opts.AddBroker(conn.BrokerURL)
	opts.SetClientID("nanit")
	opts.SetUsername(conn.Username)
	opts.SetPassword(conn.Password)
	opts.SetCleanSession(false)

	client := MQTT.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Error().Str("broker_url", conn.BrokerURL).Err(token.Error()).Msg("Unable to connect to MQTT broker")
		return token.Error()
	}

	log.Info().Str("broker_url", conn.BrokerURL).Msg("Successfully connected to MQTT broker")

	unsubscribe := conn.StateManager.Subscribe(func(babyUID string, state BabyState) {
		if state.Temperature != nil {
			token := client.Publish(fmt.Sprintf("nanit/babies/%v/temperature", babyUID), 0, false, fmt.Sprintf("%v", float32(*state.Temperature)/1000))
			if token.Wait(); token.Error() != nil {
				log.Error().Msg("Unable to publish temperature update")
			}
		}

		if state.Humidity != nil {
			token := client.Publish(fmt.Sprintf("nanit/babies/%v/humidity", babyUID), 0, false, fmt.Sprintf("%v", float32(*state.Humidity)/1000))
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
