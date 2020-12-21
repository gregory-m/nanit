package app

import (
	"time"

	"github.com/rs/zerolog/log"
	"gitlab.com/adam.stanek/nanit/pkg/baby"
	"gitlab.com/adam.stanek/nanit/pkg/client"
	"gitlab.com/adam.stanek/nanit/pkg/utils"
)

func processSensorData(babyUID string, sensorData []*client.SensorData, stateManager *baby.StateManager) {
	// Parse sensor update
	stateUpdate := baby.State{}
	for _, sensorDataSet := range sensorData {
		if *sensorDataSet.SensorType == client.SensorType_TEMPERATURE {
			stateUpdate.SetTemperatureMilli(*sensorDataSet.ValueMilli)
		} else if *sensorDataSet.SensorType == client.SensorType_HUMIDITY {
			stateUpdate.SetHumidityMilli(*sensorDataSet.ValueMilli)
		} else if *sensorDataSet.SensorType == client.SensorType_NIGHT {
			stateUpdate.SetIsNight(*sensorDataSet.Value == 1)
		}
	}

	stateManager.Update(babyUID, stateUpdate)
}

func requestLocalStreaming(babyUID string, targetURL string, conn *client.WebsocketConnection, stateManager *baby.StateManager) {
	log.Info().Str("target", targetURL).Msg("Requesting local streaming")

	awaitResponse := conn.SendRequest(client.RequestType_PUT_STREAMING, &client.Request{
		Streaming: &client.Streaming{
			Id:       client.StreamIdentifier(client.StreamIdentifier_MOBILE).Enum(),
			RtmpUrl:  utils.ConstRefStr(targetURL),
			Status:   client.Streaming_Status(client.Streaming_STARTED).Enum(),
			Attempts: utils.ConstRefInt32(3),
		},
	})

	_, err := awaitResponse(30 * time.Second)

	stateUpdate := baby.State{}
	if err != nil {
		log.Error().Err(err).Msg("Failed to request local streaming")
		stateUpdate.SetLocalStreamingInitiated(false)
	} else {
		log.Info().Msg("Local streaming successfully requested")
		stateUpdate.SetLocalStreamingInitiated(true)
	}

	stateManager.Update(babyUID, stateUpdate)
}
