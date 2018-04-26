package protocol

import (
	"errors"
	"sync"

	"github.com/dedis/cothority"
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/sign/schnorr"
	"github.com/dedis/onet"
	"github.com/dedis/onet/network"

	"github.com/dedis/cothority/evoting/lib"
	"github.com/dedis/cothority/skipchain"
)

/*
Each participating node begins with verifying the integrity of each mix. If
If all mixes are correct a partial decryption of the last mix is performed using
the node's shared secret from the DKG. The result is the appended to the election
skipchain before prompting the next node. A node sets a flag in its partial if it
cannot verify all the mixes. The leaf node notifies the root upon completing its turn, which
terminates the protocol.

Schema:

        [Prompt]            [Prompt]            [Prompt]         [Terminate]
  Root ------------> Node1 ------------> Node2 --> ... --> Leaf ------------> Root

The protocol can only be started by the election's creator and is non-repeatable.
*/

// NameDecrypt is the protocol identifier string.
const NameDecrypt = "decrypt"

// Decrypt is the core structure of the protocol.
type Decrypt struct {
	*onet.TreeNodeInstance

	User      uint32
	Signature []byte

	Secret   *lib.SharedSecret // Secret is the private key share from the DKG.
	Election *lib.Election     // Election to be decrypted.

	Finished           chan bool // Flag to signal protocol termination.
	LeaderParticipates bool      // LeaderParticipates is a flag to denote if leader should calculate the partial.
	successReplies     int
	mutex              sync.Mutex

	Skipchain *skipchain.Service
}

func init() {
	network.RegisterMessages(PromptDecrypt{}, TerminateDecrypt{})
	onet.GlobalProtocolRegister(NameDecrypt, NewDecrypt)
}

// NewDecrypt initializes the protocol object and registers all the handlers.
func NewDecrypt(node *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	decrypt := &Decrypt{TreeNodeInstance: node, Finished: make(chan bool, 1)}
	decrypt.RegisterHandlers(decrypt.HandlePrompt, decrypt.HandleTerminate)
	return decrypt, nil
}

// Start is called on the root node prompting it to send itself a Prompt message.
func (d *Decrypt) Start() error {
	return d.HandlePrompt(MessagePromptDecrypt{d.TreeNode(), PromptDecrypt{}})
}

// HandlePrompt retrieves the mixes, verifies them and performs a partial decryption
// on the last mix before appending it to the election skipchain.
func (d *Decrypt) HandlePrompt(prompt MessagePromptDecrypt) error {
	mixes, err := d.Election.Mixes(d.Skipchain)
	if err != nil {
		return err
	}
	target := 2 * len(d.Election.Roster.List) / 3
	if len(mixes) <= target {
		return errors.New("Not enough mixes")
	}
	partials, err := d.Election.Partials(d.Skipchain)
	delegate := func() error {
		if d.IsRoot() {
			d.successReplies = len(partials)
			if d.LeaderParticipates {
				d.successReplies++
			}
			d.Broadcast(&PromptDecrypt{})
			return nil
		}
		// report to root
		defer d.Done()
		return d.SendTo(d.Root(), &TerminateDecrypt{})
	}
	if d.IsRoot() && !d.LeaderParticipates {
		return delegate()
	}

	mix := mixes[len(mixes)-1]
	points := make([]kyber.Point, len(mix.Ballots))
	for i := range points {
		points[i] = lib.Decrypt(d.Secret.V, mix.Ballots[i].Alpha, mix.Ballots[i].Beta)
	}

	partial := &lib.Partial{
		Points:    points,
		Node:      d.Name(),
		PublicKey: d.Public(),
	}
	data, err := partial.PublicKey.MarshalBinary()
	if err != nil {
		return d.SendTo(d.Root(), &TerminateDecrypt{Error: err.Error()})
	}
	sig, err := schnorr.Sign(cothority.Suite, d.Private(), data)
	if err != nil {
		return d.SendTo(d.Root(), &TerminateDecrypt{Error: err.Error()})
	}
	partial.Signature = sig
	transaction := lib.NewTransaction(partial, d.User, d.Signature)
	if err = lib.StoreUsingWebsocket(d.Election.ID, d.Election.Roster, transaction); err != nil {
		return d.SendTo(d.Root(), &TerminateDecrypt{Error: err.Error()})
	}
	return delegate()
}

// finish terminates the protocol within onet.
func (d *Decrypt) finish(err error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	if err == nil {
		d.successReplies++
	}
	if d.successReplies > 2*len(d.Election.Roster.List)/3 {
		d.Done()
		d.Finished <- true
	}
}

// HandleTerminate concludes to the protocol.
func (d *Decrypt) HandleTerminate(terminate MessageTerminateDecrypt) error {
	if terminate.Error != "" {
		d.finish(errors.New(terminate.Error))
	} else {
		d.finish(nil)
	}
	return nil
}
