package main

import (
	"time"

	"github.com/rs/zerolog/log"
)

func registerWebsocketHandlers(conn *WebsocketConnection, localStreamServer string) {

	// Send initial set of requests upon successful connection
	conn.OnReady(func(conn *WebsocketConnection) {
		// Ask for sensor data
		conn.SendRequest(RequestType_GET_SENSOR_DATA, Request{
			GetSensorData: &GetSensorData{
				All: constRefBool(true),
			},
		})

		// Push streaming URL
		if localStreamServer != "" {
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
	conn.OnTermination(func() { termC <- true })

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
}
