package protocol

import (
	"errors"
	"fmt"

	"github.com/dedis/kyber"
	"github.com/dedis/kyber/pairing"
	"github.com/dedis/kyber/sign/bls"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"github.com/dedis/onet/simul/monitor"
)

// Sign the message with this node and aggregates with all child signatures (in structResponses)
// Also aggregates the child bitmasks
func generateSignature(ps pairing.Suite, t *onet.TreeNodeInstance, publics []kyber.Point, private kyber.Scalar, structResponses []StructResponse,
	msg []byte, ok bool) (kyber.Point, *Mask, error) {

	if t == nil {
		return nil, nil, fmt.Errorf("TreeNodeInstance should not be nil, but is")
	} else if structResponses == nil {
		return nil, nil, fmt.Errorf("StructResponse should not be nil, but is")
	} else if publics == nil {
		return nil, nil, fmt.Errorf("publics should not be nil, but is")
	} else if msg == nil {
		return nil, nil, fmt.Errorf("msg should not be nil, but is")
	}

	// extract lists of responses
	var signatures []kyber.Point
	var masks [][]byte

	for _, r := range structResponses {
		atmp, err := signedByteSliceToPoint(ps, r.CoSiReponse)
		_ = err
		signatures = append(signatures, atmp)
		masks = append(masks, r.Mask)
	}

	//generate personal mask
	var personalMask *Mask
	if ok {
		personalMask, _ = NewMask(ps, publics, t.Index())
	} else {
		personalMask, _ = NewMask(ps, publics, -1)
	}

	masks = append(masks, personalMask.Mask())

	// generate personal signature and append to other sigs
	personalSig, err := bls.Sign(ps, private, msg)

	if err != nil {
		return nil, nil, err
	}
	personalPointSig, err := signedByteSliceToPoint(ps, personalSig)
	if !ok {
		personalPointSig = ps.G1().Point()
	}

	signatures = append(signatures, personalPointSig)

	// Aggregate all signatures
	aggSignature, aggMask, err := aggregateSignatures(ps, signatures, masks)
	if err != nil {
		log.Lvl3(t.ServerIdentity().Address, "failed to create aggregate signature")
		return nil, nil, err
	}

	//create final aggregated mask
	finalMask, err := NewMask(ps, publics, -1)
	if err != nil {
		return nil, nil, err
	}
	err = finalMask.SetMask(aggMask)
	if err != nil {
		return nil, nil, err
	}

	log.Lvl3(t.ServerIdentity().Address, "is done aggregating signatures with total of", len(signatures), "signatures")

	return aggSignature, finalMask, nil
}

func signedByteSliceToPoint(ps pairing.Suite, sig []byte) (kyber.Point, error) {
	pointSig := ps.G1().Point()

	if err := pointSig.UnmarshalBinary(sig); err != nil {
		return nil, err
	}

	return pointSig, nil
}

func PointToByteSlice(ps pairing.Suite, sig kyber.Point) ([]byte, error) {
	byteSig, err := sig.MarshalBinary()
	if err != nil {
		return nil, err
	}

	return byteSig, nil
}

func publicByteSliceToPoint(ps pairing.Suite, public []byte) (kyber.Point, error) {
	pointPublic := ps.G2().Point()

	if err := pointPublic.UnmarshalBinary(public); err != nil {
		return nil, err
	}

	return pointPublic, nil

}

func PublicKeyToByteSlice(ps pairing.Suite, public kyber.Point) ([]byte, error) {
	bytePublic, err := public.MarshalBinary()
	if err != nil {
		return nil, err
	}

	return bytePublic, nil
}

func privateByteSliceToScalar(ps pairing.Suite, private []byte) (kyber.Scalar, error) {
	scalarPrivate := ps.G2().Scalar()
	if err := scalarPrivate.UnmarshalBinary(private); err != nil {
		return nil, err
	}
	return scalarPrivate, nil
}

func PrivateKeyToByteSlice(ps pairing.Suite, private kyber.Scalar) ([]byte, error) {
	bytePrivate, err := private.MarshalBinary()
	if err != nil {
		return nil, err
	}
	return bytePrivate, nil
}

func aggregateResponses(ps pairing.Suite, publics []kyber.Point,
	structResponses []StructResponse) (kyber.Point, *Mask, error) {
	if publics == nil {
		return nil, nil, fmt.Errorf("publics should not be nil, but is")
	} else if structResponses == nil {
		return nil, nil, fmt.Errorf("structCommitments should not be nil, but is")
	}

	// extract lists of responses and masks
	var signatures []kyber.Point
	var masks [][]byte

	for _, r := range structResponses {
		atmp, err := signedByteSliceToPoint(ps, r.CoSiReponse)
		_ = err
		signatures = append(signatures, atmp)
		masks = append(masks, r.Mask)
	}

	// create final aggregated mask
	finalMask, err := NewMask(ps, publics, -1)
	if err != nil {
		return nil, nil, err
	}

	aggResponse := ps.G1().Point()
	aggMask := finalMask.Mask()
	if len(masks) > 0 {
		//aggregate responses and masks
		aggResponse, aggMask, err = aggregateSignatures(ps, signatures, masks)
		if err != nil {
			return nil, nil, err
		}
	}

	err = finalMask.SetMask(aggMask)
	if err != nil {
		return nil, nil, err
	}
	return aggResponse, finalMask, nil
}

// AggregateResponses returns the sum of given responses.
// TODO add mask data?
func aggregateSignatures(ps pairing.Suite, signatures []kyber.Point, masks [][]byte) (sum kyber.Point, sig []byte, err error) {
	if signatures == nil {
		return nil, nil, fmt.Errorf("no signatures provided")
	}
	aggMask := make([]byte, len(masks[0]))
	r := ps.G1().Point()

	for i, signature := range signatures {

		r = r.Add(r, signature)
		aggMask, err = AggregateMasks(aggMask, masks[i])
		if err != nil {
			return nil, nil, err
		}
	}
	return r, aggMask, nil
}

func AppendSigAndMask(signature []byte, mask *Mask) []byte {
	return append(signature, mask.mask...)
}

// Verify checks the given cosignature on the provided message using the list
// of public keys and cosigning policy.
func Verify(ps pairing.Suite, publics []kyber.Point, message, sig []byte, policy Policy) error {
	if publics == nil {
		return errors.New("no public keys provided")
	}
	if message == nil {
		return errors.New("no message provided")
	}
	if sig == nil {
		return errors.New("no signature provided")
	}

	lenCom := ps.G1().PointLen()
	signature := sig[:lenCom]

	// Unpack the participation mask and get the aggregate public key
	mask, err := NewMask(ps, publics, -1)
	if err != nil {
		return err
	}

	mask.SetMask(sig[lenCom:])

	pks := mask.AggregatePublic

	err = bls.Verify(ps, pks, message, signature)
	if err != nil {
		return fmt.Errorf("didn't get a valid signature: %s", err)
	} else {
		log.Lvl1("Signature verified and is correct!")
	}

	log.Lvl1("m.CountEnabled():", mask.CountEnabled())
	monitor.RecordSingleMeasure("correct_nodes", float64(mask.CountEnabled()))

	if !policy.Check(mask) {
		return errors.New("the policy is not fulfilled")
	}

	return nil
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
