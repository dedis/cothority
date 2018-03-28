package protocol

import (
	"errors"

	"github.com/dedis/cothority"
	"github.com/dedis/kyber/proof"
	"github.com/dedis/kyber/shuffle"
	"github.com/dedis/kyber/util/random"
	"github.com/dedis/onet"
	"github.com/dedis/onet/network"

	"github.com/dedis/cothority/evoting/lib"
)

/*
Each participating node creates a verifiable shuffle with a corresponding proof
of the last last block in election skipchain. This block is either a box of
encrypted ballot (in case of the root node) or a mix of the previous node. Every
newly created shuffle is appended to the chain before the next node is prompted
to create its shuffle. The leaf node notifies the root upon storing its shuffle,
which terminates the protocol. The individual mixes are not verified here but
only in a later stage of the election.

Schema:

        [Prompt]            [Prompt]            [Prompt]         [Terminate]
  Root ------------> Node1 ------------> Node2 --> ... --> Leaf ------------> Root

The protocol can only be started by the election's creator and is non-repeatable.
*/

// NameShuffle is the protocol identifier string.
const NameShuffle = "shuffle"

// Shuffle is the core structure of the protocol.
type Shuffle struct {
	*onet.TreeNodeInstance

	User      uint32
	Signature []byte
	Election  *lib.Election // Election to be shuffled.

	Finished chan bool // Flag to signal protocol termination.
}

func init() {
	network.RegisterMessages(PromptShuffle{}, TerminateShuffle{})
	onet.GlobalProtocolRegister(NameShuffle, NewShuffle)
}

// NewShuffle initializes the protocol object and registers all the handlers.
func NewShuffle(node *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	shuffle := &Shuffle{TreeNodeInstance: node, Finished: make(chan bool, 1)}
	shuffle.RegisterHandlers(shuffle.HandlePrompt, shuffle.HandleTerminate)
	return shuffle, nil
}

// Start is called on the root node prompting it to send itself a Prompt message.
func (s *Shuffle) Start() error {
	return s.HandlePrompt(MessagePrompt{s.TreeNode(), PromptShuffle{}})
}

// HandlePrompt retrieves, shuffles and stores the mix back on the skipchain.
func (s *Shuffle) HandlePrompt(prompt MessagePrompt) error {
	var ballots []*lib.Ballot
	if s.IsRoot() {
		box, err := s.Election.Box()
		if err != nil {
			return err
		}
		ballots = box.Ballots
	} else {
		mixes, err := s.Election.Mixes()
		if err != nil {
			return err
		}
		ballots = mixes[len(mixes)-1].Ballots
		defer s.finish()
	}

	if len(ballots) < 2 {
		return errors.New("not enough (> 2) ballots to shuffle")

	}

	a, b := lib.Split(ballots)
	g, d, prov := shuffle.Shuffle(cothority.Suite, nil, s.Election.Key, a, b, random.New())
	proof, err := proof.HashProve(cothority.Suite, "", prov)
	if err != nil {
		return err
	}
	mix := &lib.Mix{Ballots: lib.Combine(g, d), Proof: proof, Node: s.Name()}
	transaction := lib.NewTransaction(mix, s.User, s.Signature)
	if err := lib.Store(s.Election.ID, s.Election.Roster, transaction); err != nil {
		return err
	}

	if s.IsLeaf() {
		return s.SendTo(s.Root(), &TerminateShuffle{})
	}
	return s.SendToChildren(&PromptShuffle{})
}

// finish terminates the protocol within onet.
func (s *Shuffle) finish() {
	s.Done()
	s.Finished <- true
}

// HandleTerminate concludes the protocol.
func (s *Shuffle) HandleTerminate(terminate MessageTerminate) error {
	s.finish()
	return nil
}
