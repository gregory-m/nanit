package main

import (
	"time"

	"github.com/rs/zerolog/log"
)

func registerWebsocketHandlers(conn *WebsocketConnection) {
	// Get sensor data upon successful connection
	conn.OnReady(func(conn *WebsocketConnection) {
		conn.SendMessage(&Message{
			Type: Message_Type(Message_REQUEST).Enum(),
			Request: &Request{
				Id:   constRefInt32(1),
				Type: RequestType(RequestType_GET_SENSOR_DATA).Enum(),
				GetSensorData: &GetSensorData{
					All: constRefBool(true),
				},
			},
		})
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

	// Streaming request
	// &Message{
	// 	Type: Message_Type(Message_REQUEST).Enum(),
	// 	Request: &Request{
	// 		Id:   constRefInt32(2),
	// 		Type: RequestType(RequestType_PUT_STREAMING).Enum(),
	// 		Streaming: &Streaming{
	// 			Id:       StreamIdentifier(StreamIdentifier_MOBILE).Enum(),
	// 			RtmpUrl:  constRefStr("rtmp://192.168.3.234:1935/nanit/live"),
	// 			Status:   Streaming_Status(Streaming_STARTED).Enum(),
	// 			Attempts: constRefInt32(3),
	// 		},
	// 	},
	// }

	// For debugging: request logs from Cam
	// &Message{
	// 	Type: Message_Type(Message_REQUEST).Enum(),
	// 	Request: &Request{
	// 		Id:   constRefInt32(3),
	// 		Type: RequestType(RequestType_GET_LOGS).Enum(),
	// 		GetLogs: &GetLogs{
	// 			Url: constRefStr("http://192.168.3.234:8080/log"),
	// 		},
	// 	},
	// }
}
