package sda

import (
	"fmt"
	"net/http"
	"strconv"

	"time"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"golang.org/x/net/websocket"
	"gopkg.in/tylerb/graceful.v1"
)

// WebSocket handles incoming client-requests using the WebService
// protocol for JavaScript-compatibility.
type WebSocket struct {
	ServerIdentity *network.ServerIdentity
	services       map[string]Service
	server         *graceful.Server
	mux            *http.ServeMux
}

const readSize = 1024
const (
	WebSocketErrorPathNotFound   = 4000
	WebSocketErrorProtobufDecode = 4001
	WebSocketErrorProtobufEncode = 4002
)

// NewWebSocket opens a webservice-listener one port above the given
// ServerIdentity.
func NewWebSocket(si *network.ServerIdentity) *WebSocket {
	w := &WebSocket{
		ServerIdentity: si,
		services:       make(map[string]Service),
	}
	webHost, err := getWebHost(w.ServerIdentity)
	log.ErrFatal(err)
	w.mux = http.NewServeMux()
	w.server = &graceful.Server{
		Timeout: 100 * time.Millisecond,
		Server: &http.Server{
			Addr:    webHost,
			Handler: w.mux,
		},
	}
	log.Lvl1("Starting to listen on", webHost)
	go w.server.ListenAndServe()
	return w
}

// RegisterService saves the service as being able to handle messages.
func (w *WebSocket) RegisterService(service string, s Service) error {
	w.services[service] = s
	return nil
}

// Register a message-handler for a service.
func (w *WebSocket) RegisterMessageHandler(service, path string) error {
	log.Lvlf3("Registering websocket for ws://hostname/%s/%s", service, path)
	h := func(ws *websocket.Conn) {
		var buf []byte
		for {
			tmp := make([]byte, readSize)
			n, err := ws.Read(tmp)
			if err != nil {
				log.Error(err)
				return
			}
			buf = append(buf, tmp[:n]...)
			if n < readSize {
				break
			}
		}
		reply, errCode := w.services[service].ProcessClientRequest(path, buf)
		if errCode == 0 {
			_, err := ws.Write(reply)
			header := make([]byte, 128)
			if ws.HeaderReader() != nil {
				n, errH := ws.HeaderReader().Read(header)
				log.Printf("%d::%d %x", n, errH, header)
			}
			if err != nil {
				log.Error(err)
				return
			}
		}
		ws.WriteClose(errCode)
	}
	w.mux.Handle(fmt.Sprintf("/%s/%s", service, path), websocket.Handler(h))
	return nil
}

func (w *WebSocket) Stop() {
	w.server.Stop(100 * time.Millisecond)
}

// getWebHost returns the host:port+1 of the serverIdentity.
func getWebHost(si *network.ServerIdentity) (string, error) {
	p, err := strconv.Atoi(si.Address.Port())
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:%d", si.Address.Host(), p+1), nil
}
