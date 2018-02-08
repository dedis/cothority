package protocol

/*
The test-file should at the very least run the protocol for a varying number
of nodes. It is even better practice to test the different methods of the
protocol, as in Test Driven Development.
*/

import (
	"testing"

	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}

//tests the root of the trees
func TestGenTreesRoot(t *testing.T) {
	nodes := []int{1, 2, 5, 20}
	subtrees := []int{1, 5, 12}
	for _, nbrNodes := range nodes {
		for _, nSubtrees := range subtrees {
			local := onet.NewLocalTest(testSuite)
			servers := local.GenServers(nbrNodes)
			roster := local.GenRosterFromHost(servers...)

			trees, err := GenTrees(roster, nbrNodes, nSubtrees)
			if err != nil {
				t.Fatal("Error in tree generation:", err)
			}
			root := trees[0].Root.ServerIdentity
			for _, tree := range trees {
				if tree.Root == nil {
					t.Fatal("Tree Root shouldn't be nil")
				}
				if tree.Root.ServerIdentity != root {
					t.Fatal("Tree Root should be the same for all trees, but isn't")
				}
				testNode(t, tree.Root, nil, tree)
			}
			local.CloseAll()
		}
	}
}

//tests the number of nodes of the tree
func TestGenTreesCount(t *testing.T) {
	nodes := []int{1, 2, 5, 20}
	subtrees := []int{1, 5, 12}
	for _, nNodes := range nodes {
		for _, nSubtrees := range subtrees {
			local := onet.NewLocalTest(testSuite)
			servers := local.GenServers(nNodes)
			roster := local.GenRosterFromHost(servers...)

			trees, err := GenTrees(roster, nNodes, nSubtrees)
			if err != nil {
				t.Fatal("Error in tree generation:", err)
			}
			totalNodes := 1
			expectedNodesPerTree := (nNodes-1)/len(trees) + 1
			for i, tree := range trees {
				if tree.Size() != expectedNodesPerTree && tree.Size() != expectedNodesPerTree+1 {
					t.Fatal("The subtree", i, "should contain", expectedNodesPerTree, "nodes, but contains", tree.Size(), "nodes")
				}
				totalNodes += tree.Size() - 1 //to account for shared leader
			}
			if totalNodes != nNodes {
				t.Fatal("Trees should in total contain", nNodes, "nodes, but they contain", totalNodes, "nodes")
			}
			local.CloseAll()
		}
	}
}

//tests that the generated tree has the good number of subtrees
func TestGenTreesSubtrees(t *testing.T) {

	nodes := []int{1, 2, 5, 20}
	subtrees := []int{1, 5, 12}
	for _, nNodes := range nodes {
		for _, nSubtrees := range subtrees {

			wantedSubtrees := nSubtrees
			if nNodes <= nSubtrees {
				wantedSubtrees = nNodes - 1
				if wantedSubtrees < 1 {
					wantedSubtrees = 1
				}
			}

			local := onet.NewLocalTest(testSuite)
			servers := local.GenServers(nNodes)
			roster := local.GenRosterFromHost(servers...)

			trees, err := GenTrees(roster, nNodes, nSubtrees)
			if err != nil {
				t.Fatal("Error in tree generation:", err)
			}
			actualSubtrees := len(trees)
			if actualSubtrees != wantedSubtrees {
				t.Fatal("There should be", wantedSubtrees, "subtrees, but there is", actualSubtrees, "subtrees")
			}
			local.CloseAll()
		}
	}
}

