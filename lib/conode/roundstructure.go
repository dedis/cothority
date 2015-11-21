package conode
import (
	"github.com/dedis/cothority/lib/sign"
	"github.com/dedis/cothority/lib/coconet"
)

/*
This gives some basic informations about a round.
 */

type RoundStructure struct {
	isRoot   bool
	isLeaf   bool
	roundNbr int
	viewNbr  int
	children map[string]coconet.Conn
}

func NewRoundStructure(node *sign.Node, viewNbr, roundNbr int) *RoundStructure {
	children := node.Children(viewNbr)
	cbs := &RoundStructure{
		isRoot: node.IsRoot(viewNbr),
		isLeaf: len(children) == 0,
		roundNbr: roundNbr,
		viewNbr: viewNbr,
		children: children,
	}
	return cbs
}
