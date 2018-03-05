package protocol

import (
	"errors"
	"fmt"

	"gopkg.in/dedis/onet.v2"
	"gopkg.in/dedis/onet.v2/network"
)

// genTrees will create a given number of subtrees of the same number of nodes.
// Each generated subtree will have the same root.
// Each generated tree have a root with one child (the subleader)
// and all other nodes in the tree will be the subleader children.
// NOTE: register being not implementable with the current API could hurt the scalability tests
// TODO: we may be able to simplify the code here to make sure the existing onet
// tree generation functions.
func genTrees(roster *onet.Roster, nNodes, nSubtrees int) ([]*onet.Tree, error) {

	// parameter verification
	if roster == nil {
		return nil, errors.New("the roster is nil")
	}
	if nNodes < 1 {
		return nil, fmt.Errorf("the number of nodes in the trees "+
			"cannot be less than one, but is %d", nNodes)
	}
	if len(roster.List) < nNodes {
		return nil, fmt.Errorf("the trees should have %d nodes, "+
			"but there is only %d servers in the roster", nNodes, len(roster.List))
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
		localRoster := onet.NewRoster(roster.List[0:1])
		rootNode := onet.NewTreeNode(0, localRoster.List[0])
		trees = append(trees, onet.NewTree(localRoster, rootNode))
		return trees, nil
	}

	// generate each subtree
	nodesPerSubtree := (nNodes - 1) / nSubtrees
	surplusNodes := (nNodes - 1) % nSubtrees

	start := 1
	for i := 0; i < nSubtrees; i++ {

		end := start + nodesPerSubtree
		if i < surplusNodes { // to handle surplus nodes
			end++
		}

		// generate tree roster
		servers := []*network.ServerIdentity{roster.List[0]}
		servers = append(servers, roster.List[start:end]...)
		treeRoster := onet.NewRoster(servers)

		var err error
		trees[i], err = genSubtree(treeRoster, 1)
		if err != nil {
			return nil, err
		}

		start = end
	}

	return trees, nil
}

// genSubtree generates a single subtree with a given subleaderID.
// The generated tree will have a root with one child (the subleader)
// and all other nodes in the roster will be the subleader children.
func genSubtree(roster *onet.Roster, subleaderID int) (*onet.Tree, error) {

	if roster == nil {
		return nil, fmt.Errorf("the roster should not be nil, but is")
	}
	if len(roster.List) < 2 {
		return nil, fmt.Errorf("the roster size must be greater than 1, but is %d", len(roster.List))
	}
	if subleaderID < 1 || subleaderID >= len(roster.List) {
		return nil, fmt.Errorf("the subleader id should be between in range [1, %d] (size of roster), but is %d", len(roster.List)-1, subleaderID)
	}

	// generate leader and subleader
	rootNode := onet.NewTreeNode(0, roster.List[0])
	subleader := onet.NewTreeNode(subleaderID, roster.List[subleaderID])
	subleader.Parent = rootNode
	rootNode.Children = []*onet.TreeNode{subleader}

	// generate leaves
	for j := 1; j < len(roster.List); j++ {
		if j != subleaderID {
			node := onet.NewTreeNode(j, roster.List[j])
			node.Parent = subleader
			subleader.Children = append(subleader.Children, node)
		}
	}

	return onet.NewTree(roster, rootNode), nil
}
