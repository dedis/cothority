package graphs

import (
	"github.com/dedis/cothority/lib/dbg"
	"strconv"
	"strings"
	"testing"
)

func TestTree(t *testing.T) {
	g := &Graph{Names: []string{"planetlab2.cs.unc.edu", "pl1.6test.edu.cn", "planetlab1.cs.du.edu", "planetlab02.cs.washington.edu", "planetlab-2.cse.ohio-state.edu", "planetlab2.cs.ubc.ca"}, mem: []float64{0, 213.949, 51.86, 76.716, 2754.531, 81.301, 214.143, 0, 169.744, 171.515, 557.526, 189.186, 51.601, 170.191, 0, 41.418, 2444.206, 31.475, 76.731, 171.43, 41.394, 0, 2470.722, 5.741, 349.881, 520.028, 374.362, 407.282, 0, 392.211, 81.381, 189.386, 31.582, 5.78, 141.273, 0}, Weights: [][]float64{[]float64{0, 213.949, 51.86, 76.716, 2754.531, 81.301}, []float64{214.143, 0, 169.744, 171.515, 557.526, 189.186}, []float64{51.601, 170.191, 0, 41.418, 2444.206, 31.475}, []float64{76.731, 171.43, 41.394, 0, 2470.722, 5.741}, []float64{349.881, 520.028, 374.362, 407.282, 0, 392.211}, []float64{81.381, 189.386, 31.582, 5.78, 141.273, 0}}}
	tree := g.Tree(2)
	t.Log(tree)
}

func TestTreeFromList(t *testing.T) {
	nodeNames := make([]string, 0)
	nodeNames = append(nodeNames, "machine0", "machine1", "machine2")
	hostsPerNode := 2
	bf := 2

	root, usedHosts, _, err := TreeFromList(nodeNames, hostsPerNode, bf)
	if err != nil {
		panic(err)
	}

	// JSON format
	// b, err := json.Marshal(root)
	// if err != nil {
	// 	t.Error(err)
	// }
	// t.Log(string(b))

	// if len(usedHosts) != len(nodeNames)*hostsPerNode {
	// 	t.Error("Should have been able to use all hosts")
	// }
	t.Log("used hosts", usedHosts)
	root.TraverseTree(PrintTreeNode)

	// Output:
	// used hosts [machine0:32600 machine1:32600 machine1:32610 machine0:32610 machine2:32600 machine2:32610]
	// machine0:32600
	// 	 machine1:32600
	// 	 machine1:32610
	// machine1:32600
	// 	 machine0:32610
	// 	 machine2:32600
	// machine0:32610
	// machine2:32600
	// machine1:32610
	// 	 machine2:32610
	// machine2:32610
}

//        1
//       /  \
//      2    2
//     / \  / \
//    1  1  1
// two 2s not used to avoid akward tree
func TestTreeFromList2(t *testing.T) {
	// 2 machines with 4 hosts each and branching factor of 2
	// this means only 6 of the 8 hosts should be used, depth will be 2
	nodeNames := make([]string, 0)
	nodeNames = append(nodeNames, "machine0", "machine1")
	hostsPerNode := 4
	bf := 2

	root, usedHosts, _, err := TreeFromList(nodeNames, hostsPerNode, bf)
	if err != nil {
		panic(err)
	}

	if len(usedHosts) != 6 {
		t.Error("Should have been able to use only 6 hosts")
	}
	t.Log("used hosts", usedHosts)
	root.TraverseTree(PrintTreeNode)
}

func checkColoring(t *Tree) bool {
	p := strings.Split(t.Name, ":")[0]
	for _, c := range t.Children {
		h := strings.Split(c.Name, ":")[0]
		if h == p {
			return false
		}
		if !checkColoring(c) {
			return false
		}
	}
	return true
}

func TestTreeFromListColoring(t *testing.T) {
	nodes := make([]string, 0)
	for i := 0; i < 20; i++ {
		nodes = append(nodes, "host"+strconv.Itoa(i))
	}
	for hpn := 1; hpn < 10; hpn++ {
		for bf := 1; bf <= hpn*len(nodes); bf++ {
			dbg.Lvl4("generating tree:", hpn, bf)
			root, hosts, retDepth, err := TreeFromList(nodes, hpn, bf)
			if err != nil {
				panic(err)
			}
			if !checkColoring(root) {
				t.Fatal("failed to properly color:", nodes, hpn, bf)
			}
			dbg.Lvl4("able to use:", len(hosts), "of:", hpn*len(nodes))

			depth := Depth(root)
			if depth != retDepth {
				panic("Returned tree depth != actual treedepth")
			}
			dbg.Lvl4("depth:", depth)
		}
	}
}
