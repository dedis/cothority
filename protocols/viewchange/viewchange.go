package view

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/satori/go.uuid"
	"math"
)

type ViewChange struct {
	// the node we are
	*sda.Node
	// treeInfos contains all the tree propositions we have. As soon as we have
	// one tree that reached the threshold of votes, we takes this one.
	// map from Tree.Id <=> TreeInfo
	treeInfos map[uuid.UUID]*TreeInfo
	// channel used to receive the new entityList + tree
	viewChan chan viewMsgChan
	// trheshold of when to accept the new view
	threshold int
	// onDone is the callback that will be called when the view change protocol
	// is finished
	onDone func()
	// initiator
	initiator bool
}

// TreeInfo is a pair of the Tree and the associated number of votes that we
// have received from other peers for this tree.
type TreeInfo struct {
	Tree  *sda.Tree
	count int
}

// NewViewChange is the simple function needed to register to SDA.
func NewViewChange(node *sda.Node) (*ViewChange, error) {
	vcp := &ViewChange{
		Node:      node,
		treeInfos: make(map[uuid.UUID]*TreeInfo),
	}
	if err := node.RegisterChannel(&vcp.viewChan); err != nil {
		return nil, err
	}

	vcp.threshold = int(math.Ceil(float64(len(vcp.EntityList().List)) / 3.0))
	go vcp.listen()
	return vcp, nil
}

// NewTreeInfo returns a fresh TreeInfo
func NewTreeInfo(tree *sda.Tree) *TreeInfo {
	return &TreeInfo{
		Tree:  tree,
		count: 0,
	}
}

func (ti *TreeInfo) AddVote() {
	ti.count++
}

func (ti *TreeInfo) IsMajority(threshold int) bool {
	return ti.count >= threshold
}

// SetupViewChange the function that a protocol can call when it
// needs to operate a view change. You must supply here the new entityList + new
// Tree you wish to apply to the tree.
func (vc *ViewChange) Propagate(tree *sda.Tree) error {
	ti := NewTreeInfo(tree)
	vc.treeInfos[tree.Id] = ti
	// create the message to broadcast
	tm := tree.MakeTreeMarshal()
	view := &View{
		EntityList: tree.EntityList,
		Tree:       tm,
	}
	// send to everyone else.
	var err error
	for _, n := range vc.Tree().ListNodes() {
		if uuid.Equal(n.Id, vc.TreeNode().Id) {
			continue
		}
		err = vc.SendTo(n, view)
	}
	return err
}

func (vcp *ViewChange) Start() error {
	panic("Should not be called for the moment")
}

func (vcp *ViewChange) Dispatch() error {
	// do nothing
	return nil
}

func (vcp *ViewChange) RegisterOnDoneCallback(fn func()) {
	vcp.onDone = fn
}

// waitAgreement will wait until it receis 2/3 of the peers.
func (vcp *ViewChange) WaitAgreement() {
	done := make(chan bool)
	fn := func() {
		done <- true
	}
	vcp.RegisterOnDoneCallback(fn)
	// wait the done signal
	<-done
}

func (vcp *ViewChange) listen() {
	for {
		select {
		case msg := <-vcp.viewChan:
			tree, err := msg.View.Tree.MakeTree(msg.View.EntityList)
			if err != nil {
				dbg.Error("Received wrong tree:", err)
				continue
			}
			var ti *TreeInfo
			var ok bool
			// do we have this tree yet or not ?
			if ti, ok = vcp.treeInfos[tree.Id]; !ok {
				dbg.Print(vcp.Name(), "Received unknown tree", tree)
				// create the TreeInfo
				ti = NewTreeInfo(tree)
				vcp.treeInfos[tree.Id] = ti
			}
			ti.AddVote()
			dbg.Print(vcp.Name(), "TreeInfo currently =", ti)
			// view change is accepted by a majority => GO
			if ti.IsMajority(vcp.threshold) {
				// register the new tree and the new entityList
				vcp.Node.Host().AddEntityList(ti.Tree.EntityList)
				vcp.Node.Host().AddTree(ti.Tree)
				// notify
				if vcp.onDone != nil {
					vcp.onDone()
				}
				//break out
				break
			}
			// if I am not an initiator of this
		}
	}
}

type viewMsgChan struct {
	*sda.TreeNode
	View
}

type View struct {
	EntityList *sda.EntityList
	Tree       *sda.TreeMarshal
}
