package randhoundco

import (
	"log"

	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/crypto/abstract"
)

func init() {
	network.RegisterPacketType(GroupRequest{})
	network.RegisterPacketType(GroupRequests{})
	network.RegisterPacketType(Group{})
	network.RegisterPacketType(Groups{})
}

// GroupRequest is used by the leader of this group to know who it should
// include in the JVSS group to form the longterm distributed secret.
type GroupRequest struct {
	Nodes []*network.ServerIdentity
}

// GroupRequests is given to the root / client of the randhoundcoSetup
// protocol so it knows the full topology of JVSS groups to create.
// This message is passed down the tree when the SetupClient protocol is
// launched.
type GroupRequests struct {
	// the session id for the JVSS-based cosi system that is taking place
	// XXX currently generated by the root but will be replaced by the output of
	// randhound once merged.
	Id []byte
	// list of the JVSS groups
	Groups []GroupRequest
	// mapping between the indices of the leaders in the Roster and the
	// group it's responsible for. -1 the index to take into account the first
	// node which is the client and does not participate.
	Leaders []int32
}

func (g *GroupRequests) Dump() {
	log.Print("GroupRequestS: ID", g.Id)
	log.Print("GroupRequestS: Leaders", g.Leaders)
	for i, g := range g.Groups {
		log.Printf("----------------%d--------------------", i)
		for _, n := range g.Nodes {
			log.Print("    ", n.Address)
		}
		log.Print("-------------------------------------")
	}
}

type wrapGroupRequests struct {
	*sda.TreeNode
	GroupRequests
}

// Group designate a list of nodes participating in a JVSS groups.
// This group has ben formed and has already generated the longterm distributed
// key.
type Group struct {
	// the server identity of the member of the group
	Identities []*network.ServerIdentity
	// the longterm public key generated by this group
	Longterm abstract.Point
}

// Groups contains all the generated JVSS groups of the system with the
// aggregate distributed key amongst all groups. This message is passed up to
// the root when the group's longterm keys are generated.
type Groups struct {
	// The ID identifying the whole session / system of JVSS groups
	Id []byte
	// the aggregated JVSS longterm key
	Aggregate abstract.Point
	// the list of JVSS groups
	Groups []Group
}

func (g *Groups) Dump() {
	log.Print("Groups: ID", g.Id)
	log.Print("Groups: Aggregate", g.Aggregate)
	for i, g := range g.Groups {
		log.Printf("----------------%d--------------------", i)
		log.Print(g.Longterm)
		for _, n := range g.Identities {
			log.Print("    ", n.Address)
		}
		log.Print("-------------------------------------")
	}
}

type wrapGroups struct {
	*sda.TreeNode
	Groups
}
