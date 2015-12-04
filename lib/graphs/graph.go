package graphs

import (
	"bytes"
	"container/list"
	"errors"
	"github.com/dedis/cothority/lib/dbg"
	"net"
	"strconv"
)

var TRIM bool = false

type Graph struct {
	Names   []string
	mem     []float64 // underlying memory for the weights matrix
	Weights [][]float64
}

func NewGraph(names []string) *Graph {
	n := len(names)
	g := &Graph{}
	g.Names = make([]string, len(names))
	copy(g.Names, names[:])
	g.mem = make([]float64, n*n)
	mem := g.mem
	g.Weights = make([][]float64, n)
	for i := range g.Weights {
		g.Weights[i], mem = mem[:n], mem[n:]
	}
	return g
}

// takes in a byte array representing an edge list and loads the graph
func (g *Graph) LoadEdgeList(edgelist []byte) {
	dbg.Lvl3(g.Names)
	fields := bytes.Fields(edgelist)
	// create name map from string to index
	dbg.Lvl3(g.Names)
	names := make(map[string]int)
	for i, n := range g.Names {
		names[n] = i
	}

	// read fields in groups of three: from, to, edgeweight
	for i := 0; i < len(fields)-2; i += 3 {
		from := string(fields[i])
		to := string(fields[i+1])
		weight, err := strconv.ParseFloat(string(fields[i+2]), 64)
		if err != nil {
			dbg.Lvl3(err)
			continue
		}
		fi, ok := names[from]
		if !ok {
			dbg.Lvl3("from not ok:", from)
			continue
		}

		ti, ok := names[to]
		if !ok {
			dbg.Lvl3("to not ok:", to)
			continue
		}

		g.Weights[fi][ti] = weight
	}
}

func (g *Graph) MST() *Tree {
	// select lowest weighted root
	root := &Tree{}
	return root
}

// breadth first_ish
// pi: parent index, bf: branching factor, visited: set of visited nodes, ti: tree index, tnodes: space for tree nodes
// returns the last used index for tree nodes
func (g *Graph) constructTree(ri int, bf int, visited []bool, tnodes []Tree) {
	dbg.Lvl3("constructing tree:", ri, bf)
	dbg.Lvl3(g.Names)
	root := &tnodes[ri]
	root.Name = g.Names[ri]
	dbg.Lvl3(root)
	visited[ri] = true
	tni := 1
	indmap := make([]int, len(visited))
	indmap[ri] = 0

	// queue for breadth first search
	queue := list.New()

	// push the root first
	queue.PushFront(ri)

	// has to iterate through all the nodes
	for {
		e := queue.Back()
		// have processed all values
		if e == nil {
			break
		}
		queue.Remove(e)
		// parent index
		pi := e.Value.(int)
		dbg.Lvl3("next:", pi)
		parent := &tnodes[indmap[pi]]

		fs := sortFloats(g.Weights[pi])
		nc := bf
		dbg.Lvl3(fs)
		// iterate through children and select the bf closest ones
		for _, ci := range fs.I {
			if nc == 0 {
				break
			}

			// if this child hasn't been visited
			// it is the closest unvisited child
			if !visited[ci] {
				dbg.Lvl3("adding child:", ci, tni)
				queue.PushFront(ci)
				cn := &tnodes[tni]
				indmap[ci] = tni
				cn.Name = g.Names[ci]
				tni++
				parent.Children = append(parent.Children, cn)
				visited[ci] = true
				nc--
			}
		}
	}
}

// nlevels : [0:n-1]
func (g *Graph) Tree(nlevels int) *Tree {
	// find node with lowest weights outbound and inbound
	n := len(g.Weights)

	tnodes := make([]Tree, n)
	root := &tnodes[0]
	ri := g.BestConnector()
	root.Name = g.Names[ri]
	if nlevels == 0 {
		return root
	}

	// find the branching factor needed
	bf := n / nlevels
	if n%nlevels != 0 {
		bf += 1
	}
	// log.Panicf("n: %d, nlevels: %d, branching factor: %d\n", n, nlevels, bf)
	dbg.Lvl3("Tree:", n, nlevels, bf)
	g.constructTree(ri, bf, make([]bool, n), tnodes)
	dbg.Lvl3("tnodes:", tnodes)
	return root
}

