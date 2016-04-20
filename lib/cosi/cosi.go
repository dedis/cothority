/*
Package cosi is the Collective Signing implementation according to the paper of
Bryan Ford: http://arxiv.org/pdf/1503.08768v1.pdf .

Stages of CoSi

The CoSi-protocol has 4 stages:

1. Announcement: The leader multicasts an announcement
of the start of this round down through the spanning tree,
optionally including the statement S to be signed.

2. Commitment: Each node i picks a random secret vi and
computes its individual commit Vi = Gvi . In a bottom-up
process, each node i waits for an aggregate commit Vˆj from
each immediate child j, if any. Node i then computes its
own aggregate commit Vˆi = Vi \prod{j ∈ Cj}{Vˆj}, where Ci is the
set of i’s immediate children. Finally, i passes Vi up to its
parent, unless i is the leader (node 0).

3. Challenge: The leader computes a collective challenge c =
H(Vˆ0 ∥ S), then multicasts c down through the tree, along
with the statement S to be signed if it was not already
announced in phase 1.

4. Response: In a final bottom-up phase, each node i waits
to receive a partial aggregate response rˆj from each of
its immediate children j ∈ Ci. Node i now computes its
individual response ri = vi − cxi, and its partial aggregate
response rˆi = ri + \sum{j ∈ Cj}{rˆj} . Node i finally passes rˆi
up to its parent, unless i is the root.
*/
package cosi

import (
	"errors"
	"time"

	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
)

// Cosi is the struct that implements the basic cosi.
type Cosi struct {
	// Suite used
	suite abstract.Suite
	// the longterm private key we use during the rounds
	private abstract.Secret
	// timestamp of when the announcement is done (i.e. timestamp of the four
	// phases)
	timestamp int64
	// random is our own secret that we wish to commit during the commitment phase.
	random abstract.Secret
	// commitment is our own commitment
	commitment abstract.Point
	// V_hat is the aggregated commit (our own + the children's)
	aggregateCommitment abstract.Point
	// challenge holds the challenge for this round
	challenge abstract.Secret
	// response is our own computed response
	response abstract.Secret
	// aggregateResponses is the aggregated response from the children + our own
	aggregateResponse abstract.Secret
}

// NewCosi returns a new Cosi struct given the suite + longterm secret.
func NewCosi(suite abstract.Suite, private abstract.Secret) *Cosi {
	return &Cosi{
		suite:   suite,
		private: private,
	}
}

// Announcement holds only the timestamp for that round
type Announcement struct {
	Timestamp int64
}

// Commitment sends it's own commit Vi and the aggregate children's
// commit V^i
type Commitment struct {
	Commitment     abstract.Point
	ChildrenCommit abstract.Point
}

// Challenge is the Hash of V^0 || S, where S is the Timestamp
// and the message
type Challenge struct {
	Challenge abstract.Secret
}

// Response holds the actual node's response ri and the
// aggregate response r^i
type Response struct {
	Response     abstract.Secret
	ChildrenResp abstract.Secret
}

// Signature is the final message out of the Cosi-protocol. It can
// be used together with the message and the aggregate public key
// to verify that it's valid.
type Signature struct {
	Challenge abstract.Secret
	Response  abstract.Secret
}

// Exception is what a node that does not want to sign should include when
// passing up a response
type Exception struct {
	Public     abstract.Point
	Commitment abstract.Point
}

// CreateAnnouncement creates an Announcement message with the timestamp set
// to the current time.
func (c *Cosi) CreateAnnouncement() *Announcement {
	now := time.Now().Unix()
	c.timestamp = now
	return &Announcement{now}
}

// Announce stores the timestamp and relays the message.
func (c *Cosi) Announce(in *Announcement) *Announcement {
	c.timestamp = in.Timestamp
	return in
}

