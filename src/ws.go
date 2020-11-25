package main

import (
	"fmt"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/sacOO7/gowebsocket"
	"google.golang.org/protobuf/proto"
)

func constRefInt32(i int32) *int32 { return &i }
func constRefBool(b bool) *bool    { return &b }
func constRefStr(s string) *string { return &s }

func getMessageBytes(data *Message) []byte {
	out, err := proto.Marshal(data)
	if err != nil {
		log.Fatal().Err(err).Msg("Unable to marshal data")
	}

	return out
}

func sendMessage(socket gowebsocket.Socket, m *Message) {
	log.Debug().Stringer("data", m).Msg("Sending message")

	bytes := getMessageBytes(m)
	log.Trace().Bytes("rawdata", bytes).Msg("Sending data")

	socket.SendBinary(bytes)
}

func sendKeepAlive(socket gowebsocket.Socket) {
	if socket.IsConnected {
		log.Trace().Msg("Sending keep alive message")
		socket.SendBinary(getMessageBytes(&Message{
			Type: Message_Type(Message_KEEPALIVE).Enum(),
		}))
	}
}

func wsConnection(authToken string, cameraUID string) func() {
	URL := fmt.Sprintf("wss://api.nanit.com/focus/cameras/%v/user_connect", cameraUID)
	// URL := "wss://192.168.3.195:442"
	socket := gowebsocket.New(URL)

	// Fore remote connections
	socket.RequestHeader.Set("Authorization", "Bearer "+authToken)

	// For local connections
	// socket.RequestHeader.Set("Authorization", "token xxxxx")

	socket.OnConnected = func(socket gowebsocket.Socket) {
		log.Info().Str("url", URL).Msg("Connected to websocket")

		sendMessage(socket, &Message{
			Type: Message_Type(Message_REQUEST).Enum(),
			Request: &Request{
				Id:   constRefInt32(1),
				Type: RequestType(RequestType_GET_SENSOR_DATA).Enum(),
				GetSensorData: &GetSensorData{
					All: constRefBool(true),
				},
			},
		})
	}

	socket.OnBinaryMessage = func(data []byte, socket gowebsocket.Socket) {
		m := &Message{}
		err := proto.Unmarshal(data, m)
		if err != nil {
			log.Fatal().Err(err).Bytes("rawdata", data).Msg("Received malformed binary message")
			return
		}

		log.Debug().Stringer("data", m).Msg("Received message")

		// if m.GetType() == Message_REQUEST {
		// 	r := m.GetRequest()
		// 	if r {
		// 		if r.GetType == RequestType_GET_SENSOR_DATA {

		// 		}
		// 	}
		// }

		// if m.GetType() == Message_RESPONSE {
		// 	r := m.GetResponse()
		// 	if r {
		// 		if r.RequestId == 1
		// 	}
		// }
	}

	socket.OnDisconnected = func(err error, socket gowebsocket.Socket) {
		if err != nil {
			log.Error().Err(err).Msg("Disconnected from server")
		} else {
			log.Warn().Msg("Disconnected from server")
		}
	}

	socket.OnConnectError = func(err error, socket gowebsocket.Socket) {
		log.Fatal().Err(err).Msg("Unable to connect")
	}

	log.Info().Str("url", URL).Msg("Connecting")
	socket.Connect()

	// getLogTimer := time.NewTimer(10 * time.Second)

	keepAliveTicker := time.NewTicker(20 * time.Second)
	keepAliveInitialTimer := time.NewTimer(2 * time.Second)

	go func() {
		for {
			select {
			case <-keepAliveInitialTimer.C:
				if socket.IsConnected {
					// For debugging: request logs from Cam
					// sendMessage(socket, &Message{
					// 	Type: Message_Type(Message_REQUEST).Enum(),
					// 	Request: &Request{
					// 		Id:   constRefInt32(3),
					// 		Type: RequestType(RequestType_GET_LOGS).Enum(),
					// 		GetLogs: &GetLogs{
					// 			Url: constRefStr("http://192.168.3.234:8080/log"),
					// 		},
					// 	},
					// })

					// Send streaming request
					// sendMessage(socket, &Message{
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
					// })

					sendKeepAlive(socket)
				} else {
					keepAliveInitialTimer.Reset(1 * time.Second)
				}
			case <-keepAliveTicker.C:
				sendKeepAlive(socket)
			}

		}
	}()

	return func() {
		log.Info().Str("url", URL).Msg("Closing websocket connection")
		socket.Close()
		keepAliveInitialTimer.Stop()
		keepAliveTicker.Stop()
	}
}
