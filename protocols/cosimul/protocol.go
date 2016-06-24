package cosimul

import (
	"github.com/dedis/cosi/protocol"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/crypto/abstract"
	"gopkg.in/dedis/cothority.v0/lib/dbg"
)

/*
This is the CoSi-protocol for simulation which supports
 verification at different level of the tree to being able
 to test at different levels:
 0: not at all
 1: only the root
 2: every node verifies
*/

// Name can be used to reference the registered protocol.
var Name = "CoSimul"

func init() {
	sda.ProtocolRegisterName(Name, NewCoSimul)
}

// VerifyResponse sets how the checks are done,
// see https://github.com/dedis/cothority/issues/260
// 0 - no check at all
// 1 - check only at root
// 2 - check at each level of the tree
var VerifyResponse = 1

// CoSimul is a protocol suited for simulation
type CoSimul struct {
	*protocol.CoSi
}

// NewCoSimul returns a new CoSi-protocol suited for simulation
func NewCoSimul(node *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
	c, err := protocol.NewCoSi(node)
	if err != nil {
		return nil, err
	}

	cosimul := &CoSimul{c.(*protocol.CoSi)}
	cosimul.RegisterResponseHook(cosimul.getResponse)

	return cosimul, nil
}

// Publics returns an array of public points for the signature- and
// verification method
func (c *CoSimul) Publics() []abstract.Point {
	var publics []abstract.Point
	for _, e := range c.Tree().Roster.List {
		publics = append(publics, e.Public)
	}
	return publics
}

func (c *CoSimul) getResponse(in []abstract.Scalar) {
	if c.IsLeaf() {
		// This is the leaf-node and we can't verify it
		return
	}

	verify := false
	switch VerifyResponse {
	case 1:
		verify = c.IsRoot()
	case 2:
		verify = !c.IsLeaf()
	}

	if verify {
		err := c.VerifyResponses(c.TreeNode().AggregatePublic())
		if err != nil {
			dbg.Error("Couldn't verify responses at our level", c.Name(), err.Error())
		} else {
			dbg.Lvl2("Successfully verified responses at", c.Name())
		}
	}
}
