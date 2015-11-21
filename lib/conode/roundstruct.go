package conode
import (
	"github.com/dedis/cothority/lib/sign"
	"github.com/dedis/cothority/lib/coconet"
	"github.com/dedis/crypto/abstract"
)

/*
This gives some basic informations about a round.
 */

type RoundStruct struct {
	name     string
	isRoot   bool
	isLeaf   bool
	roundNbr int
	viewNbr  int
	parent   string
	children map[string]coconet.Conn
	suite    abstract.Suite
}

func NewRoundStruct(node *sign.Node, viewNbr, roundNbr int) *RoundStruct {
	children := node.Children(viewNbr)
	cbs := &RoundStruct{
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

func (r *RoundStruct)SetRoundType(roundType string, out []*sign.SigningMessage) {
	for i := range (out) {
		out[i].Am.RoundType = roundType
	}
}