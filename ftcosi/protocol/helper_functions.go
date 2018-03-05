package protocol

import (
	"fmt"

	"gopkg.in/dedis/kyber.v2"
	"gopkg.in/dedis/kyber.v2/sign/cosi"
	"gopkg.in/dedis/onet.v2"
	"gopkg.in/dedis/onet.v2/log"
	"gopkg.in/dedis/onet.v2/network"
)

// generateCommitmentAndAggregate generates a personal secret and commitment
// and returns respectively the secret, an aggregated commitment and an aggregated mask
func generateCommitmentAndAggregate(s cosi.Suite, t *onet.TreeNodeInstance, publics []kyber.Point,
	structCommitments []StructCommitment) (kyber.Scalar, kyber.Point, *cosi.Mask, error) {

	if t == nil {
		return nil, nil, nil, fmt.Errorf("TreeNodeInstance should not be nil, but is")
	} else if publics == nil {
		return nil, nil, nil, fmt.Errorf("publics should not be nil, but is")
	} else if structCommitments == nil {
		return nil, nil, nil, fmt.Errorf("structCommitments should not be nil, but is")
	}

	//extract lists of commitments and masks
	var commitments []kyber.Point
	var masks [][]byte
	for _, c := range structCommitments {
		commitments = append(commitments, c.CoSiCommitment)
		masks = append(masks, c.Mask)
	}

	//generate personal secret and commitment
	secret, commitment := cosi.Commit(s)
	commitments = append(commitments, commitment)

	//generate personal mask
	personalMask, err := cosi.NewMask(s, publics, t.Public())
	if err != nil {
		return nil, nil, nil, err
	}
	masks = append(masks, personalMask.Mask())

	//aggregate commitments and masks
	aggCommitment, aggMask, err :=
		cosi.AggregateCommitments(s, commitments, masks)
	if err != nil {
		return nil, nil, nil, err
	}

	//create final aggregated mask
	finalMask, err := cosi.NewMask(s, publics, nil)
	if err != nil {
		return nil, nil, nil, err
	}
	err = finalMask.SetMask(aggMask)
	if err != nil {
		return nil, nil, nil, err
	}

	return secret, aggCommitment, finalMask, nil
}

// generateResponse generates a personal response based on the secret
// and returns the aggregated response of all children and the node
func generateResponse(s cosi.Suite, t *onet.TreeNodeInstance, structResponses []StructResponse,
	secret kyber.Scalar, challenge kyber.Scalar) (kyber.Scalar, error) {

	if t == nil {
		return nil, fmt.Errorf("TreeNodeInstance should not be nil, but is")
	} else if structResponses == nil {
		return nil, fmt.Errorf("StructResponse should not be nil, but is")
	} else if secret == nil {
		return nil, fmt.Errorf("secret should not be nil, but is")
	} else if challenge == nil {
		return nil, fmt.Errorf("challenge should not be nil, but is")
	}

	// extract lists of responses
	var responses []kyber.Scalar
	for _, c := range structResponses {
		responses = append(responses, c.CoSiReponse)
	}

	// generate personal response
	personalResponse, err := cosi.Response(s, t.Private(), secret, challenge)
	if err != nil {
		return nil, err
	}
	responses = append(responses, personalResponse)
	log.Lvl3(t.ServerIdentity().Address, "Verification successful")

	// aggregate responses
	aggResponse, err := cosi.AggregateResponses(s, responses)
	if err != nil {
		log.Lvl3(t.ServerIdentity().Address, "failed to create aggregate response")
		return nil, err
	}

	log.Lvl3(t.ServerIdentity().Address, "is done aggregating responses with total of",
		len(responses), "responses")
	return aggResponse, nil
}

// GetSubleaderIDs returns a slice of subleaders for tree
func GetSubleaderIDs(tree *onet.Tree, nNodes, nSubtrees int) ([]network.ServerIdentityID, error) {
	exampleTrees, err := genTrees(tree.Roster, nNodes, nSubtrees)
	if err != nil {
		return nil, fmt.Errorf("error in creation of example tree:%s", err)
	}
	subleadersIDs := make([]network.ServerIdentityID, 0)
	for _, subtree := range exampleTrees {
		if len(subtree.Root.Children) < 1 {
			return nil, fmt.Errorf("expected a subtree with at least a subleader, but found none")
		}
		subleadersIDs = append(subleadersIDs, subtree.Root.Children[0].ServerIdentity.ID)
	}
	return subleadersIDs, nil
}

// GetLeafsIDs returns a slice of leaves for tree
func GetLeafsIDs(tree *onet.Tree, nNodes, nSubtrees int) ([]network.ServerIdentityID, error) {
	exampleTrees, err := genTrees(tree.Roster, nNodes, nSubtrees)
	if err != nil {
		return nil, fmt.Errorf("error in creation of example tree:%s", err)
	}
	leafsIDs := make([]network.ServerIdentityID, 0)
	for _, subtree := range exampleTrees {
		if len(subtree.Root.Children) < 1 {
			return nil, fmt.Errorf("expected a subtree with at least a subleader, but found none")
		}
		for _, leaf := range subtree.Root.Children[0].Children {
			leafsIDs = append(leafsIDs, leaf.ServerIdentity.ID)
		}
	}
	return leafsIDs, nil
}
