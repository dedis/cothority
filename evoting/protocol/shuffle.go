package protocol

import (
	"errors"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/kyber/v3/proof"
	"go.dedis.ch/kyber/v3/shuffle"
	"go.dedis.ch/kyber/v3/sign/schnorr"
	"go.dedis.ch/kyber/v3/util/random"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"

	"go.dedis.ch/cothority/v3/evoting/lib"
	"go.dedis.ch/cothority/v3/skipchain"
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

	User     uint32
	Election *lib.Election // Election to be shuffled.

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
// LG: rewrote the function to correctly call Done - probably should be
// rewritten even further. There are three `func() error` now with different
// error-handling. In case of error:
//   1. we abort and stop processing
//   2. will return the error, but first call 3.
//   3. try to call it and abort if error found
func (s *Shuffle) HandlePrompt(prompt MessagePrompt) error {
	added := 0
	var mixes []*lib.Mix
	var target int
	var ballots []*lib.Ballot

	// If this fails, we return and call `Done()`
	err := func() error {
		var err error
		mixes, err = s.Election.Mixes(s.Skipchain)
		if err != nil {
			return err
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
		target = 2 * len(s.Election.Roster.List) / 3

		if len(ballots) < 2 {
			if err := s.SendTo(s.Root(), &TerminateShuffle{
				Error: "shuffle error: not enough (> 2) ballots to shuffle",
			}); err != nil {
				log.Error(err)
			}
			return errors.New("not enough ballots to shuffle")
		}

		return nil
	}()
	if err != nil {
		s.Done()
		log.Error(err)
		return err
	}

	// This may fail, but we want to call the next node if we do. So no `Done`
	// when this fails.
	var mix *lib.Mix
	err = func() error {
		if len(mixes) > target {
			return s.SendTo(s.Root(), &TerminateShuffle{})
		}

		if s.IsRoot() && !s.LeaderParticipates {
			return nil
		}

		a, b := lib.Split(ballots)
		// Protect from missing input.
		for i := range a {
			if a[i] == nil {
				a[i] = cothority.Suite.Point().Null()
			}
		}
		for i := range b {
			if b[i] == nil {
				b[i] = cothority.Suite.Point().Null()
			}
		}
		g, d, prov := shuffle.Shuffle(cothority.Suite, nil, s.Election.Key, a, b, random.New())
		proof, err := proof.HashProve(cothority.Suite, "", prov)
		if err != nil {
			return err
		}
		mix = &lib.Mix{
			Ballots: lib.Combine(g, d),
			Proof:   proof,
			NodeID:  s.ServerIdentity().ID,
		}
		data, err := s.ServerIdentity().Public.MarshalBinary()
		if err != nil {
			return err
		}
		sig, err := schnorr.Sign(cothority.Suite, s.Private(), data)
		if err != nil {
			return err
		}
		mix.Signature = sig
		added = 1
		return nil
	}()

	// And send the result to the skipchain in case of success.
	if err == nil && added == 1 {
		transaction := lib.NewTransaction(mix, s.User)
		log.Lvl3(s.ServerIdentity(), "sending transaction to websocket")
		err = lib.StoreUsingWebsocket(s.Election.ID, s.Election.Roster, transaction)
		if err != nil {
			log.Lvl1(s.ServerIdentity(), "couldn't store new block - this is fatal:", err)
			s.Done()
			return s.SendTo(s.Root(), &TerminateShuffle{Error: err.Error()})
		}
	}

	// This is the continuing branch that is called even if the previous one returned
	// an error. If this fails on the root, we're done, too.
	errContinue := func() error {
		if len(s.Children()) > 0 && len(mixes)+added <= target {
			child := s.Children()[0]
			for {
				// err here only checks for network errors while trying to
				// send a message to the child
				log.Lvl3(s.ServerIdentity(), "sending to", child.ServerIdentity)
				err = s.SendTo(child, &PromptShuffle{})
				if err != nil {
					// retry with next one
					log.Lvl2(s.ServerIdentity(), "Couldn't send to", child.ServerIdentity, err)
					if len(child.Children) > 0 {
						child = child.Children[0]
					} else {
						errString := "shuffle error: retried all nodes, couldn't shuffle required number of times"
						log.Lvl2(s.ServerIdentity(), errString)
						err = s.SendTo(s.Root(), &TerminateShuffle{Error: errString})
						return errors.New(errString)
					}
				} else {
					return nil
				}
			}
		}
		if !s.IsRoot() {
			return s.SendTo(s.Root(), &TerminateShuffle{})
		}
		return nil
	}()

	// For the root-node, if only 2. failed, we're not done yet.
	if errContinue != nil || !s.IsRoot() {
		log.Lvl3(s.ServerIdentity(), "done")
		s.Done()
	}

	// Return a nice error string.
	if errContinue != nil {
		if err != nil {
			errConc := errors.New(err.Error() + " :: " + errContinue.Error())
			log.Error(errConc)
			return errConc
		}
		log.Error(errContinue)
		return errContinue
	}
	log.Lvl3(s.ServerIdentity(), "Done", err)
	return err
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
