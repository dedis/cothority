package cosi

import (
	"errors"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
)

// Cosi is the struct that implements the "vanilla" cosi.
type Cosi struct {
	// Suite used
	suite abstract.Suite
	// the longterm private key we use durings the rounds
	private abstract.Secret
	// random is our own secret that we wish to commit during the commitment phase.
	random abstract.Secret
	// commitment is our own commitment
	commitment abstract.Point
	// V_hat is the aggregated commit (our own + the children's)
	aggregateCommitment abstract.Point
	// challenge holds the challenge for this round
	challenge abstract.Secret
	// response is our own response computed
	response abstract.Secret
	// aggregateResponses is the aggregated response from the children + our own
	aggregateResponse abstract.Secret
}

// NewCosi returns a new Cosi struct out of the suite + longterm secret.
//
func NewCosi(suite abstract.Suite, private abstract.Secret) *Cosi {
	return &Cosi{
		suite:   suite,
		private: private,
	}
}

type Commitment struct {
	Commitment     abstract.Point
	ChildrenCommit abstract.Point
}

type Challenge struct {
	Challenge abstract.Secret
}

type Response struct {
	Response     abstract.Secret
	ChildrenResp abstract.Secret
}

// CreateCommitment creates the commitment out of the randoms secret and returns
// the message to pass up in the tree. This is typically called by leaves.
func (c *Cosi) CreateCommitment() *Commitment {
	c.genCommit()
	return &Commitment{
		Commitment: c.commitment,
	}
}

// Commit creates the commitment / secret + aggregate children commitments from
// the children's messages.
func (c *Cosi) Commit(comms []*Commitment) (*Commitment, error) {
	// generate our own commit
	c.genCommit()
	// take the children commitment
	child_v_hat := c.suite.Point().Null()
	for _, com := range comms {
		// Add commitment of child
		child_v_hat = child_v_hat.Add(child_v_hat, com.Commitment)
		// add commitment of its children if there is some (i.e. if it is not a
		// leaf)
		if com.ChildrenCommit != nil {
			child_v_hat = child_v_hat.Add(child_v_hat, com.ChildrenCommit)
		}
	}
	// add our own commitment to the global V_hat
	c.aggregateCommitment = child_v_hat.Add(child_v_hat, c.commitment)
	return &Commitment{
		ChildrenCommit: child_v_hat,
		Commitment:     c.commitment,
	}, nil

}

// CreateChallenge will create the challenge out of the message it has been given.
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

// Challenge will simply keep in memory the Challenge from the message.
func (c *Cosi) Challenge(ch *Challenge) *Challenge {
	c.challenge = ch.Challenge
	return ch
}

func (c *Cosi) CreateResponse() (*Response, error) {
	err := c.genResponse()
	return &Response{Response: c.response}, err
}

// Response will generate the response from the commitment,challenge and the
// response of its children.
func (c *Cosi) Response(responses []*Response) (*Response, error) {
	if err := c.genResponse(); err != nil {
		return nil, err
	}
	aggregateResponse := c.suite.Secret().Zero()
	for _, resp := range responses {
		// add responses of child
		aggregateResponse = aggregateResponse.Add(aggregateResponse, resp.Response)
		// add responses of its children if there is some (i.e. if it is not a
		// leaf)
		if resp.ChildrenResp != nil {
			aggregateResponse = aggregateResponse.Add(aggregateResponse, resp.ChildrenResp)
		}
	}
	c.aggregateResponse = aggregateResponse

	return &Response{
		Response:     c.response,
		ChildrenResp: aggregateResponse,
	}, nil

}

// genCommit will generate a random secret vi and compute its indivudual commit
// Vi = G^vi
func (c *Cosi) genCommit() {
	kp := config.NewKeyPair(c.suite)
	c.random = kp.Secret
	c.commitment = kp.Public
}

// genResponse will create the response
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
	return nil
}
