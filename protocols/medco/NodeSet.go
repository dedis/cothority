package medco

import (
	"github.com/dedis/cothority/lib/sda"
	"errors"
	"github.com/satori/go.uuid"
)

const max_Node_Count = 32

var masks = make(map[uuid.UUID]NodeSet)


type NodeSet uint32

func (s *NodeSet) Add(n *sda.TreeNode, tree *sda.Tree) error {
	if mask,err := getMask(n, tree); err == nil {
		*s += mask
		return nil
	} else {
		return errors.New("Cannot add Node to the set:\n"+err.Error())
	}
}

func (s *NodeSet) Contains(n *sda.TreeNode, tree *sda.Tree) bool {
	if mask, err := getMask(n, tree); err == nil {
		return *s & mask != 0
	}
	return false
}


func getMask(n *sda.TreeNode, tree *sda.Tree) (NodeSet, error) {
	var 	(mask NodeSet
		err error
		ok bool)
	if mask,ok = masks[n.Id]; !ok {
		mask, err = computeMask(n, tree)
		if err == nil {
			masks[n.Id] = mask
		}
	}
	return NodeSet(mask), err
}

func computeMask(n *sda.TreeNode, tree *sda.Tree) (NodeSet, error) {
	for i,node := range tree.ListNodes() {
		if node.Equal(n) {
			return NodeSet(1<<uint(i)), nil
		}
	}
	return 0, errors.New("No corresponding TreeNode in Tree.")
}
