package protocol

import (
	"errors"
	"fmt"

	"github.com/dedis/onet"
	"github.com/dedis/onet/network"
)

// BlsProtocolTree represents the subtrees used in the BLS CoSi protocol
type BlsProtocolTree []*onet.Tree

// NewBlsProtocolTree creates a new tree that can be used in the BLS CoSi protocol
func NewBlsProtocolTree(tree *onet.Tree, nSubTrees int) (BlsProtocolTree, error) {
	return genTrees(tree, nSubTrees)
}

// GetLeaves returns the server identities of the leaves
func (pt BlsProtocolTree) GetLeaves() []*network.ServerIdentity {
	si := []*network.ServerIdentity{}

	for _, t := range pt {
		for _, c := range t.Root.Children[0].Children {
			si = append(si, c.ServerIdentity)
		}
	}

	return si
}

// GetSubLeaders returns the server identities of the subleaders
func (pt BlsProtocolTree) GetSubLeaders() []*network.ServerIdentity {
	si := []*network.ServerIdentity{}

	for _, t := range pt {
		si = append(si, t.Root.Children[0].ServerIdentity)
	}

	return si
}

// genTrees will create a given number of subtrees of the same number of nodes.
// Each generated subtree will have the same root.
// Each generated tree has a root with one child (the subleader)
// and all other nodes in the tree will be the subleader children.
// NOTE: register being not implementable with the current API could hurt the scalability tests
// TODO: we may be able to simplify the code here to make sure the existing onet
// tree generation functions.
func genTrees(tree *onet.Tree, nSubtrees int) ([]*onet.Tree, error) {
	roster := tree.Roster
	nNodes := len(roster.List)
	root := tree.Root.RosterIndex

	// parameter verification
	if roster == nil {
		return nil, errors.New("the roster is nil")
	}
	if nNodes < 1 {
		return nil, fmt.Errorf("the number of nodes in the trees "+
			"cannot be less than one, but is %d", nNodes)
	}
	if nSubtrees < 1 {
		return nil, fmt.Errorf("the number of subtrees"+
			"cannot be less than one, but is %d", nSubtrees)
	}

	if nNodes <= nSubtrees {
		nSubtrees = nNodes - 1
	}

	trees := make([]*onet.Tree, nSubtrees)

	if nSubtrees == 0 {
		rootNode := onet.NewTreeNode(root, roster.List[root])
		trees = append(trees, onet.NewTree(roster, rootNode))
		return trees, nil
	}

	// generate each subtree
	nodesPerSubtree := (nNodes - 1) / nSubtrees
	surplusNodes := (nNodes - 1) % nSubtrees

	pointer := 0
	for i := 0; i < nSubtrees; i++ {
		length := nodesPerSubtree + 1
		if i < surplusNodes { // to handle surplus nodes
			length++
		}

		// generate indexes to the roster
		nodes := make([]int, length)
		for j := range nodes {
			if j == 0 {
				nodes[j] = root
			} else {
				if pointer == root {
					pointer++
				}
				nodes[j] = pointer
				pointer++
			}
		}

		var err error
		trees[i], err = genSubtree(roster, nodes)
		if err != nil {
			return nil, err
		}
	}

	return trees, nil
}

// genSubtree generates a single subtree defined by the list of indexes
// to the rootRoster.
// The generated tree will have a root with one child (the subleader)
// and all other nodes in the roster will be the subleader children.
func genSubtree(roster *onet.Roster, nodes []int) (*onet.Tree, error) {
	if roster == nil {
		return nil, fmt.Errorf("the roster should not be nil, but is")
	}
	if len(nodes) < 2 {
		return nil, fmt.Errorf("the node list must be greater than 1, but is %d", len(nodes))
	}
	if len(nodes) > len(roster.List) {
		return nil, errors.New("nodes list is longer than roster list")
	}
	for _, i := range nodes {
		if i < 0 || i >= len(roster.List) {
			return nil, errors.New("element of nodes list is out of range")
		}
	}
	if nodes[0] == nodes[1] {
		return nil, errors.New("subleader and leader cannot be the same")
	}
	for _, i := range nodes[2:] {
		if i == nodes[0] || i == nodes[1] {
			return nil, errors.New("duplicate root or subleader node")
		}
	}

	// generate leader and subleader
	rootNode := onet.NewTreeNode(nodes[0], roster.List[nodes[0]])
	subleader := onet.NewTreeNode(nodes[1], roster.List[nodes[1]])
	subleader.Parent = rootNode
	rootNode.Children = []*onet.TreeNode{subleader}

	// generate leaves
	for j := 2; j < len(nodes); j++ {
		node := onet.NewTreeNode(nodes[j], roster.List[nodes[j]])
		node.Parent = subleader
		subleader.Children = append(subleader.Children, node)
	}

	return onet.NewTree(roster, rootNode), nil
}