// CreateCommitment creates the commitment out of the random secret and returns
// the message to pass up in the tree. This is typically called by the leaves.
func (c *Cosi) CreateCommitment() *Commitment {
	c.genCommit()
	return &Commitment{
		Commitment: c.commitment,
	}
}

// Commit creates the commitment / secret + aggregate children commitments from
// the children's messages.
func (c *Cosi) Commit(comms []*Commitment) *Commitment {
	// generate our own commit
	c.genCommit()

	// take the children commitment
	childVHat := c.suite.Point().Null()
	for _, com := range comms {
		// Add commitment of one child
		childVHat = childVHat.Add(childVHat, com.Commitment)
		// add commitment of it's children if there is one (i.e. if it is not a
		// leaf)
		if com.ChildrenCommit != nil {
			childVHat = childVHat.Add(childVHat, com.ChildrenCommit)
		}
	}
	// add our own commitment to the global V_hat
	c.aggregateCommitment = c.suite.Point().Add(childVHat, c.commitment)
	return &Commitment{
		ChildrenCommit: childVHat,
		Commitment:     c.commitment,
	}

}

// CreateChallenge creates the challenge out of the message it has been given.
// This is typically called by Root.
func (c *Cosi) CreateChallenge(msg []byte) (*Challenge, error) {
	pb, err := c.aggregateCommitment.MarshalBinary()
	cipher := c.suite.Cipher(pb)
	cipher.Message(nil, nil, msg)
	c.challenge = c.suite.Secret().Pick(cipher)
	return &Challenge{
		Challenge: c.challenge,
	}, err
}

// Challenge keeps in memory the Challenge from the message.
func (c *Cosi) Challenge(ch *Challenge) *Challenge {
	c.challenge = ch.Challenge
	return ch
}

// CreateResponse is called by a leaf to create its own response from the
// challenge + commitment + private key. It returns the response to send up to
// the tree.
func (c *Cosi) CreateResponse() (*Response, error) {
	err := c.genResponse()
	return &Response{Response: c.response}, err
}

// Response generates the response from the commitment, challenge and the
// responses of its children.
func (c *Cosi) Response(responses []*Response) (*Response, error) {
	// create your own response
	if err := c.genResponse(); err != nil {
		return nil, err
	}
	aggregateResponse := c.suite.Secret().Zero()
	for _, resp := range responses {
		// add responses of child
		aggregateResponse = aggregateResponse.Add(aggregateResponse, resp.Response)
		// add responses of it's children if there is one (i.e. if it is not a
		// leaf)
		if resp.ChildrenResp != nil {
			aggregateResponse = aggregateResponse.Add(aggregateResponse, resp.ChildrenResp)
		}
	}
	// Add our own
	c.aggregateResponse = c.suite.Secret().Add(aggregateResponse, c.response)
	return &Response{
		Response:     c.response,
		ChildrenResp: aggregateResponse,
	}, nil

}

// GetAggregateResponse returns the aggregated response that this cosi has
// accumulated.
func (c *Cosi) GetAggregateResponse() abstract.Secret {
	return c.aggregateResponse
}

// GetChallenge returns the challenge that were passed down to this cosi.
func (c *Cosi) GetChallenge() abstract.Secret {
	return c.challenge
}

// GetCommitment returns the commitment generated by this CoSi (not aggregated).
func (c *Cosi) GetCommitment() abstract.Point {
	return c.commitment
}

// Signature returns a cosi Signature <=> a Schnorr signature. CAREFUL: you must
// call that when you are sure you have all the aggregated respones (i.e. the
// root of the tree if you use a tree).
func (c *Cosi) Signature() *Signature {
	return &Signature{
		c.challenge,
		c.aggregateResponse,
	}
}

