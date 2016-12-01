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

// A webservice handles incoming client-requests using the WebService
// protocol for JavaScript-compatibility.
type WebService struct {
	ServerIdentity *network.ServerIdentity
	services       map[string]Service
	server         *http.Server
	mux            *http.ServeMux
}

// NewWebService opens a webservice-listener one port above the given
// ServerIdentity.
func NewWebService(si *network.ServerIdentity) (*WebService, error) {
	w := &WebService{
		ServerIdentity: si,
	}
	w.Listening()
	return w, nil
}

// Listening starts to listen on the appropriate port.
func (w *WebService) Listening() {
	go func() {
		webHost, err := getWebHost(w.ServerIdentity)
		log.ErrFatal(err)
		log.Lvl1("Starting to listen on", webHost)
		w.mux = http.NewServeMux()
		w.server = &http.Server{
			Addr:    webHost,
			Handler: w.mux,
		}
		w.server.ListenAndServe()
	}()
}

// RegisterService saves the service as being able to handle messages.
func (w *WebService) RegisterService(service string, s Service) error {
	w.services[service] = s
	return nil
}

// Register a message-handler for a service.
func (w *WebService) RegisterMessageHandler(service, handler string, t reflect.Type) error {
	h := func(ws *websocket.Conn) {
		for {
			msg := reflect.New(t)
			buf := make([]byte, ws.Len())
			err := protobuf.Decode(buf, msg)
			if err != nil {
				log.Errorf("Couldn't decode msg %s: %x", t, buf)
			} else {
				cr := &ClientRequest{}
				w.services[service].ProcessClientRequest(nil, cr)
			}
		}
	}
	w.mux.Handle(service+"/"+handler, websocket.Handler(h))
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