// tests the second and third level of all trees
func TestGenTreesComplete(t *testing.T) {
	nodes := []int{1, 2, 5, 20}
	subtrees := []int{1, 5, 12}
	for _, nNodes := range nodes {
		for _, nSubtrees := range subtrees {
			local := onet.NewLocalTest(testSuite)
			servers := local.GenServers(nNodes)
			roster := local.GenRosterFromHost(servers...)

			trees, err := GenTrees(roster, nNodes, nSubtrees)
			if err != nil {
				t.Fatal("Error in tree generation:", err)
			}

			nodesDepth2 := ((nNodes - 1) / nSubtrees) - 1
			for _, tree := range trees {
				if tree.Size() < 2 {
					// local.CloseAll()
					continue
				}
				subleader := tree.Root.Children[0]
				if len(subleader.Children) < nodesDepth2 || len(subleader.Children) > nodesDepth2+1 {
					t.Fatal(nNodes, "node(s),", nSubtrees, "subtrees: There should be",
						nodesDepth2, "to", nodesDepth2+1, "second level node(s),"+
							" but there is a subtree with", len(subleader.Children), "second level node(s).")
				}
				testNode(t, subleader, tree.Root, tree)
				for _, m := range subleader.Children {
					if len(m.Children) > 0 {
						t.Fatal("the tree should be at most 2 level deep, but is not")
					}
					testNode(t, m, subleader, tree)
				}
			}
			local.CloseAll()
		}
	}
}

//global tests to be performed on every node,
func testNode(t *testing.T, node, parent *onet.TreeNode, tree *onet.Tree) {
	if node.Parent != parent {
		t.Fatal("a node has not the right parent in the field \"parent\"")
	}
	addr, _ := tree.Roster.Search(node.ServerIdentity.ID)
	if addr == -1 {
		t.Fatal("a node in the tree is runing on a server that is not in the tree's roster")
	}
}

//tests that the GenTree function returns errors correctly
func TestGenTreesErrors(t *testing.T) {
	negativeNumbers := []int{0, -1, -2, -12, -34}
	positiveNumber := 12
	for _, negativeNumber := range negativeNumbers {
		local := onet.NewLocalTest(testSuite)
		servers := local.GenServers(positiveNumber)
		roster := local.GenRosterFromHost(servers...)

		trees, err := GenTrees(roster, negativeNumber, positiveNumber)
		if err == nil {
			t.Fatal("the GenTree function should throw an error" +
				" with negative number of nodes, but doesn't")
		}
		if trees != nil {
			t.Fatal("the GenTree function should return a nil tree" +
				" with errors, but doesn't")
		}

		trees, err = GenTrees(roster, positiveNumber, negativeNumber)
		if err == nil {
			t.Fatal("the GenTree function should throw an error" +
				" with negative number of subtrees, but doesn't")
		}
		if trees != nil {
			t.Fatal("the GenTree function should return a nil tree" +
				" with errors, but doesn't")
		}

		local.CloseAll()
	}
}

//tests that the GenTree function returns roster errors correctly
func TestGenTreesRosterErrors(t *testing.T) {
	local := onet.NewLocalTest(testSuite)

	trees, err := GenTrees(nil, 12, 3)
	if err == nil {
		t.Fatal("the GenTree function should throw an error" +
			" with an nil roster, but doesn't")
	}
	if trees != nil {
		t.Fatal("the GenTree function should return a nil tree" +
			" with errors, but doesn't")
	}

	servers := local.GenServers(2)
	roster := local.GenRosterFromHost(servers...)

	trees, err = GenTrees(roster, 12, 3)
	if err == nil {
		t.Fatal("the GenTree function should throw an error" +
			" with a roster containing less servers than the number of nodes, but doesn't")
	}
	if trees != nil {
		t.Fatal("the GenTree function should return a nil tree" +
			" with errors, but doesn't")
	}

	local.CloseAll()
}

//tests that the GenTree function uses as many different servers from the roster as possible
func TestGenTreesUsesWholeRoster(t *testing.T) {

	servers := []int{5, 13, 20}
	nNodes := 5
	for _, nServers := range servers {

		local := onet.NewLocalTest(testSuite)
		servers := local.GenServers(nServers)
		roster := local.GenRosterFromHost(servers...)

		trees, err := GenTrees(roster, nNodes, 4)
		if err != nil {
			t.Fatal("Error in tree generation:", err)
		}

		serverSet := make(map[*network.ServerIdentity]bool)
		expectedUsedServers := nNodes
		if nServers < nNodes {
			expectedUsedServers = nServers
		}

		//get all the used serverIdentities
		for _, tree := range trees {
			serverSet[tree.Root.ServerIdentity] = true
			if tree.Size() > 1 {
				subleader := tree.Root.Children[0]
				serverSet[subleader.ServerIdentity] = true
				for _, m := range subleader.Children {
					serverSet[m.ServerIdentity] = true
				}
			}
		}

		if len(serverSet) != expectedUsedServers {
			t.Fatal("the generated tree should use", expectedUsedServers,
				"different servers but uses", len(serverSet))
		}

		local.CloseAll()
	}
}

