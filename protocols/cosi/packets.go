package cosi

import (
	"errors"

	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/crypto/abstract"
)

func init() {
	sda.RegisterProtocolIO(func() sda.ProtocolIO {
		return new(ProtocolIO)
	})
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
var ProtocolPacketID = network.RegisterPacketType(ProtocolPacket{})

// ProtocolPacket is the main message for the CoSi protocol which includes
// every information that the CoSi protocol might need.
type ProtocolPacket struct {
	Phase uint32

	OverlayMessage *sda.OverlayMessage

	Ann  *Announcement
	Comm *Commitment
	Chal *Challenge
	Resp *Response
}

// ProtocolIO implements the sda.ProtocolIO interface for the CoSi protocol.
type ProtocolIO struct{}

// Wrap takes a dynamic OverlayMessage and returns a ProtocolPacket.
func (p *ProtocolIO) Wrap(msg interface{}, info *sda.OverlayMessage) (interface{}, error) {
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

// Unwrap takes a ProtocolPacket and returns the corresponding dynamic OverlayMessage.
func (p *ProtocolIO) Unwrap(msg interface{}) (interface{}, *sda.OverlayMessage, error) {
	var inner interface{}
	packet, ok := msg.(ProtocolPacket)
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

// PacketType returns the ProtocolPacket-type.
func (p *ProtocolIO) PacketType() network.PacketTypeID {
	return ProtocolPacketID
}

func (p *ProtocolIO) Name() string {
	return Name
}

// Announcement is sent down the tree to start the collective signature.
type Announcement struct {
}

// Commitment of all nodes, aggregated over all children.
type Commitment struct {
	Comm abstract.Point
}

// Challenge is the challenge against the aggregate commitment.
type Challenge struct {
	Chall abstract.Scalar
}

// Response of all nodes, aggregated over all children.
type Response struct {
	Resp abstract.Scalar
}

// Overlay-structures to retrieve the sending TreeNode.
type chanAnnouncement struct {
	*sda.TreeNode
	Announcement
}

type chanCommitment struct {
	*sda.TreeNode
	Commitment
}

type chanChallenge struct {
	*sda.TreeNode
	Challenge
}

type chanResponse struct {
	*sda.TreeNode
	Response
}
