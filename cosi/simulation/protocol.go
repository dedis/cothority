package main

import (
	p "go.dedis.ch/cothority/v4/cosi/protocol"
	"go.dedis.ch/kyber/v4"
	"go.dedis.ch/onet/v4"
	"go.dedis.ch/onet/v4/log"
)

/*
This is the CoSi-protocol for simulation which supports
 verification at different level.
*/

// Name can be used to reference the registered protocol.
var Name = "CoSimul"

func init() {
	onet.GlobalProtocolRegister(Name, NewCoSimul)
}

// VRType defines what verifications are done
// see https://github.com/dedis/cothority/issues/260
type VRType int

const (
	// NoCheck will do no check at all
	NoCheck = VRType(0)
	// RootCheck will check only at root
	RootCheck = VRType(1)
	// AllCheck check at each level of the tree, except the leafs
	AllCheck = VRType(2)
)

// CoSimul is a protocol suited for simulation
type CoSimul struct {
	*p.CoSi
	// VerifyResponse sets how the checks are done,
	VerifyResponse VRType
}

// NewCoSimul returns a new CoSi-protocol suited for simulation
func NewCoSimul(node *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	c, err := p.NewProtocol(node)
	if err != nil {
		return nil, err
	}

	cosimul := &CoSimul{c.(*p.CoSi), RootCheck}
	cosimul.RegisterResponseHook(cosimul.getResponse)

	return cosimul, nil
}

// Publics returns an array of public points for the signature- and
// verification method
func (c *CoSimul) Publics() []kyber.Point {
	var publics []kyber.Point
	for _, e := range c.Tree().Roster.List {
		publics = append(publics, e.Public)
	}
	return publics
}

func (c *CoSimul) getResponse(in []kyber.Scalar) {
	if c.IsLeaf() {
		// This is the leaf-node and we can't verify it
		return
	}

	verify := false
	switch c.VerifyResponse {
	case NoCheck:
		log.Lvl3("Not checking at all")
	case RootCheck:
		verify = c.IsRoot()
	case AllCheck:
		verify = !c.IsLeaf()
	}

	if verify {
		err := c.VerifyResponses(c.TreeNode().AggregatePublic(c.Suite()))
		if err != nil {
			log.Error("Couldn't verify responses at our level", c.Name(), err.Error())
		} else {
			log.Lvl2("Successfully verified responses at", c.Name())
		}
	}
}