// VerifyResponses verifies the response this CoSi has against the aggregated
// public key the tree is using.
// Check that: base**r_hat * X_hat**c == V_hat
func (c *Cosi) VerifyResponses(aggregatedPublic abstract.Point) error {
	commitment := c.suite.Point()
	commitment = commitment.Add(commitment.Mul(nil, c.aggregateResponse), c.suite.Point().Mul(aggregatedPublic, c.challenge))
	// T is the recreated V_hat
	T := c.suite.Point().Null()
	T = T.Add(T, commitment)
	// TODO put that into exception mechanism later
	// T.Add(T, cosi.ExceptionV_hat)
	if !T.Equal(c.aggregateCommitment) {
		return errors.New("recreated commitment is not equal to one given")
	}
	return nil

}

// genCommit generates a random secret vi and computes it's individual commit
// Vi = G^vi
func (c *Cosi) genCommit() {
	kp := config.NewKeyPair(c.suite)
	c.random = kp.Secret
	c.commitment = kp.Public
}

// genResponse creates the response
func (c *Cosi) genResponse() error {
	if c.private == nil {
		return errors.New("No private key given in this cosi")
	}
	if c.random == nil {
		return errors.New("No random secret computed in this cosi")
	}
	if c.challenge == nil {
		return errors.New("No challenge computed in this cosi")
	}
	// resp = random - challenge * privatekey
	// i.e. ri = vi - c * xi
	resp := c.suite.Secret().Mul(c.private, c.challenge)
	c.response = resp.Sub(c.random, resp)
	// no aggregation here
	c.aggregateResponse = c.response
	return nil
}

// VerifySignature verifies if the challenge and the secret (from the response phase) form a
// correct signature for this message using the aggregated public key.
func VerifySignature(suite abstract.Suite, msg []byte, public abstract.Point, challenge, secret abstract.Secret) error {
	// recompute the challenge and check if it is the same
	commitment := suite.Point()
	commitment = commitment.Add(commitment.Mul(nil, secret), suite.Point().Mul(public, challenge))

	return verifyCommitment(suite, msg, commitment, challenge)

}

func verifyCommitment(suite abstract.Suite, msg []byte, commitment abstract.Point, challenge abstract.Secret) error {
	pb, err := commitment.MarshalBinary()
	if err != nil {
		return err
	}
	cipher := suite.Cipher(pb)
	cipher.Message(nil, nil, msg)
	// reconstructed challenge
	reconstructed := suite.Secret().Pick(cipher)
	if !reconstructed.Equal(challenge) {
		return errors.New("Reconstructed challenge not equal to one given")
	}
	return nil
}

// VerifySignatureWithException will verify the signature taking into account
// the exceptions given. An exception is the pubilc key + commitment of a peer that did not
// sign.
// NOTE: No exception mechanism for "before" commitment has been yet coded.
func VerifySignatureWithException(suite abstract.Suite, public abstract.Point, msg []byte, challenge, secret abstract.Secret, exceptions []Exception) error {
	// first reduce the aggregate public key
	subPublic := suite.Point().Add(suite.Point().Null(), public)
	aggExCommit := suite.Point().Null()
	for _, ex := range exceptions {
		subPublic = subPublic.Sub(subPublic, ex.Public)
		aggExCommit = aggExCommit.Add(aggExCommit, ex.Commitment)
	}

	// recompute the challenge and check if it is the same
	commitment := suite.Point()
	commitment = commitment.Add(commitment.Mul(nil, secret), suite.Point().Mul(public, challenge))
	// ADD the exceptions commitment here
	commitment = commitment.Add(commitment, aggExCommit)
	// check if it is ok
	return verifyCommitment(suite, msg, commitment, challenge)
}

// VerifyCosiSignatureWithException is a wrapper around VerifySignatureWithException
// but it takes a Signature instead of the Challenge/Response
func VerifyCosiSignatureWithException(suite abstract.Suite, public abstract.Point, msg []byte, signature *Signature, exceptions []Exception) error {
	return VerifySignatureWithException(suite, public, msg, signature.Challenge, signature.Response, exceptions)
}
