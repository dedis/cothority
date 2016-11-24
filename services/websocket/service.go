package websocket

/*
 */

import (
	"net/http"

	"strconv"

	"fmt"

	"encoding/binary"

	"reflect"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/cothority/services/status"
	"github.com/dedis/crypto/eddsa"
	"golang.org/x/net/websocket"
)

// ServiceName is the name to refer to the Template service from another
// package.
const ServiceName = "WebSocket"

func init() {
	sda.RegisterNewService(ServiceName, newService)
	network.RegisterPacketType(&WSStatus{})
	network.RegisterPacketType(&Module{})
}

// Service is our template-service
type Service struct {
	*sda.ServiceProcessor
	path   string
	server *http.Server
}

type Module struct {
	Module map[string]string
}

// Simple status-example
type WSStatus struct {
	//Status map[string]string
	Status map[string]*Module
	//Status map[string]sda.Status
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
		webHost, err := getWebHost(s.ServerIdentity())
		log.ErrFatal(err)
		log.Lvl1("Starting to listen on", webHost)
		hand := http.NewServeMux()
		s.server = &http.Server{
			Addr:    webHost,
			Handler: hand,
		}
		hand.Handle("/debug", websocket.Handler(s.debugHandler))
		hand.Handle("/ping", websocket.Handler(s.oldPingHandler))
		hand.Handle("/status", websocket.Handler(s.statusHandler))
		hand.Handle("/sign", websocket.Handler(s.signHandler))
		log.ErrFatal(s.server.ListenAndServe())
	}()
}

func (s *Service) debugHandler(ws *websocket.Conn) {
	log.Lvl1("Started debug")
	for {
		buf := make([]byte, 1)
		_, err := ws.Read(buf)
		if err != nil {
			return
		}
		log.Printf("Received %x", buf)
	}
}

func (s *Service) newPingHandler(ws *websocket.Conn) {
	log.Print()
}

func (s *Service) oldPingHandler(ws *websocket.Conn) {
	log.Lvl1("Started ping")
	buf := make([]byte, 4)
	_, err := ws.Read(buf)
	log.Print("Received", buf)
	if err != nil {
		log.Error(err)
		return
	}
	_, err = ws.Write([]byte("pong"))
	if err != nil {
		log.Error(err)
		return
	}
	log.Lvl1("Sent pong")
}

func (s *Service) statusHandler(ws *websocket.Conn) {
	log.Lvl1("starting to handle")
	sizeBuf := make([]byte, 2)
	n, err := ws.Read(sizeBuf)
	if err != nil {
		log.Error(err)
		return
	}
	if n != 2 {
		log.Error("Couldn't read 2 bytes")
		return
	}
	size := binary.LittleEndian.Uint16(sizeBuf)
	buf := make([]byte, size)
	read, err := ws.Read(buf)
	if err != nil {
		log.Error(err)
		return
	}
	if read != int(size) {
		log.Error("Read only", read, "instead of", size)
		return
	}
	_, msg, err := network.UnmarshalRegistered(buf)
	req, ok := msg.(*status.Request)
	log.Lvlf1("Received request: %x %v %t", buf, req, ok)
	//stat := s.GetService(status.ServiceName)
	//reply, err := stat.(*status.Stat).Request(nil, req)
	//if err != nil {
	//	log.Error(err)
	//	return
	//}
	log.Lvl1(s.ReportStatus())
	msgStat := &WSStatus{Status: map[string]*Module{}}
	for k, v := range s.ReportStatus() {
		msgStat.Status[k] = &Module{Module: map[string]string{}}
		for k2, v2 := range v {
			msgStat.Status[k].Module[k2] = v2
		}
	}
	log.Lvl1("Sending", msgStat)
	buf, err = network.MarshalRegisteredType(msgStat)
	if err != nil {
		log.Error(err)
		return
	}
	err = websocket.Message.Send(ws, buf)
	if err != nil {
		log.Error(err)
		return
	}
	log.Lvl1("Sent message")
}

func (s *Service) signHandler(ws *websocket.Conn) {
	log.Lvl1("starting to handle")
	sizeBuf := make([]byte, 2)
	n, err := ws.Read(sizeBuf)
	if err != nil {
		log.Error(err)
		return
	}
	if n != 2 {
		log.Error("Couldn't read 2 bytes")
		return
	}
	size := binary.LittleEndian.Uint16(sizeBuf)
	buf := make([]byte, size)
	read, err := ws.Read(buf)
	if err != nil {
		log.Error(err)
		return
	}
	if read != int(size) {
		log.Error("Read only", read, "instead of", size)
		return
	}
	_, msg, err := network.UnmarshalRegistered(buf)
	log.Lvlf1("Received message: %v", reflect.TypeOf(msg))
	req, ok := msg.(*SignRequest)
	log.Lvlf1("Received request: buf(%x) req(%v) converted(%t)", buf, req, ok)
	//req = &SignRequest{Hash: []byte("myhash")}
	keypair := eddsa.NewEdDSA(nil)
	agg, err := keypair.Public.MarshalBinary()
	if err != nil {
		log.Error(err)
		return
	}
	log.Printf("Hash is %x", req.Hash)
	signature, err := keypair.Sign(req.Hash)
	if err != nil {
		log.Error(err)
		return
	}
	//agg, _ := s.ServerIdentity().Public.MarshalBinary()
	//ssignature, err := crypto.SignSchnorr(network.Suite, s.Private(), req.Hash)
	//if err != nil {
	//	log.Error(err)
	//	return
	//}
	//signature, _ := ssignature.MarshalBinary()
	signReply := &SignReply{
		Signature: signature,
		Aggregate: agg,
	}
	log.Lvlf1("Sending %x", signReply)
	buf, err = network.MarshalRegisteredType(signReply)
	if err != nil {
		log.Error(err)
		return
	}
	err = websocket.Message.Send(ws, buf)
	if err != nil {
		log.Error(err)
		return
	}
	log.Lvl1("Sent message")
}

func (s *Service) PingHandler(si *network.ServerIdentity, p *Ping) (network.Body, error) {
	return p, nil
}

type Stat struct {
	Host map[string]string
}

func getWebHost(si *network.ServerIdentity) (string, error) {
	p, err := strconv.Atoi(si.Address.Port())
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:%d", si.Address.Host(), p+100), nil
}

// newTemplate receives the context and a path where it can write its
// configuration, if desired. As we don't know when the service will exit,
// we need to save the configuration on our own from time to time.
func newService(c *sda.Context, path string) sda.Service {
	s := &Service{
		ServiceProcessor: sda.NewServiceProcessor(c),
		path:             path,
	}

	network.RegisterPacketType(Stat{})
	s.RegisterMessage(s.PingHandler)
	s.Listening()
	return s
}
