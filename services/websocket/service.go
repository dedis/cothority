package websocket

/*
 */

import (
	"net/http"

	"net"
	"strconv"

	"fmt"

	"io/ioutil"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/cothority/services/status"
	"golang.org/x/net/websocket"
)

// ServiceName is the name to refer to the Template service from another
// package.
const ServiceName = "WebSocket"

func init() {
	sda.RegisterNewService(ServiceName, newService)
}

// Service is our template-service
type Service struct {
	*sda.ServiceProcessor
	path   string
	server *http.Server
}

// NewProtocol is called on all nodes of a Tree (except the root, since it is
// the one starting the protocol) so it's the Service that will be called to
// generate the PI on all others node.
// If you use CreateProtocolSDA, this will not be called, as the SDA will
// instantiate the protocol on its own. If you need more control at the
// instantiation of the protocol, use CreateProtocolService, and you can
// give some extra-configuration to your protocol in here.
func (s *Service) NewProtocol(tn *sda.TreeNodeInstance, conf *sda.GenericConfig) (sda.ProtocolInstance, error) {
	log.Lvl3("Not templated yet")
	return nil, nil
}

func (s *Service) Shutdown() {
	log.Lvl1("Shutting down service websocket")
}

func (s *Service) Listening() {
	go func() {
		log.Lvl1("Starting to listen on", s.ServerIdentity())
		webHost, err := getWebHost(s.ServerIdentity())
		log.ErrFatal(err)
		hand := http.NewServeMux()
		s.server = &http.Server{
			Addr:    webHost,
			Handler: hand,
		}
		hand.Handle("/status", websocket.Handler(s.statusHandler))
		log.ErrFatal(s.server.ListenAndServe())
	}()
}

func (s *Service) statusHandler(ws *websocket.Conn) {
	buf, err := ioutil.ReadAll(ws)
	log.ErrFatal(err)
	_, msg, err := network.UnmarshalRegistered(buf)
	req, ok := msg.(*status.Request)
	log.Print(req, ok)
	log.Print(s.ReportStatus())
}

func getWebHost(si *network.ServerIdentity) (string, error) {
	log.Print(si.Addresses[0])
	host, portStr, err := net.SplitHostPort(si.Addresses[0])
	if err != nil {
		return "", err
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:%d", host, port+100), nil
}

// newTemplate receives the context and a path where it can write its
// configuration, if desired. As we don't know when the service will exit,
// we need to save the configuration on our own from time to time.
func newService(c *sda.Context, path string) sda.Service {
	s := &Service{
		ServiceProcessor: sda.NewServiceProcessor(c),
		path:             path,
	}

	return s
}
