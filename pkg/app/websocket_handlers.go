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

func registerWebsocketHandlers(babyUID string, conn *client.WebsocketConnection, localStreamServer string, stateManager *baby.StateManager) {

	// Send initial set of requests upon successful connection
	conn.OnReady(func(conn *client.WebsocketConnection) {
		// Ask for sensor data (initial request)
		conn.SendRequest(client.RequestType_GET_SENSOR_DATA, &client.Request{
			GetSensorData: &client.GetSensorData{
				All: utils.ConstRefBool(true),
			},
		})

		// Push streaming URL
		if localStreamServer != "" {
			babyState := stateManager.GetBabyState(babyUID)

			if !babyState.GetLocalStreamingInitiated() {
				go requestLocalStreaming(babyUID, localStreamServer, conn, stateManager)
			}
		}

		// Ask for logs
		// conn.SendRequest(RequestType_GET_LOGS, Request{
		// 	GetLogs: &GetLogs{
		// 		Url: constRefStr("http://192.168.3.234:8080/log"),
		// 	},
		// })
	})

	// Keep-alive
	var ticker *time.Ticker

	conn.OnTermination(func() {
		if ticker != nil {
			ticker.Stop()
		}
	})

	conn.OnReady(func(conn *client.WebsocketConnection) {
		if ticker == nil {
			ticker = time.NewTicker(20 * time.Second)
		} else {
			ticker.Reset(20 * time.Second)
		}

		go func() {
			for {
				select {
				case <-ticker.C:
					conn.SendMessage(&client.Message{
						Type: client.Message_Type(client.Message_KEEPALIVE).Enum(),
					})
				}
			}
		}()
	})

	// Reading sensor data
	conn.OnMessage(func(m *client.Message, conn *client.WebsocketConnection) {
		// Sensor request initiated by us on start (or some other client, we don't care)
		if *m.Type == client.Message_RESPONSE && m.Response != nil {
			if *m.Response.RequestType == client.RequestType_GET_SENSOR_DATA && len(m.Response.SensorData) > 0 {
				processSensorData(babyUID, m.Response.SensorData, stateManager)
			}
		} else

		// Communication initiated from a cam
		// Note: it sends the updates periodically on its own + whenever some significant change occurs
		if *m.Type == client.Message_REQUEST && m.Request != nil {
			if *m.Request.Type == client.RequestType_PUT_SENSOR_DATA && len(m.Request.SensorData_) > 0 {
				processSensorData(babyUID, m.Request.SensorData_, stateManager)
			}
		}
	})
}
