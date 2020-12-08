package main

import (
	"time"

	"github.com/rs/zerolog/log"
)

type SensorInfoPayload struct {
	Value      int32
	ValueMilli int32
	IsAlert    bool
}

type SensorUpdate struct {
	Temperature *SensorInfoPayload
	Humidity    *SensorInfoPayload
}

func processSensorData(sensorData []*SensorData) {
	// Parse sensor update
	sensorUpdate := &SensorUpdate{}
	for _, sensorDataSet := range sensorData {
		if *sensorDataSet.SensorType == SensorType_TEMPERATURE {
			sensorUpdate.Temperature = &SensorInfoPayload{
				Value:      *sensorDataSet.Value,
				ValueMilli: *sensorDataSet.ValueMilli,
				IsAlert:    *sensorDataSet.IsAlert,
			}
		} else if *sensorDataSet.SensorType == SensorType_HUMIDITY {
			sensorUpdate.Humidity = &SensorInfoPayload{
				Value:      *sensorDataSet.Value,
				ValueMilli: *sensorDataSet.ValueMilli,
				IsAlert:    *sensorDataSet.IsAlert,
			}
		}
	}

	if sensorUpdate.Humidity != nil || sensorUpdate.Temperature != nil {
		msg := log.Debug()

		if sensorUpdate.Temperature != nil {
			msg.Float32("temperature", float32(sensorUpdate.Temperature.ValueMilli)/1000)
		}

		if sensorUpdate.Humidity != nil {
			msg.Float32("humidity", float32(sensorUpdate.Humidity.ValueMilli)/1000)
		}

		msg.Msg("Received sensor data update")
	}
}

func registerWebsocketHandlers(conn *WebsocketConnection, localStreamServer string) {

	// Send initial set of requests upon successful connection
	conn.OnReady(func(conn *WebsocketConnection) {
		// Ask for sensor data (initial request)
		conn.SendRequest(RequestType_GET_SENSOR_DATA, Request{
			GetSensorData: &GetSensorData{
				All: constRefBool(true),
			},
		})

		// Push streaming URL
		if localStreamServer != "" {
			log.Info().Str("target", localStreamServer).Msg("Requesting local streaming")

			conn.SendRequest(RequestType_PUT_STREAMING, Request{
				Streaming: &Streaming{
					Id:       StreamIdentifier(StreamIdentifier_MOBILE).Enum(),
					RtmpUrl:  constRefStr(localStreamServer),
					Status:   Streaming_Status(Streaming_STARTED).Enum(),
					Attempts: constRefInt32(3),
				},
			})
		}

		// Ask for logs
		// conn.SendRequest(RequestType_GET_LOGS, Request{
		// 	GetLogs: &GetLogs{
		// 		Url: constRefStr("http://192.168.3.234:8080/log"),
		// 	},
		// })
	})

	// Listen for termination
	termC := make(chan bool, 1)
	conn.OnTermination(func() {
		// Closing the channel will broadcast to all the listeners, regardless to their number
		close(termC)
	})

	// Keep-alive
	conn.OnReady(func(conn *WebsocketConnection) {
		go func() {
			ticker := time.NewTicker(20 * time.Second)
			for {
				select {
				case <-termC:
					log.Trace().Msg("Canceling keep-alive ticker")
					ticker.Stop()
					return
				case <-ticker.C:
					conn.SendMessage(&Message{
						Type: Message_Type(Message_KEEPALIVE).Enum(),
					})
				}
			}
		}()
	})

	// Reading sensor data
	conn.OnMessage(func(m *Message, conn *WebsocketConnection) {
		// Sensor request initiated by us on start (or some other client, we don't care)
		if *m.Type == Message_RESPONSE && m.Response != nil {
			if *m.Response.RequestType == RequestType_GET_SENSOR_DATA && len(m.Response.SensorData) > 0 {
				processSensorData(m.Response.SensorData)
			}
		} else

		// Communication initiated from a cam
		// Note: it sends the updates periodically on its own + whenever some significant change occurs
		if *m.Type == Message_REQUEST && m.Request != nil {
			if *m.Request.Type == RequestType_PUT_SENSOR_DATA && len(m.Request.SensorData_) > 0 {
				processSensorData(m.Request.SensorData_)
			}
		}
	})
}
