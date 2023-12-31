// Copyright 2023 Northern.tech AS
//
//	Licensed under the Apache License, Version 2.0 (the "License");
//	you may not use this file except in compliance with the License.
//	You may obtain a copy of the License at
//
//	    http://www.apache.org/licenses/LICENSE-2.0
//
//	Unless required by applicable law or agreed to in writing, software
//	distributed under the License is distributed on an "AS IS" BASIS,
//	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//	See the License for the specific language governing permissions and
//	limitations under the License.
package shell

import (
	"bufio"
	"errors"
	"io"

	"github.com/mendersoftware/go-lib-micro/ws"
	wsshell "github.com/mendersoftware/go-lib-micro/ws/shell"
	log "github.com/sirupsen/logrus"

	"github.com/northerntechhq/nt-connect/api"
)

var (
	ErrExecWriteBytesShort = errors.New("failed to write the whole message")
)

const pipStdoutBufferSize = 255

type Shell struct {
	sock      api.Sender
	sessionId string
	r         io.Reader
	w         io.Writer
	running   bool
}

// Create a new shell, note that we assume that r Reader and w Writer
// are already connected to the i/o of the shell process and ws websocket
// is connected and ws.SetReadDeadline(time.Now().Add(defaultPingWait))
// was already called and ping-pong was established
func NewShell(sock api.Sender, sessionId string, r io.Reader, w io.Writer) *Shell {
	shell := Shell{
		sock:      sock,
		sessionId: sessionId,
		r:         r,
		w:         w,
		running:   false,
	}
	return &shell
}

func (s *Shell) Start() {
	go s.pipeStdout()
	s.running = true
}

func (s *Shell) Stop() {
	s.running = false
}

func (s *Shell) IsRunning() bool {
	return s.running
}

func (s *Shell) sendStopMessage(err error) {
	body := []byte{}
	status := wsshell.ErrorMessage
	if err != nil {
		body = []byte(err.Error())
		status = wsshell.ErrorMessage
	}
	msg := ws.ProtoMsg{
		Header: ws.ProtoHdr{
			Proto:     ws.ProtoTypeShell,
			MsgType:   wsshell.MessageTypeStopShell,
			SessionID: s.sessionId,
			Properties: map[string]interface{}{
				"status": status,
			},
		},
		Body: body,
	}
	err = s.sock.Send(msg)
	if err != nil {
		log.Debugf("error on write: %s", err.Error())
	}
}

func (s *Shell) pipeStdout() {
	raw := make([]byte, pipStdoutBufferSize)
	sr := bufio.NewReader(s.r)
	for {
		if !s.IsRunning() {
			return
		}
		n, err := sr.Read(raw)
		if err != nil {
			log.Errorf("error reading stdout: %s", err)
			s.sendStopMessage(err)
			return
		} else if !s.IsRunning() {
			return
		}

		msg := ws.ProtoMsg{
			Header: ws.ProtoHdr{
				Proto:     ws.ProtoTypeShell,
				MsgType:   wsshell.MessageTypeShellCommand,
				SessionID: s.sessionId,
				Properties: map[string]interface{}{
					"status": wsshell.NormalMessage,
				},
			},
			Body: raw[:n],
		}

		err = s.sock.Send(msg)
		if err != nil {
			log.Debugf("error on write: %s", err.Error())
		}
	}
}
