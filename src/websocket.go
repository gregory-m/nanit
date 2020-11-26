package main

import (
	"fmt"
	sync "sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/sacOO7/gowebsocket"
	"google.golang.org/protobuf/proto"
)

type WebsocketConnection struct {
	CameraUID string
	Session   *AppSession
	API       *NanitClient
	Socket    gowebsocket.Socket
	Attempter *Attempter

	HandleReady       func(*WebsocketConnection)
	HandleTermination func()
	HandleMessage     func(*Message, *WebsocketConnection)
}

func NewWebsocketConnection(cameraUID string, session *AppSession, api *NanitClient) *WebsocketConnection {
	return &WebsocketConnection{
		CameraUID: cameraUID,
		Session:   session,
		API:       api,
	}
}

func (conn *WebsocketConnection) Start() {
	conn.Attempter = NewAttempter(
		func(attempt *Attempt) error {
			return runWebsocket(conn, attempt)
		},
		[]time.Duration{
			// 2 * time.Second,
			30 * time.Second,
			2 * time.Minute,
			15 * time.Minute,
			1 * time.Hour,
		},
		2*time.Second,
	)

	go conn.Attempter.Run()
}

func (conn *WebsocketConnection) Stop() {
	conn.Attempter.Stop()
}

func (conn *WebsocketConnection) OnReady(handler func(*WebsocketConnection)) {
	prev := conn.HandleReady
	if prev != nil {
		conn.HandleReady = func(conn *WebsocketConnection) {
			prev(conn)
			handler(conn)
		}
	} else {
		conn.HandleReady = handler
	}
}

func (conn *WebsocketConnection) OnTermination(handler func()) {
	prev := conn.HandleTermination
	if prev != nil {
		conn.HandleTermination = func() {
			prev()
			handler()
		}
	} else {
		conn.HandleTermination = handler
	}
}

func (conn *WebsocketConnection) OnMessage(handler func(*Message, *WebsocketConnection)) {
	prev := conn.HandleMessage
	if prev != nil {
		conn.HandleMessage = func(m *Message, conn *WebsocketConnection) {
			prev(m, conn)
			handler(m, conn)
		}
	} else {
		conn.HandleMessage = handler
	}
}

func (conn *WebsocketConnection) SendMessage(m *Message) {
	log.Debug().Stringer("data", m).Msg("Sending message")

	bytes := getMessageBytes(m)
	log.Trace().Bytes("rawdata", bytes).Msg("Sending data")

	conn.Socket.SendBinary(bytes)
}

func runWebsocket(conn *WebsocketConnection, attempt *Attempt) error {
	// Reauthorize if it is not a first try or if the session is older then 10 minutes
	if attempt.Number > 1 || time.Since(conn.Session.AuthTime) > 10*time.Minute {
		conn.API.Authorize()
	}

	// Remote
	url := fmt.Sprintf("wss://api.nanit.com/focus/cameras/%v/user_connect", conn.CameraUID)
	auth := fmt.Sprintf("Bearer %v", conn.Session.AuthToken)

	// Local
	// url := "wss://192.168.3.195:442"
	// auth := fmt.Sprintf("token %v", userCamToken)

	// -------

	terminationC := make(chan error, 1)
	var once sync.Once // Just because gowebsocket is buggy and can invoke OnDisconnect multiple times :-/

	conn.Socket = gowebsocket.New(url)
	conn.Socket.RequestHeader.Set("Authorization", auth)

	// Handle new connection
	conn.Socket.OnConnected = func(socket gowebsocket.Socket) {
		log.Info().Str("url", url).Msg("Connected to websocket")
		if conn.HandleReady != nil {
			conn.HandleReady(conn)
		}
	}

	// Handle failed attempts for connection
	conn.Socket.OnConnectError = func(err error, socket gowebsocket.Socket) {
		log.Error().Err(err).Msg("Unable to establish websocket connection")
		once.Do(func() { terminationC <- err })
		close(terminationC)
	}

	// Handle lost connection
	conn.Socket.OnDisconnected = func(err error, socket gowebsocket.Socket) {
		once.Do(func() {
			if err != nil {
				log.Error().Err(err).Msg("Disconnected from server")
				terminationC <- err
			} else {
				log.Warn().Msg("Disconnected from server")
				terminationC <- nil
			}
		})
	}

	// Handle messages
	conn.Socket.OnBinaryMessage = func(data []byte, socket gowebsocket.Socket) {
		m := &Message{}
		err := proto.Unmarshal(data, m)
		if err != nil {
			log.Error().Err(err).Bytes("rawdata", data).Msg("Received malformed binary message")
			return
		}

		log.Debug().Stringer("data", m).Msg("Received message")
		if conn.HandleMessage != nil {
			conn.HandleMessage(m, conn)
		}
	}

	log.Info().Str("url", url).Msg("Connecting to websocket")
	conn.Socket.Connect()

	for {
		select {
		case err := <-terminationC:
			if conn.HandleTermination != nil {
				conn.HandleTermination()
			}
			return err
		case <-attempt.InterruptC:
			log.Debug().Msg("Closing websocket on interrupt")
			if conn.HandleTermination != nil {
				conn.HandleTermination()
			}
			conn.Socket.Close()
			return nil
		}
	}
}

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