//tests that the subtree generator puts the correct subleader in place
func TestGenSubtreePutsCorrectSubleader(t *testing.T) {

	nodes := []int{2, 5, 20}
	subleaderIDs := []int{1, 2, 3, 12}
	for _, nNodes := range nodes {
		for _, subleaderID := range subleaderIDs {

			local := onet.NewLocalTest(testSuite)
			servers := local.GenServers(nNodes)
			roster := local.GenRosterFromHost(servers...)

			tree, err := GenSubtree(roster, subleaderID)

			if subleaderID >= nNodes { //should generate an error
				if err == nil {
					t.Fatal("subtree generation should return an error with a subleader id " +
						"that is greater or equal to the number of nodes, but doesn't")
				} else { // correctly generates error
					local.CloseAll()
					continue
				}
			} else if err != nil {
				t.Fatal("error in subtree generation:", err)
			}

			if len(tree.Root.Children) != 1 {
				t.Fatal("subtree should have exactly one subleader, but has", len(tree.Root.Children))
			}

			subleader := tree.Root.Children[0]

			if subleader.ServerIdentity.ID != roster.List[subleaderID].ID {
				t.Fatal("the subtree should have the node", subleaderID, "as subleader, but doesn't")
			}
			local.CloseAll()
		}
	}
}

// Tests that the subtree generator returns the correct structure
// that is a root, with one child and all other nodes as this child' children
func TestGenSubtreeStructure(t *testing.T) {

	nodes := []int{2, 5, 20}
	subleaderID := 1
	for _, nNodes := range nodes {

		local := onet.NewLocalTest(testSuite)
		servers := local.GenServers(nNodes)
		roster := local.GenRosterFromHost(servers...)

		tree, err := GenSubtree(roster, subleaderID)
		if err != nil {
			t.Fatal("error in subtree generation:", err)
		}

		if tree.Size() != nNodes {
			t.Fatal("the subtree should contain", nNodes, "nodes, but contains", tree.Size())
		}
		if len(tree.Root.Children) != 1 {
			t.Fatal("subtree should have exactly one subleader, but has", len(tree.Root.Children))
		}
		nLeaves := len(tree.Root.Children[0].Children)
		if nLeaves != nNodes-2 {
			t.Fatal("subtree should have", nNodes-2, "leaves, but has", nLeaves)
		}

		local.CloseAll()
	}
}

// Tests that the subtree generator throws errors with invalid parameters
func TestGenSubtreeErrors(t *testing.T) {

	nodes := []int{2, 5, 20}
	correctSubleaderID := 1
	for _, nNodes := range nodes {

		local := onet.NewLocalTest(testSuite)
		servers := local.GenServers(nNodes)
		roster := local.GenRosterFromHost(servers...)

		_, err := GenSubtree(roster, -5)
		if err == nil {
			t.Fatal("subtree generator should throw an error with a negative subleader id, but doesn't")
		}

		_, err = GenSubtree(roster, 0)
		if err == nil {
			t.Fatal("subtree generator should throw an error with a zero subleader id, but doesn't")
		}

		_, err = GenSubtree(roster, nNodes)
		if err == nil {
			t.Fatal("subtree generator should throw an error with a too big subleader id, but doesn't")
		}

		_, err = GenSubtree(nil, correctSubleaderID)
		if err == nil {
			t.Fatal("subtree generator should throw an error with a nil roster, but doesn't")
		}

		emptyRoster := local.GenRosterFromHost(make([]*onet.Server, 0)...)
		_, err = GenSubtree(emptyRoster, correctSubleaderID)
		if err == nil {
			t.Fatal("subtree generator should throw an error with a nil roster, but doesn't")
		}

		local.CloseAll()
	}
}