// return the index of the best connector
func (g *Graph) BestConnector() int {
	n := len(g.Weights)
	dist := make([]float64, n)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			dist[i] += g.Weights[i][j] + g.Weights[j][i]
		}
	}
	index := 0
	min := dist[0]
	for i := 1; i < n; i++ {
		if dist[i] < min {
			index = i
			min = dist[i]
		}
	}
	return index
}

var ErrNoNodesGiven = errors.New("No Nodes Given as argument")
var ErrNoHosts = errors.New("Can't have 0 hosts per node")

// Given a list of machine names, a number of hosts per machine(node),
// and a branching factor
// Return a Tree that meets the branching factor restriction, and
// has no two adjacent nodes in the Tree running as hosts on the same machine
// Also returns a list of the host names created and their ports
func TreeFromList(nodeNames []string, hostsPerNode int, bf int, startMachine ...string) (
	*Tree, []string, int, error) {
	if len(nodeNames) < 1 {
		return nil, nil, 0, ErrNoNodesGiven
	}

	if hostsPerNode <= 0 {
		// TODO: maybe something else is desired here
		return nil, nil, 0, ErrNoHosts
	}

	// Hosts on one machine get ports starting with StartPort
	// and distanced 10 away
	StartPort := 2000

	// Map from nodes to their hosts
	mp := make(map[string][]string)

	// Generate host names
	hostAddr := make([]string, 0)
	for _, nodeName := range nodeNames {
		mp[nodeName] = make([]string, 0)
		for i := 0; i < hostsPerNode; i++ {
			addr := net.JoinHostPort(nodeName, strconv.Itoa(StartPort+i*10))
			hostAddr = append(hostAddr, addr)

			mp[nodeName] = append(mp[nodeName], addr)
		}
	}

	var startM string
	if len(startMachine) > 0 {
		startM = startMachine[0]
	} else {
		startM = nodeNames[0]
	}

	root, usedHostAddr, err := ColorTree(nodeNames, hostAddr, hostsPerNode, bf, startM, mp)
	return root, usedHostAddr, Depth(root), err
}

func ColorTree(nodeNames []string, hostAddr []string, hostsPerNode int, bf int, startM string, mp map[string][]string) (
	*Tree, []string, error) {

	if hostsPerNode <= 0 {
		// TODO: maybe something else is desired here
		return nil, nil, ErrNoHosts
	}

	nodesTouched := make([]string, 0)
	nodesTouched = append(nodesTouched, startM)

	rootHost := mp[startM][0]
	mp[startM] = mp[startM][1:]

	hostsCreated := make([]string, 0)
	hostsCreated = append(hostsCreated, rootHost)
	depth := make([]int, 0)
	depth = append(depth, 1)

	hostTNodes := make([]*Tree, 0)
	rootTNode := &Tree{Name: rootHost}
	hostTNodes = append(hostTNodes, rootTNode)

	for i := 0; i < len(hostsCreated); i++ {
		curHost := hostsCreated[i]
		curDepth := depth[i]
		curTNode := hostTNodes[i]
		curNode, _, _ := net.SplitHostPort(curHost)

		curTNode.Children = make([]*Tree, 0)

		for c := 0; c < bf; c++ {
			var newHost string
			nodesTouched, mp, newHost = GetFirstFreeNode(nodesTouched, mp, curNode)
			if newHost == "" {
				if TRIM == true {
					rootTNode, hostsCreated = TrimLastIncompleteLevel(rootTNode, hostsCreated, depth, bf)
				}
				return rootTNode, hostsCreated, nil
				// break
			}

			// create Tree Node for the new host
			newHostTNode := &Tree{Name: newHost}
			curTNode.Children = append(curTNode.Children, newHostTNode)

			// keep track of created hosts and nodes
			hostsCreated = append(hostsCreated, newHost)
			depth = append(depth, curDepth+1)
			hostTNodes = append(hostTNodes, newHostTNode)

			// keep track of machines used in FIFO order
			node, _, _ := net.SplitHostPort(newHost)
			nodesTouched = append(nodesTouched, node)
		}
		// dbg.Lvl3(i, hostsCreated)
	}

	if TRIM == true {
		rootTNode, hostsCreated = TrimLastIncompleteLevel(rootTNode, hostsCreated, depth, bf)
	}
	return rootTNode, hostsCreated, nil
}

