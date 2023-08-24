package ws

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/mendersoftware/go-lib-micro/ws"
	"github.com/northerntechhq/nt-connect/api"
	"github.com/vmihailenco/msgpack/v5"
)

type socket struct {
	msgChan chan ws.ProtoMsg
	errChan chan error
	done    chan struct{}
	mu      sync.Mutex
	conn    *websocket.Conn
}

func (sock *socket) ReceiveChan() <-chan ws.ProtoMsg {
	return sock.msgChan
}
func (sock *socket) ErrorChan() <-chan error {
	return sock.errChan
}

var ErrClosed = errors.New("closed")

func (sock *socket) Send(msg ws.ProtoMsg) error {
	var (
		err error
		b   []byte
	)
	select {
	case <-sock.done:
		return ErrClosed

	default:
		b, err = msgpack.Marshal(msg)
		if err != nil {
			return err
		}
		err = sock.conn.WriteMessage(websocket.BinaryMessage, b)
	}
	return err
}

func (sock *socket) term() bool {
	sock.mu.Lock()
	defer sock.mu.Unlock()
	select {
	case <-sock.done:
		return true
	default:
		close(sock.done)
	}
	return false
}

func (sock *socket) Close() error {
	if !sock.term() {
		return sock.conn.Close()
	}
	return nil
}

func (sock *socket) pushError(err error) {
	select {
	case sock.errChan <- err:
	default:
	}
}

func newSocket(conn *websocket.Conn) *socket {
	return &socket{
		msgChan: make(chan ws.ProtoMsg),
		errChan: make(chan error, 1),
		conn:    conn,
	}
}

func Connect(ctx context.Context, authz *api.Authz) (api.Socket, error) {
	const APIURLConnect = "/api/devices/v1/deviceconnect/connect"
	url := strings.TrimRight(authz.ServerURL, "/") + APIURLConnect
	url = strings.Replace(url, "http", "ws", 1)
	conn, rsp, err := websocket.DefaultDialer.DialContext(
		ctx, url, http.Header{
			"Authorization": []string{"Bearer " + authz.Token},
		},
	)
	if err != nil {
		return nil, err
	}
	if rsp.StatusCode >= 300 {
		if rsp.StatusCode == 401 {
			return nil, api.ErrUnauthorized
		}
	}
	sock := newSocket(conn)
	// receive pipe
	go func() {
		defer sock.Close()
		defer close(sock.msgChan)
		for {
			var msg ws.ProtoMsg
			_, r, err := conn.NextReader()
			if err != nil {
				sock.pushError(err)
				return
			}
			err = msgpack.NewDecoder(r).
				Decode(&msg)
			if err != nil {
				sock.pushError(err)
			}
			select {
			case <-sock.done:
				return
			case sock.msgChan <- msg:
			}
		}
	}()
	return sock, nil
}