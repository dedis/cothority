package conode
import (
	"github.com/dedis/cothority/lib/sign"
	"github.com/dedis/cothority/lib/coconet"
	"github.com/dedis/crypto/abstract"
)

/*
This gives some basic informations about a round.
 */

type RoundStructure struct {
	name     string
	isRoot   bool
	isLeaf   bool
	roundNbr int
	viewNbr  int
	parent   string
	children map[string]coconet.Conn
	suite    abstract.Suite
}

func NewRoundStructure(node *sign.Node, viewNbr, roundNbr int) *RoundStructure {
	children := node.Children(viewNbr)
	cbs := &RoundStructure{
		name: node.Name(),
		isRoot: node.IsRoot(viewNbr),
		isLeaf: len(children) == 0,
		roundNbr: roundNbr,
		viewNbr: viewNbr,
		parent: node.Parent(viewNbr),
		children: children,
		suite: node.Suite(),
	}
	return cbs
}
