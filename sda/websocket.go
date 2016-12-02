package sda

import (
	"fmt"
	"net/http"
	"strconv"

	"reflect"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/protobuf"
	"golang.org/x/net/websocket"
)

// WebSocket handles incoming client-requests using the WebService
// protocol for JavaScript-compatibility.
type WebSocket struct {
	ServerIdentity *network.ServerIdentity
	services       map[string]Service
	server         *http.Server
	mux            *http.ServeMux
}

// NewWebSocket opens a webservice-listener one port above the given
// ServerIdentity.
func NewWebSocket(si *network.ServerIdentity) *WebSocket {
	w := &WebSocket{
		ServerIdentity: si,
		services:       make(map[string]Service),
	}
	go w.Listening()
	return w
}

// Listening starts to listen on the appropriate port.
func (w *WebSocket) Listening() {
	webHost, err := getWebHost(w.ServerIdentity)
	log.ErrFatal(err)
	w.mux = http.NewServeMux()
	w.server = &http.Server{
		Addr:    webHost,
		Handler: w.mux,
	}
	log.Lvl1("Starting to listen on", webHost)
	w.server.ListenAndServe()
}

// RegisterService saves the service as being able to handle messages.
func (w *WebSocket) RegisterService(service string, s Service) error {
	w.services[service] = s
	return nil
}

// Register a message-handler for a service.
func (w *WebSocket) RegisterMessageHandler(service, handler string, t reflect.Type) error {
	log.Lvlf3("Registering websocket for ws://hostname/%s/%s", service, handler)
	h := func(ws *websocket.Conn) {
		log.Print("Got message for", service, handler, ws)
		buf := make([]byte, 128)
		n, err := ws.Read(buf)
		log.Print("Read message:", n, err)
		msg := reflect.New(t)
		err = protobuf.Decode(buf, msg.Interface())
		if err != nil {
			log.Errorf("Couldn't decode msg %s: %x", t, buf, err)
		} else {
			log.Print("Introducing client request to", w.services[service])
			reply := w.services[service].ProcessClientRequest(msg)
			buf, err = protobuf.Encode(reply)
			if err != nil {
				log.Error(buf)
				return
			}
			_, err = ws.Write(buf)
			if err != nil {
				log.Error(err)
				return
			}
			log.Lvl1("Sent reply:", reply)
		}
	}
	w.mux.Handle(fmt.Sprintf("/%s/%s", service, handler), websocket.Handler(h))
	return nil
}

// getWebHost returns the host:port+1 of the serverIdentity.
func getWebHost(si *network.ServerIdentity) (string, error) {
	p, err := strconv.Atoi(si.Address.Port())
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:%d", si.Address.Host(), p+1), nil
}
