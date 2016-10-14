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

func TestTreeNodeSendToItSelf(t *testing.T) {
	log.TestOutput(true, 5)
	GlobalProtocolRegister(ownName, newOwnProto)
	local := NewLocalTest()
	defer local.CloseAll()

	hosts, _, tree := local.GenTree(1, true)
	pi, err := hosts[0].overlay.CreateProtocolSDA(ownName, tree)
	log.ErrFatal(err)
	go pi.Start()

	log.Print("waiting on ownCh")
	<-ownCh

}

var ownCh = make(chan bool)

const ownName = "Own"

type OwnMessage struct {
	Val int
}

type wrapOwn struct {
	*TreeNode
	OwnMessage
}

// ownProto is a protocol that sends a message to itself
type ownProto struct {
	*TreeNodeInstance
}

func newOwnProto(tn *TreeNodeInstance) (ProtocolInstance, error) {
	o := &ownProto{tn}
	o.RegisterHandler(o.receiveOwnMsg)
	return o, nil
}

func (o *ownProto) Start() error {
	log.Print("Sending to ourself")
	err := o.SendTo(o.TreeNode(), &OwnMessage{12})
	log.Print("Sent to ourself DONE", err)
	return nil
}

func (o *ownProto) receiveOwnMsg(wrap wrapOwn) error {
	ownCh <- true
	o.Done()
	return nil
}