// Go through list of nodes(machines) and choose a hostName on the first node that
// still has room for more hosts on it and is != curNode
// If such a machine does not exist, loop through the map from machine names
// to their available host names, and choose a name on the first free machine != curNode
// Return updated nodes and map (completely full nodes are deleted ) and the chosen hostName
func GetFirstFreeNode(nodes []string, mp map[string][]string, curNode string) (
	[]string, map[string][]string, string) {
	var chosen string
	uNodes := make([]string, 0)

	// loop through recently selected machines that already have hosts
	var i int
	var node string
	for i, node = range nodes {
		if node != curNode {
			if len(mp[node]) > 0 {
				// choose hostname on this node
				chosen = mp[node][0]
				mp[node] = mp[node][1:]

				// if still not fully used, add to updated nodes
				if len(mp[node]) > 0 {
					uNodes = append(uNodes, node)
				}
				break
			} else {
				// remove full node
				delete(mp, node)
			}
		}
	}

	// keep in list nodes after chosen node
	for ; i < len(nodes); i++ {
		uNodes = append(uNodes, node)
	}

	if chosen != "" {
		// we were able to make a choice
		// but all recently seen nodes before 'i' were fully used
		return uNodes, mp, chosen
	}

	// all recently seen nodes were fully used or == curNode
	// must choose free machine from map
	for node := range mp {
		if node != curNode {
			if len(mp[node]) > 0 {
				chosen = mp[node][0]
				mp[node] = mp[node][1:]
				break
			}
		}
	}

	return uNodes, mp, chosen
}

func TrimLastIncompleteLevel(root *Tree, hosts []string, depths []int, bf int) (*Tree, []string) {
	treed := Depth(root)

	var sj, j int
	n := len(hosts)

	expectedNNodes := 1
	// if there is any incomplete tree level it should be treed
	// if it's not treed, then the tree should be fully filled for its depth
	// we check that this is the case and if treed is incomplete
	// we remove the nodes and hosts from the treed level
	lastLevel := treed
	for d := 1; d <= treed; d++ {
		nNodes := 0
		sj = j
		for ; j < n && depths[j] == d; j++ {
			nNodes++
		}

		if nNodes != expectedNNodes {
			lastLevel = d
			break
		}
		expectedNNodes *= bf
	}

	if lastLevel != treed {
		panic("Incomplete level is not last tree level" + strconv.Itoa(lastLevel) + " " + strconv.Itoa(treed))
	}

	bhMap := make(map[string]bool)
	badHosts := hosts[sj:]
	for _, bh := range badHosts {
		bhMap[bh] = true
	}
	newRoot := &Tree{Name: root.Name}
	TrimTree(newRoot, root, bhMap)

	d := Depth(newRoot)
	if len(badHosts) != 0 && d != treed-1 {
		dbg.Lvl3(d, "!=", treed-1)
		panic("TrimTree return wrong result")
	} else {
		if len(badHosts) == 0 && d != treed {
			dbg.Lvl3(d, "!=", treed)
			panic("TrimTree return wrong result")
		}
	}

	// dbg.Lvl3("			Trimmed", n-sj, "nodes")
	return newRoot, hosts[:sj]

}

func TrimTree(newRoot *Tree, oldRoot *Tree, bhMap map[string]bool) {
	for _, c := range oldRoot.Children {
		if _, bad := bhMap[c.Name]; !bad {
			newNode := &Tree{Name: c.Name}
			newRoot.Children = append(newRoot.Children, newNode)

			TrimTree(newNode, c, bhMap)
		}
	}

}
