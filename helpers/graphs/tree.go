package graphs

import (
	"encoding/hex"
	"fmt"

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

func (t *Tree) TraverseTree(f func(*Tree)) {
	f(t)
	for _, c := range t.Children {
		c.TraverseTree(f)
	}
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

func PrintTreeNode(t *Tree) {
	fmt.Println(t.Name)

	for _, c := range t.Children {
		fmt.Println("\t", c.Name)
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
