package client

import (
	"errors"
	"fmt"
	sync "sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/sacOO7/gowebsocket"
	"gitlab.com/adam.stanek/nanit/pkg/session"
	"gitlab.com/adam.stanek/nanit/pkg/utils"
	"google.golang.org/protobuf/proto"
)

type unhandledRequest struct {
	Request        *Request
	HandleResponse func(response *Response)
}

// WebsocketConnection - connection container
type WebsocketConnection struct {
	CameraUID     string
	Session       *session.Session
	API           *NanitClient
	Socket        gowebsocket.Socket
	Attempter     *utils.Attempter
	LastRequestID int32

	UnhandledRequests     map[int32]unhandledRequest
	UnhandledRequestsLock sync.Mutex

	HandleReady       func(*WebsocketConnection)
	HandleTermination func()
	HandleMessage     func(*Message, *WebsocketConnection)
}

// NewWebsocketConnection - constructor
func NewWebsocketConnection(cameraUID string, session *session.Session, api *NanitClient) *WebsocketConnection {
	return &WebsocketConnection{
		CameraUID:         cameraUID,
		Session:           session,
		API:               api,
		LastRequestID:     0,
		UnhandledRequests: make(map[int32]unhandledRequest),
		HandleMessage: func(m *Message, conn *WebsocketConnection) {
			if *m.Type == Message_RESPONSE && m.Response != nil {
				conn.UnhandledRequestsLock.Lock()
				unhandled, ok := conn.UnhandledRequests[*m.Response.RequestId]
				conn.UnhandledRequestsLock.Unlock()

				if ok && *m.Response.RequestType == *unhandled.Request.Type {
					unhandled.HandleResponse(m.Response)
				}
			}
		},
	}
}

// RunWithinContext - starts websocket connection attempt loop
func (conn *WebsocketConnection) RunWithinContext(ctx utils.GracefulContext) {
	utils.AttempterRunWithinContext(
		func(attempt *utils.Attempt) error {
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
		ctx,
	)
}

// OnReady - registers handler which will be called upon successfully established connection
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

// OnTermination - registers handler which will be called whenever the connection gets terminated
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

// OnMessage - registers handler which will be called upon incoming message
func (conn *WebsocketConnection) OnMessage(handler func(*Message, *WebsocketConnection)) {
	prev := conn.HandleMessage
	conn.HandleMessage = func(m *Message, conn *WebsocketConnection) {
		prev(m, conn)
		handler(m, conn)
	}
}

// SendMessage - low-level helper for sending raw message
// Note: Use SendRequest() for requests
func (conn *WebsocketConnection) SendMessage(m *Message) {
	var msg *zerolog.Event

	if *m.Type == Message_KEEPALIVE {
		msg = log.Trace()
	} else {
		msg = log.Debug()
	}

	msg.Stringer("data", m).Msg("Sending message")

	bytes := getMessageBytes(m)
	log.Trace().Bytes("rawdata", bytes).Msg("Sending data")

	conn.Socket.SendBinary(bytes)
}

// SendRequest - sends request to the cam and returns await function. Await function waits for the response and returns it
func (conn *WebsocketConnection) SendRequest(reqType RequestType, requestData *Request) func(time.Duration) (*Response, error) {
	id := atomic.AddInt32(&conn.LastRequestID, 1)

	requestData.Id = utils.ConstRefInt32(id)
	requestData.Type = RequestType(reqType).Enum()

	m := &Message{
		Type:    Message_Type(Message_REQUEST).Enum(),
		Request: requestData,
	}

	conn.SendMessage(m)

	return func(timeout time.Duration) (*Response, error) {
		resC := make(chan *Response, 1)

		defer func() {
			conn.UnhandledRequestsLock.Lock()
			delete(conn.UnhandledRequests, id)
			conn.UnhandledRequestsLock.Unlock()
		}()

		conn.UnhandledRequestsLock.Lock()
		conn.UnhandledRequests[id] = unhandledRequest{
			Request: m.Request,
			HandleResponse: func(res *Response) {
				resC <- res
			},
		}
		conn.UnhandledRequestsLock.Unlock()

		timer := time.NewTimer(timeout)

		select {
		case <-timer.C:
			close(resC)
			return nil, errors.New("Request timeout")
		case res := <-resC:
			timer.Stop()
			close(resC)

			if res.StatusCode == nil {
				return res, errors.New("No status code received")
			} else if *res.StatusCode != 200 {
				if res.GetStatusMessage() != "" {
					return res, errors.New(res.GetStatusMessage())
				}

				return res, fmt.Errorf("Unexpected status code %v", *res.StatusCode)
			}

			return res, nil
		}
	}
}

func runWebsocket(conn *WebsocketConnection, attempt *utils.Attempt) error {
	// Reauthorize if it is not a first try or we assume we don't have a valid token
	conn.API.MaybeAuthorize(attempt.Number > 1)

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
			go conn.HandleReady(conn)
		}
	}

	// Handle failed attempts for connection
	conn.Socket.OnConnectError = func(err error, socket gowebsocket.Socket) {
		log.Error().Str("url", url).Err(err).Msg("Unable to establish websocket connection")
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

	log.Trace().Msg("Connecting to websocket")
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

func getMessageBytes(data *Message) []byte {
	out, err := proto.Marshal(data)
	if err != nil {
		log.Fatal().Err(err).Msg("Unable to marshal data")
	}

	return out
}
