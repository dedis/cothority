package sda

import (
	"testing"

	"github.com/dedis/cothority/log"
)

func TestTreeNodeCreateProtocol(t *testing.T) {
	GlobalProtocolRegister(spawnName, newSpawnProto)
	local := NewLocalTest()
	defer local.CloseAll()

	hosts, _, tree := local.GenTree(1, true)
	pi, err := hosts[0].overlay.CreateProtocolSDA(spawnName, tree)
	log.ErrFatal(err)
	p := pi.(*spawnProto)
	p.spawn = true
	go p.Start()

	// wait once for the protocol just created
	<-spawnCh
	// wait once more for the protocol created inside the first one
	<-spawnCh
}

// spawnCh is used to dispatch information from a spawnProto to the test
var spawnCh = make(chan bool)

const spawnName = "Spawn"

// spawnProto is a simple protocol which just spawn another protocol when
// started
type spawnProto struct {
	*TreeNodeInstance
	spawn bool
}

func newSpawnProto(tn *TreeNodeInstance) (ProtocolInstance, error) {
	return &spawnProto{
		TreeNodeInstance: tn,
	}, nil
}

func (s *spawnProto) Start() error {
	r := s.Roster()
	tree := r.GenerateBinaryTree()
	spawnCh <- true
	if !s.spawn {
		return nil
	}
	proto, err := s.CreateProtocol(spawnName, tree)
	log.ErrFatal(err)
	go proto.Start()
	return nil
}
