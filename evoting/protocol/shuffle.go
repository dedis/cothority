package protocol

import (
	"errors"

	"github.com/dedis/cothority"
	"github.com/dedis/kyber/proof"
	"github.com/dedis/kyber/shuffle"
	"github.com/dedis/kyber/sign/schnorr"
	"github.com/dedis/kyber/util/random"
	"github.com/dedis/onet"
	"github.com/dedis/onet/network"

	"github.com/dedis/cothority/evoting/lib"
	"github.com/dedis/cothority/skipchain"
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

	Finished chan error // Flag to signal protocol termination.

	Skipchain          *skipchain.Service
	LeaderParticipates bool // LeaderParticipates is a flag that denotes if leader should participate in the shuffle
}

func init() {
	network.RegisterMessages(PromptShuffle{}, TerminateShuffle{})
	onet.GlobalProtocolRegister(NameShuffle, NewShuffle)
}

// NewShuffle initializes the protocol object and registers all the handlers.
func NewShuffle(node *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	shuffle := &Shuffle{TreeNodeInstance: node, Finished: make(chan error, 1)}
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
	mixes, err := s.Election.Mixes(s.Skipchain)
	if !s.IsRoot() {
		defer s.Done()
	}

	if len(mixes) == 0 {
		box, err := s.Election.Box(s.Skipchain)
		if err != nil {
			return err
		}
		ballots = box.Ballots
	} else {
		ballots = mixes[len(mixes)-1].Ballots
	}

	// base condition
	target := 2 * len(s.Election.Roster.List) / 3

	added := 0
	continueProtocol := func() error {
		if len(s.Children()) > 0 && len(mixes)+added <= target {
			child := s.Children()[0]
			for {
				// err here only checks for network errors while trying to
				// send a message to the child
				err = s.SendTo(child, &PromptShuffle{})
				if err != nil {
					// retry with next one
					if len(child.Children) > 0 {
						child = child.Children[0]
					} else {
						errString := "shuffle error: retried all nodes, couldn't shuffle required number of times"
						return s.SendTo(s.Root(), &TerminateShuffle{Error: errString})
					}
				} else {
					return nil
				}
			}
		}
		return s.SendTo(s.Root(), &TerminateShuffle{})
	}

	defer continueProtocol()

	if len(mixes) > target {
		return s.SendTo(s.Root(), &TerminateShuffle{})
	}

	if len(ballots) < 2 {
		return s.SendTo(s.Root(), &TerminateShuffle{
			Error: "shuffle error: not enough (> 2) ballots to shuffle",
		})
	}

	if s.IsRoot() && !s.LeaderParticipates {
		return nil
	}

	a, b := lib.Split(ballots)
	g, d, prov := shuffle.Shuffle(cothority.Suite, nil, s.Election.Key, a, b, random.New())
	proof, err := proof.HashProve(cothority.Suite, "", prov)
	if err != nil {
		return err
	}
	mix := &lib.Mix{
		Ballots:   lib.Combine(g, d),
		Proof:     proof,
		Node:      s.Name(),
		PublicKey: s.ServerIdentity().Public,
	}
	data, err := mix.PublicKey.MarshalBinary()
	if err != nil {
		return err
	}
	sig, err := schnorr.Sign(cothority.Suite, s.Private(), data)
	if err != nil {
		return err
	}
	mix.Signature = sig
	transaction := lib.NewTransaction(mix, s.User, s.Signature)
	err = lib.StoreUsingWebsocket(s.Election.ID, s.Election.Roster, transaction)
	if err != nil {
		return err
	}
	added = 1
	return nil
}

// finish terminates the protocol within onet.
func (s *Shuffle) finish(err error) {
	s.Done()
	s.Finished <- err
}

// HandleTerminate concludes the protocol.
func (s *Shuffle) HandleTerminate(terminate MessageTerminate) error {
	if terminate.Error != "" {
		s.finish(errors.New(terminate.Error))
	} else {
		s.finish(nil)
	}
	return nil
}
