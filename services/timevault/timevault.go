package timevault

import (
	"errors"
	"time"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/protocols/timevault"
	"github.com/dedis/crypto/abstract"
)

// ServiceName is the name to refer to the CoSi service
const ServiceName = "Timevault"

func init() {
	sda.RegisterNewService(ServiceName, newTimevaultService)
}

type service struct {
	*sda.ServiceProcessor
	c    sda.Context
	path string
	// current holds all the timevault protocols running currently
	current map[sda.EntityListID]*timevault.TimeVault
}

type sealRequest struct {
	Group   *sda.EntityList
	Timeout time.Duration
}

var sealRequestType = network.RegisterMessageType(sealRequest{})

// SealResponse is the response coming out from the timevault Service when asked
// to Seal(). It contains the public key that can be used for anything and its
// ID to correctly identifying the "sealing".
type SealResponse struct {
	Key abstract.Point
	ID  timevault.SID
}

// SealResponseType is the type that will be sent through the network for this
// message
var SealResponseType = network.RegisterMessageType(SealResponse{})

// OpenRequest is the request to ask the timevault Service to reveal a private
// key
type openRequest struct {
	// EntityList to ask for opening
	Group *sda.EntityList
	// ID is the ID of the "seal"'d private key
	ID timevault.SID
}

// OpenRequestType is the type that goes through the network for tthe
// OpenRequest packet
var OpenRequestType = network.RegisterMessageType(openRequest{})

// OpenResponse contains the private key of a sealing
type OpenResponse struct {
	// ID is the ID of the sealing
	ID timevault.SID
	// Private is the corresponding private key
	Private abstract.Secret
}

// OpenResponseType is the type that goes through the network for an
// OpenResponse type
var OpenResponseType = network.RegisterMessageType(OpenResponse{})

func newTimevaultService(c sda.Context, path string) sda.Service {
	tv := &service{
		ServiceProcessor: sda.NewServiceProcessor(c),
		c:                c,
		path:             path,
		current:          make(map[sda.EntityListID]*timevault.TimeVault),
	}
	dbg.ErrFatal(tv.RegisterMessage(tv.handleClientSeal))
	dbg.ErrFatal(tv.RegisterMessage(tv.handleClientOpen))
	return tv
}

// NewProtocol implements the Service interface
func (tv *service) NewProtocol(tn *sda.TreeNodeInstance, conf *sda.GenericConfig) (sda.ProtocolInstance, error) {
	dbg.Lvl2("Timevault service -> New Protocol event")
	pi, err := timevault.NewTimeVaultProtocol(tn)
	tv.current[tn.EntityList().Id] = pi
	return pi, err
}

func (tv *service) handleClientSeal(e *network.Entity, seal *sealRequest) (network.ProtocolMessage, error) {
	tree := seal.Group.GenerateBinaryTree()
	tni := tv.c.NewTreeNodeInstance(tree, tree.Root)
	dbg.Lvl2("Timevault service -> Client Seal request")
	p, err := timevault.NewTimeVaultProtocol(tni)
	if err != nil {
		return nil, err
	}
	if err := tv.c.RegisterProtocolInstance(p); err != nil {
		dbg.Error(err)
	}
	tv.current[tni.EntityList().Id] = p
	id, pub, err := p.Seal(seal.Timeout)
	dbg.Print("handleClientSeal", id)
	resp := &SealResponse{
		Key: pub,
		ID:  id,
	}
	return resp, err
}

var ErrUnknownGroup = errors.New("Group requested to Open is not known to this service")

func (tv *service) handleClientOpen(e *network.Entity, open *openRequest) (network.ProtocolMessage, error) {
	proto, ok := tv.current[open.Group.Id]
	if !ok {
		return nil, ErrUnknownGroup
	}
	dbg.Print("handleClientOpen", open.ID)
	x, err := proto.Open(open.ID)
	if err != nil {
		return nil, err
	}

	return &OpenResponse{
		ID:      open.ID,
		Private: x,
	}, nil
}
