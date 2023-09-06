// Copyright 2023 Northern.tech AS
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//        http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

package session

import (
	"context"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/mendersoftware/go-lib-micro/ws"
	wspf "github.com/mendersoftware/go-lib-micro/ws/portforward"
	"github.com/northerntechhq/nt-connect/api"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/vmihailenco/msgpack/v5"
)

const (
	portForwardBuffSize          = 4096
	portForwardConnectionTimeout = time.Second * 600
)

var (
	errPortForwardInvalidMessage = errors.New(
		"invalid port-forward message: missing connection_id, remote_port or protocol",
	)
	errPortForwardUnkonwnMessageType = errors.New("unknown message type")
	errPortForwardUnkonwnConnection  = errors.New("unknown connection")
)

type PortForwarder struct {
	SessionID      string
	ConnectionID   string
	Sender         api.Sender
	conn           net.Conn
	closed         bool
	ctx            context.Context
	ctxCancel      context.CancelFunc
	mutexAck       *sync.Mutex
	portForwarders map[string]*PortForwarder
}

func (f *PortForwarder) Connect(protocol string, host string, portNumber uint16) error {
	log.Debugf(
		"port-forward[%s/%s] connect: %s/%s:%d",
		f.SessionID,
		f.ConnectionID,
		protocol,
		host,
		portNumber,
	)

	if protocol == wspf.PortForwardProtocolTCP || protocol == wspf.PortForwardProtocolUDP {
		conn, err := net.Dial(protocol, host+":"+strconv.Itoa(int(portNumber)))
		if err != nil {
			return err
		}
		f.conn = conn
	} else {
		return errors.New("unknown protocol: " + protocol)
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	f.ctx = ctx
	f.ctxCancel = cancelFunc

	go f.Read()

	return nil
}

func (f *PortForwarder) Close(sendStopMessage bool) error {
	if f.closed {
		return nil
	}
	f.closed = true
	log.Debugf("port-forward[%s/%s] close", f.SessionID, f.ConnectionID)
	if sendStopMessage {
		m := ws.ProtoMsg{
			Header: ws.ProtoHdr{
				Proto:     ws.ProtoTypePortForward,
				MsgType:   wspf.MessageTypePortForwardStop,
				SessionID: f.SessionID,
				Properties: map[string]interface{}{
					wspf.PropertyConnectionID: f.ConnectionID,
				},
			},
		}
		if err := f.Sender.Send(m); err != nil {
			log.Errorf("portForwardHandler: webSock.WriteMessage(%+v)", err)
		}
	}
	defer delete(f.portForwarders, f.ConnectionID)
	f.ctxCancel()
	return f.conn.Close()
}

func (f *PortForwarder) Read() {
	errChan := make(chan error)
	dataChan := make(chan []byte)

	go func() {
		data := make([]byte, portForwardBuffSize)

		for {
			n, err := f.conn.Read(data)
			if err != nil {
				errChan <- err
				break
			}
			if n > 0 {
				tmp := make([]byte, n)
				copy(tmp, data[:n])
				dataChan <- tmp
			}
		}
	}()

	for {
		select {
		case err := <-errChan:
			if err != io.EOF {
				log.Errorf(
					"port-forward[%s/%s] error: %v\n",
					f.SessionID,
					f.ConnectionID,
					err.Error(),
				)
			}
			f.Close(true)
		case data := <-dataChan:
			log.Debugf("port-forward[%s/%s] read %d bytes", f.SessionID, f.ConnectionID, len(data))

			// lock the ack mutex, we don't allow more than one in-flight message
			f.mutexAck.Lock()

			m := ws.ProtoMsg{
				Header: ws.ProtoHdr{
					Proto:     ws.ProtoTypePortForward,
					MsgType:   wspf.MessageTypePortForward,
					SessionID: f.SessionID,
					Properties: map[string]interface{}{
						wspf.PropertyConnectionID: f.ConnectionID,
					},
				},
				Body: data,
			}
			if err := f.Sender.Send(m); err != nil {
				log.Errorf("portForwardHandler: webSock.WriteMessage(%+v)", err)
			}
		case <-time.After(portForwardConnectionTimeout):
			f.Close(true)
		case <-f.ctx.Done():
			return
		}
	}
}

func (f *PortForwarder) Write(body []byte) error {
	log.Debugf("port-forward[%s/%s] write %d bytes", f.SessionID, f.ConnectionID, len(body))
	_, err := f.conn.Write(body)
	if err != nil {
		return err
	}
	return nil
}

type PortForwardHandler struct {
	portForwarders map[string]*PortForwarder
}

func PortForward() Constructor {
	return func() SessionHandler {
		return &PortForwardHandler{
			portForwarders: make(map[string]*PortForwarder),
		}
	}
}

func (h *PortForwardHandler) Close() error {
	for _, f := range h.portForwarders {
		f.Close(false)
	}
	return nil
}

func (h *PortForwardHandler) ServeProtoMsg(msg *ws.ProtoMsg, w api.Sender) {
	var err error
	switch msg.Header.MsgType {
	case wspf.MessageTypePortForwardNew:
		err = h.portForwardHandlerNew(msg, w)
	case wspf.MessageTypePortForwardStop:
		err = h.portForwardHandlerStop(msg, w)
	case wspf.MessageTypePortForward:
		err = h.portForwardHandlerForward(msg, w)
	case wspf.MessageTypePortForwardAck:
		err = h.portForwardHandlerAck(msg, w)
	default:
		err = errPortForwardUnkonwnMessageType
	}
	if err != nil {
		log.Errorf("portForwardHandler(%+v)", err)

		errMessage := err.Error()
		body, err := msgpack.Marshal(&wspf.Error{
			Error:       &errMessage,
			MessageType: &msg.Header.MsgType,
		})
		if err != nil {
			log.Errorf("portForwardHandler: msgpack.Marshal(%+v)", err)
		}
		response := ws.ProtoMsg{
			Header: ws.ProtoHdr{
				Proto:     ws.ProtoTypePortForward,
				MsgType:   wspf.MessageTypeError,
				SessionID: msg.Header.SessionID,
			},
			Body: body,
		}
		if err := w.Send(response); err != nil {
			log.Errorf("portForwardHandler: webSock.WriteMessage(%+v)", err)
		}
	}
}

func (h *PortForwardHandler) portForwardHandlerNew(message *ws.ProtoMsg, w api.Sender) error {
	req := &wspf.PortForwardNew{}
	err := msgpack.Unmarshal(message.Body, req)
	if err != nil {
		return err
	}

	protocol := req.Protocol
	host := req.RemoteHost
	portNumber := req.RemotePort
	connectionID, _ := message.Header.Properties[wspf.PropertyConnectionID].(string)

	if protocol == nil || *protocol == "" || host == nil || *host == "" || portNumber == nil ||
		*portNumber == 0 ||
		connectionID == "" {
		return errPortForwardInvalidMessage
	}

	portForwarder := &PortForwarder{
		SessionID:      message.Header.SessionID,
		ConnectionID:   connectionID,
		Sender:         w,
		mutexAck:       &sync.Mutex{},
		portForwarders: h.portForwarders,
	}

	h.portForwarders[connectionID] = portForwarder

	log.Infof(
		"port-forward: new %s/%s: %s/%s:%d",
		message.Header.SessionID,
		connectionID,
		*protocol,
		*host,
		*portNumber,
	)
	err = portForwarder.Connect(string(*protocol), *host, *portNumber)
	if err != nil {
		delete(h.portForwarders, connectionID)
		return err
	}

	response := ws.ProtoMsg{
		Header: ws.ProtoHdr{
			Proto:     message.Header.Proto,
			MsgType:   message.Header.MsgType,
			SessionID: message.Header.SessionID,
			Properties: map[string]interface{}{
				wspf.PropertyConnectionID: connectionID,
			},
		},
	}
	if err := w.Send(response); err != nil {
		log.Errorf("portForwardHandler: webSock.WriteMessage(%+v)", err)
	}

	return nil
}

func (h *PortForwardHandler) portForwardHandlerStop(message *ws.ProtoMsg, w api.Sender) error {
	connectionID, _ := message.Header.Properties[wspf.PropertyConnectionID].(string)
	if portForwarder, ok := h.portForwarders[connectionID]; ok {
		log.Infof("port-forward: stop %s/%s", message.Header.SessionID, connectionID)
		defer delete(h.portForwarders, connectionID)
		if err := portForwarder.Close(false); err != nil {
			return err
		}

		response := ws.ProtoMsg{
			Header: ws.ProtoHdr{
				Proto:     message.Header.Proto,
				MsgType:   message.Header.MsgType,
				SessionID: message.Header.SessionID,
				Properties: map[string]interface{}{
					wspf.PropertyConnectionID: connectionID,
				},
			},
		}
		if err := w.Send(response); err != nil {
			log.Errorf("portForwardHandler: webSock.WriteMessage(%+v)", err)
		}

		return nil
	} else {
		return errPortForwardUnkonwnConnection
	}
}

func (h *PortForwardHandler) portForwardHandlerForward(
	message *ws.ProtoMsg,
	w api.Sender,
) error {
	connectionID, _ := message.Header.Properties[wspf.PropertyConnectionID].(string)
	if portForwarder, ok := h.portForwarders[connectionID]; ok {
		err := portForwarder.Write(message.Body)
		// send ack
		response := ws.ProtoMsg{
			Header: ws.ProtoHdr{
				Proto:     message.Header.Proto,
				MsgType:   wspf.MessageTypePortForwardAck,
				SessionID: message.Header.SessionID,
				Properties: map[string]interface{}{
					wspf.PropertyConnectionID: connectionID,
				},
			},
		}
		if err := w.Send(response); err != nil {
			log.Errorf("portForwardHandler: webSock.WriteMessage(%+v)", err)
		}
		return err
	} else {
		return errPortForwardUnkonwnConnection
	}
}

func (h *PortForwardHandler) portForwardHandlerAck(message *ws.ProtoMsg, w api.Sender) error {
	connectionID, _ := message.Header.Properties[wspf.PropertyConnectionID].(string)
	if portForwarder, ok := h.portForwarders[connectionID]; ok {
		// unlock the ack mutex, do not panic if it is not locked
		defer func() {
			if r := recover(); r != nil {
				log.Errorf("portForwardHandlerAck: recover(%+v)", r)
			}
		}()
		portForwarder.mutexAck.Unlock()
		return nil
	}
	return errPortForwardUnkonwnConnection
}
