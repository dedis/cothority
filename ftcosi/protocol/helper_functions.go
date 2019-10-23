package protocol

import (
	"fmt"

	"go.dedis.ch/kyber/v4"
	"go.dedis.ch/kyber/v4/sign/cosi"
	"go.dedis.ch/onet/v4"
	"go.dedis.ch/onet/v4/log"
	"go.dedis.ch/onet/v4/network"
)

// aggregateCommitments returns an aggregated commitment and an aggregated mask
func aggregateCommitments(s cosi.Suite, publics []kyber.Point,
	structCommitments []StructCommitment) (kyber.Point, *cosi.Mask, error) {

	if publics == nil {
		return nil, nil, fmt.Errorf("publics should not be nil, but is")
	} else if structCommitments == nil {
		return nil, nil, fmt.Errorf("structCommitments should not be nil, but is")
	}

	//extract lists of commitments and masks
	var commitments []kyber.Point
	var masks [][]byte
	for _, c := range structCommitments {
		commitments = append(commitments, c.CoSiCommitment)
		masks = append(masks, c.Mask)
	}

	//create final aggregated mask
	finalMask, err := cosi.NewMask(s, publics, nil)
	if err != nil {
		return nil, nil, err
	}

	aggCommitment := s.Point().Null()
	aggMask := finalMask.Mask()
	if len(masks) > 0 {
		//aggregate commitments and masks
		aggCommitment, aggMask, err =
			cosi.AggregateCommitments(s, commitments, masks)
		if err != nil {
			return nil, nil, err
		}
	}

	err = finalMask.SetMask(aggMask)
	if err != nil {
		return nil, nil, err
	}

	return aggCommitment, finalMask, nil
}

// generateResponse generates a personal response based on the secret
// and returns the aggregated response of all children and the node
func aggregateResponses(s cosi.Suite, structResponses []StructResponse) (kyber.Scalar, error) {

	if structResponses == nil {
		return nil, fmt.Errorf("StructResponse should not be nil, but is")
	}

	// extract lists of responses
	var responses []kyber.Scalar
	for _, c := range structResponses {
		responses = append(responses, c.CoSiReponse)
	}

	// aggregate responses
	aggResponse, err := cosi.AggregateResponses(s, responses)
	if err != nil {
		log.Lvl3("failed to create aggregate response")
		return nil, err
	}

	log.Lvl3("done aggregating responses with total of", len(responses), "responses")
	return aggResponse, nil
}

// GetSubleaderIDs returns a slice of subleaders for tree
func GetSubleaderIDs(tree *onet.Tree, root, nNodes, nSubtrees int) ([]network.ServerIdentityID, error) {
	exampleTrees, err := genTrees(tree.Roster, root, nNodes, nSubtrees)
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
func GetLeafsIDs(tree *onet.Tree, root, nNodes, nSubtrees int) ([]network.ServerIdentityID, error) {
	exampleTrees, err := genTrees(tree.Roster, root, nNodes, nSubtrees)
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
