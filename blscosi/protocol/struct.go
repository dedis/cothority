package protocol

import (
	"errors"
	"fmt"
	"time"

	"go.dedis.ch/kyber/v4"
	"go.dedis.ch/kyber/v4/pairing"
	"go.dedis.ch/kyber/v4/sign"
	"go.dedis.ch/kyber/v4/sign/bls"
	"go.dedis.ch/onet/v4"
	"go.dedis.ch/onet/v4/ciphersuite"
	"go.dedis.ch/onet/v4/log"
	"go.dedis.ch/onet/v4/network"
)

// DefaultProtocolName can be used from other packages to refer to this protocol.
// If this name is used, then the suite used to verify signatures must be
// the default cothority.Suite.
const DefaultProtocolName = "blsCoSiProtoDefault"

// DefaultSubProtocolName the name of the default sub protocol, started by the
// main protocol.
const DefaultSubProtocolName = "blsSubCoSiProtoDefault"

func init() {
	network.RegisterMessages(&Announcement{}, &Response{}, &Stop{})
}

// ResponseMap is the container used to store responses coming from the children.
type ResponseMap map[int]*Response

// BlsSignature contains the message and its aggregated signature.
type BlsSignature []byte

// GetMask creates and returns the mask associated with the signature. If
// no mask has been appended, a mask with every bit enabled is returned.
func (sig BlsSignature) GetMask(suite pairing.Suite, publics []kyber.Point) (*sign.Mask, error) {
	mask, err := sign.NewMask(suite, publics, nil)
	if err != nil {
		return nil, err
	}

	lenCom := suite.G1().PointLen()
	bits := sig[lenCom:]

	if len(bits) == 0 {
		for i := 0; i < mask.Len(); i++ {
			mask.SetBit(i, true)
		}
	} else {
		err := mask.SetMask(sig[lenCom:])
		if err != nil {
			return mask, err
		}
	}

	return mask, nil
}

// Point creates the point associated with the signature in G1.
func (sig BlsSignature) Point(suite pairing.Suite) (kyber.Point, error) {
	pointSig := suite.G1().Point()

	if err := pointSig.UnmarshalBinary(sig); err != nil {
		return nil, err
	}

	return pointSig, nil
}

// Verify checks the signature over the message using the public keys and a default policy.
func (sig BlsSignature) Verify(ps pairing.Suite, msg []byte, publics []kyber.Point) error {
	policy := sign.NewThresholdPolicy(DefaultThreshold(len(publics)))

	return sig.VerifyWithPolicy(ps, msg, publics, policy)
}

// VerifyWithPolicy checks the signature over the message using the given public keys and policy.
func (sig BlsSignature) VerifyWithPolicy(ps pairing.Suite, msg []byte, publics []kyber.Point, policy sign.Policy) error {
	if publics == nil || len(publics) == 0 {
		return errors.New("no public keys provided")
	}
	if msg == nil {
		return errors.New("no message provided")
	}
	if sig == nil || len(sig) == 0 {
		return errors.New("no signature provided")
	}

	lenCom := ps.G1().PointLen()
	signature := sig[:lenCom]

	// Unpack the participation mask and get the aggregate public key
	mask, err := sig.GetMask(ps, publics)
	if err != nil {
		return err
	}

	pubs := mask.Participants()
	aggPub := bls.AggregatePublicKeys(ps, pubs...)

	err = bls.Verify(ps, aggPub, msg, signature)
	if err != nil {
		return fmt.Errorf("didn't get a valid signature: %s", err)
	}

	log.Lvl3("Signature verified and is correct!")
	log.Lvl3("m.CountEnabled():", mask.CountEnabled())

	if !policy.Check(mask) {
		return errors.New("the policy is not fulfilled")
	}

	return nil
}

// Announcement is the blscosi annoucement message.
type Announcement struct {
	Msg       []byte // statement to be signed
	Data      []byte
	Nonce     []byte
	Timeout   time.Duration
	Threshold int
}

// StructAnnouncement just contains Announcement and the data necessary to identify and
// process the message in the onet framework.
type StructAnnouncement struct {
	*onet.TreeNode
	Announcement
}

// Response is the blscosi response message.
type Response struct {
	Signature *ciphersuite.CipherData
}

// StructResponse just contains Response and the data necessary to identify and
// process the message in the onet framework.
type StructResponse struct {
	*onet.TreeNode
	Response
}

// Refusal is the signed refusal response from a given node.
type Refusal struct {
	Signature *ciphersuite.CipherData
}

// StructRefusal contains the refusal and the treenode that sent it.
type StructRefusal struct {
	*onet.TreeNode
	Refusal
}

// Stop is a message used to instruct a node to stop its protocol.
type Stop struct{}

// StructStop is a wrapper around Stop for it to work with onet.
type StructStop struct {
	*onet.TreeNode
	Stop
}
