package cosi

import (
	"errors"

	"go.dedis.ch/kyber/v4"
	"go.dedis.ch/onet/v4"
	"go.dedis.ch/onet/v4/network"
)

func init() {
	onet.RegisterMessageProxy(func() onet.MessageProxy {
		return new(MessageProxy)
	})
	for _, r := range []interface{}{
		Announcement{},
		Commitment{},
		Challenge{},
		Response{},
	} {
		network.RegisterMessage(r)
	}
}

const (
	// AnnouncementPhase is the ID of the Announcement message
	AnnouncementPhase uint32 = 1
	// CommitmentPhase  is the ID of the Commitment message
	CommitmentPhase = 2
	// ChallengePhase is the ID of the Challenge message
	ChallengePhase = 3
	// ResponsePhase is the ID of the Response message
	ResponsePhase = 4
)

// ProtocolPacketID is the network.PacketTypeID of the CoSi ProtocolPacket
var ProtocolPacketID = network.RegisterMessage(ProtocolPacket{})

// ProtocolPacket is the main message for the CoSi protocol which includes
// every information that the CoSi protocol might need.
type ProtocolPacket struct {
	Phase uint32

	OverlayMessage *onet.OverlayMsg

	Ann  *Announcement
	Comm *Commitment
	Chal *Challenge
	Resp *Response
}

// MessageProxy implements the onet.MessageProxy interface for the CoSi protocol.
type MessageProxy struct{}

// Wrap implements the onet.MessageProxy interface by wrapping up any of the
// four-step messages into a ProtooclPacket.
func (p *MessageProxy) Wrap(msg interface{}, info *onet.OverlayMsg) (interface{}, error) {
	var packet = new(ProtocolPacket)
	packet.OverlayMessage = info

	switch inner := msg.(type) {
	case *Announcement:
		packet.Ann = inner
		packet.Phase = AnnouncementPhase
	case *Commitment:
		packet.Comm = inner
		packet.Phase = CommitmentPhase
	case *Challenge:
		packet.Chal = inner
		packet.Phase = ChallengePhase
	case *Response:
		packet.Resp = inner
		packet.Phase = ResponsePhase
	}

	return packet, nil
}

// Unwrap implements the onet.MessageProxy interface by unwraping and returning the
// specific message of one of the four steps.
func (p *MessageProxy) Unwrap(msg interface{}) (interface{}, *onet.OverlayMsg, error) {
	var inner interface{}
	packet, ok := msg.(*ProtocolPacket)
	if !ok {
		return nil, nil, errors.New("cosi protocolio: unknown packet to unwrap")
	}

	if packet.OverlayMessage == nil {
		return nil, nil, errors.New("cosi protocolio: no overlay information given")
	}

	switch packet.Phase {
	case AnnouncementPhase:
		inner = packet.Ann
	case CommitmentPhase:
		inner = packet.Comm
	case ChallengePhase:
		inner = packet.Chal
	case ResponsePhase:
		inner = packet.Resp
	}
	return inner, packet.OverlayMessage, nil
}

// PacketType implements the onet.MessageProxy interface by returning the type of
// the ProtocolPacket.
func (p *MessageProxy) PacketType() network.MessageTypeID {
	return ProtocolPacketID
}

// Name implements the onet.MessageProxy interface by returning the name under
// which cosi.MessageProxy is registered.
func (p *MessageProxy) Name() string {
	return Name
}

// Announcement is sent down the tree to start the collective signature.
type Announcement struct {
}

// Commitment of all nodes, aggregated over all children.
type Commitment struct {
	Comm kyber.Point
}

// Challenge is the challenge against the aggregate commitment.
type Challenge struct {
	Chall kyber.Scalar
}

// Response of all nodes, aggregated over all children.
type Response struct {
	Resp kyber.Scalar
}

// Overlay-structures to retrieve the sending TreeNode.
type chanAnnouncement struct {
	*onet.TreeNode
	Announcement
}

type chanCommitment struct {
	*onet.TreeNode
	Commitment
}

type chanChallenge struct {
	*onet.TreeNode
	Challenge
}

type chanResponse struct {
	*onet.TreeNode
	Response
}
