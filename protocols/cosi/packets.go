package cosi

import (
	"errors"

	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/crypto/abstract"
)

func init() {
	sda.RegisterProtocolIO(Name, func() sda.ProtocolIO {
		return new(ProtocolIO)
	})
}

const (
	AnnouncementPhase uint32 = 1
	CommitmentPhase          = 2
	ChallengePhase           = 3
	ResponsePhase            = 4
)

var ProtocolPacketID = network.RegisterPacketType(ProtocolPacket{})

type ProtocolPacket struct {
	Phase uint32

	Info *sda.OverlayMessage

	Ann  *Announcement
	Comm *Commitment
	Chal *Challenge
	Resp *Response
}

type ProtocolIO struct{}

func (p *ProtocolIO) Wrap(msg interface{}, info *sda.OverlayMessage) (interface{}, error) {
	var packet = new(ProtocolPacket)
	packet.Info = info

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

func (p *ProtocolIO) Unwrap(msg interface{}) (interface{}, *sda.OverlayMessage, error) {
	var inner interface{}
	packet, ok := msg.(ProtocolPacket)
	if !ok {
		return nil, nil, errors.New("cosi protocolio: unknown packet to unwrap")
	}

	if packet.Info == nil {
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
	return inner, packet.Info, nil
}

func (p *ProtocolIO) PacketType() network.PacketTypeID {
	return ProtocolPacketID
}

// Announcement is broadcasted message initiated and signed by proposer.
type Announcement struct {
}

// Commitment of all nodes together with the data they want
// to have signed
type Commitment struct {
	Comm abstract.Point
}

// Challenge is the challenge computed by the root-node.
type Challenge struct {
	Chall abstract.Scalar
}

// Response with which every node replies with.
type Response struct {
	Resp abstract.Scalar
}

//Theses are pairs of TreeNode + the actual message we want to listen on.
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
