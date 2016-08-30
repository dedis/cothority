package timestamp

import (
	"errors"
	"fmt"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/protocols/swupdate"
	"github.com/dedis/cothority/sda"
	"time"
)

// ServiceName can be used to refer to the name of this service
const ServiceName = "Timestamp"

var timestampSID sda.ServiceID
var dummyVerfier = func(data []byte) bool {
	log.Print("Got time", string(data))
	return true
}

func init() {
	sda.RegisterNewService(ServiceName, newTimestampService)
	timestampSID = sda.ServiceFactory.ServiceID(ServiceName)

	// TODO register all packets which will be send around:
	// TODO network.RegisterPacketType(&{})
}

type Service struct {
	*sda.ServiceProcessor
	// Epoch is is the time that needs to pass until
	// the timestamp service attempts to collectively sign the batches
	// of statements collected. Reasonable choices would be from 10 seconds
	// upto some hours.
	EpochDuration time.Duration

	path string
	// TODO currentTree
	// TODO queue for requests
}

// NewProtocol is called on all nodes of a Tree (except the root, since it is
// the one starting the protocol) so it's the Service that will be called to
// generate the PI on all others node.
func (s *Service) NewProtocol(tn *sda.TreeNodeInstance, conf *sda.GenericConfig) (sda.ProtocolInstance, error) {
	log.Lvl2("Timestamp Service received New Protocol event")
	var pi sda.ProtocolInstance
	var err error
	/*if tn.ProtocolName() != swupdate.NewCoSiUpdate() {
		return nil, errors.New("Expected " + regularCosi + " as protocol but got " + tn.ProtocolName())
	}*/
	// TODO does this work? Maybe each node should have a unique protocol
	// name instead
	sda.ProtocolRegisterName("UpdateCosi", func(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
		// TODO for now we provide a dummy verification function. It
		// just prints out the timestamp, received in the Announcement.
		return swupdate.NewCoSiUpdate(n, dummyVerfier)
	})
	return pi, err
}

// SignatureRequest is what the Cosi service is expected to receive from clients.
type SignatureRequest struct {
	Message []byte
	Roster  *sda.Roster
}

// SignatureResponse is what the Cosi service will reply to clients.
type SignatureResponse struct {
	Sum       []byte
	Signature []byte
}

// SignatureRequest treats external request to this service.
func (s *Service) SignatureRequest(si *network.ServerIdentity, req *SignatureRequest) (network.Body, error) {
	tree := req.Roster.GenerateBinaryTree()
	tni := s.NewTreeNodeInstance(tree, tree.Root, swupdate.ProtcolName)
	// TODO how often do we want to init the protocol??!
	pi, err := swupdate.NewCoSiUpdate(tni, dummyVerfier)
	if err != nil {
		return nil, errors.New("Couldn't make new protocol: " + err.Error())
	}
	s.RegisterProtocolInstance(pi)

	pi.SigningMessage(req.Message)
	// Take the raw message (already expecting a hash for the timestamp
	// service)
	response := make(chan []byte)
	pi.RegisterSignatureHook(func(sig []byte) {
		response <- sig
	})
	log.Lvl3("Cosi Service starting up root protocol")
	go pi.Dispatch()
	go pi.Start()
	sig := <-response
	fmt.Printf("%s: Signed a message.\n", time.Now().Format("Mon Jan 2 15:04:05 -0700 MST 2006"))

	return &SignatureResponse{
		Sum:       req.Message,
		Signature: sig,
	}, nil
}

func newTimestampService(c *sda.Context, path string) sda.Service {
	s := &Service{
		ServiceProcessor: sda.NewServiceProcessor(c),
		path:             path,
		//TODO Epoch:            epochDuration,
	}
	err := s.RegisterMessage(s.SignatureRequest)
	if err != nil {
		log.ErrFatal(err, "Couldn't register message:")
	}
	return s
}
