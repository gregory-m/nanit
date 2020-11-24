package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/sacOO7/gowebsocket"
	"google.golang.org/protobuf/proto"
	"gopkg.in/gookit/color.v1"
)

var red = color.FgRed.Render
var green = color.FgGreen.Render

func getMessageBytes(data *Message) []byte {
	out, err := proto.Marshal(data)
	if err != nil {
		log.Fatal(fmt.Errorf("Unable to marshal data: %v", err))
	}

	return out
}

func sendMessage(socket gowebsocket.Socket, m *Message) {
	log.Println(fmt.Sprintf("%v message: %v", red("Sending"), m))

	bytes := getMessageBytes(m)
	// log.Println(fmt.Sprintf("Sending data: %v\n", bytes))

	socket.SendBinary(bytes)
}

func sendKeepAlive(socket gowebsocket.Socket) {
	if socket.IsConnected {
		log.Println("Sending keep alive message")
		sendMessage(socket, &Message{
			Type: Message_KEEPALIVE,
		})
		// socket.SendBinary(getMessageBytes(&Message{
		// 	Type: Message_KEEPALIVE,
		// }))
	}
}

func wsConnection(authToken string, cameraUID string) {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	ticker := time.NewTicker(20 * time.Second)
	initialKeepAlive := time.NewTimer(2 * time.Second)
	getLogTimer := time.NewTimer(10 * time.Second)

	URL := fmt.Sprintf("wss://api.nanit.com/focus/cameras/%v/user_connect", cameraUID)
	// URL := "wss://192.168.3.195:442"
	socket := gowebsocket.New(URL)

	socket.RequestHeader.Set("Authorization", "Bearer "+authToken)
	// socket.RequestHeader.Set("Authorization", "token xxxxx")

	socket.OnConnected = func(socket gowebsocket.Socket) {
		log.Println("Connected to server")

		sendMessage(socket, &Message{
			Type: &Message_REQUEST,
			Request: &Request{
				Id:   1,
				Type: &RequestType_GET_SENSOR_DATA,
				GetSensorData: &GetSensorData{
					All: true,
				},
			},
		})
	}

	socket.OnBinaryMessage = func(data []byte, socket gowebsocket.Socket) {
		// log.Println("Recieved binary data ", data)

		m := &Message{}
		err := proto.Unmarshal(data, m)
		if err != nil {
			log.Println(fmt.Sprintf("Received malformed binary message: %v", data))
			return
		}

		log.Println(fmt.Sprintf("%v message: %v", green("Received"), m))

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
		log.Println("Disconnected from server ")
		return
	}

	socket.OnConnectError = func(err error, socket gowebsocket.Socket) {
		log.Println(fmt.Errorf("Unable to connect: %v", err))
	}

	log.Println("Connecting")
	socket.Connect()

	for {
		select {
		case <-initialKeepAlive.C:
			if socket.IsConnected {
				sendMessage(socket, &Message{
					Type: Message_REQUEST,
					Request: &Request{
						Id:   2,
						Type: RequestType_PUT_STREAMING,
						Streaming: &Streaming{
							Id:       StreamIdentifier_MOBILE,
							RtmpUrl:  "rtmp://192.168.3.234:1935/nanit/live",
							Status:   Streaming_STARTED,
							Attempts: 2,
						},
					},
				})

				// sendKeepAlive(socket)
			} else {
				initialKeepAlive.Reset(1 * time.Second)
			}
		case <-getLogTimer.C:
			sendMessage(socket, &Message{
				Type: Message_REQUEST,
				Request: &Request{
					Id:   3,
					Type: RequestType_GET_LOGS,
					GetLogs: &GetLogs{
						Url: "http://192.168.3.234:8080/log",
					},
				},
			})
		case <-ticker.C:
			sendKeepAlive(socket)
		case <-interrupt:
			log.Println("interrupt")
			ticker.Stop()
			initialKeepAlive.Stop()

			socket.Close()
			return
		}
	}
}
