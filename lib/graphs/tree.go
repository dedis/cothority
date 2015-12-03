package graphs

import (
	"encoding/hex"
	"strings"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/crypto/abstract"
)

// tree easy to deal with
// default json encoding can be read as the
// Tree section of a config file
type Tree struct {
	Name string `json:"name"`
	// hex encoded public and private keys
	PriKey   string  `json:"prikey,omitempty"`
	PubKey   string  `json:"pubkey,omitempty"`
	Children []*Tree `json:"children,omitempty"`
}

func (t *Tree) FindByName(name string, depth int) (*Tree, int) {
	if t.Name == name {
		return t, depth
	}
	for _, c := range t.Children {
		ct, d := c.FindByName(name, depth+1)
		if ct != nil {
			return ct, d
		}
	}
	return nil, depth
}

func (t *Tree) TraverseTree(f func(*Tree)) {
	f(t)
	for _, c := range t.Children {
		c.TraverseTree(f)
	}
}

// Simply organizes a list of nodes into a tree with a branching factor = bf
// bfs style
func CreateLocalTree(nodeNames []string, bf int) *Tree {
	if bf < 1 {
		panic("Branching Factor < 1 in CreateLocalTree:/")
	}
	var root *Tree = new(Tree)
	root.Name = nodeNames[0]
	var index int = 1
	bfs := make([]*Tree, 1)
	bfs[0] = root
	for len(bfs) > 0 && index < len(nodeNames) {
		t := bfs[0]
		t.Children = make([]*Tree, 0)
		lbf := 0
		// create space for enough children
		// init them
		for lbf < bf && index < len(nodeNames) {
			child := new(Tree)
			child.Name = nodeNames[index]
			// append the children to the list of trees to visit
			bfs = append(bfs, child)
			t.Children = append(t.Children, child)
			index += 1
			lbf += 1
		}
		bfs = bfs[1:]
	}
	return root
}

// generate keys for the tree
func (t *Tree) GenKeys(suite abstract.Suite, rand abstract.Cipher) {
	t.TraverseTree(func(t *Tree) {
		PrivKey := suite.Secret().Pick(rand)
		PubKey := suite.Point().Mul(nil, PrivKey)
		prk, _ := PrivKey.MarshalBinary()
		pbk, _ := PubKey.MarshalBinary()
		t.PriKey = string(hex.EncodeToString(prk))
		t.PubKey = string(hex.EncodeToString(pbk))
	})
}

func (t *Tree) Visit(fn func(*Tree)) {
	fn(t)
	for _, c := range t.Children {
		c.Visit(fn)
	}
}

func PrintTreeNode(t *Tree) {
	dbg.Lvl3(t.Name)

	for _, c := range t.Children {
		dbg.Lvl3("\t", c.Name)
	}
}

func Depth(t *Tree) int {
	md := 0
	for _, c := range t.Children {
		dc := Depth(c)
		if dc > md {
			md = dc
		}
	}
	return md + 1
}

func (t *Tree) String(depth int) string {
	str := strings.Repeat("\t", depth) + t.Name + "\n"
	for _, c := range t.Children {
		str += c.String(depth + 1)
	}
	return str + "\n"
}
