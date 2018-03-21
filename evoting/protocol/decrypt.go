package protocol

import (
	"github.com/dedis/kyber"
	"github.com/dedis/onet"
	"github.com/dedis/onet/network"

	"github.com/dedis/cothority/evoting/lib"
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

	Finished chan bool // Flag to signal protocol termination.
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
	if !d.IsRoot() {
		defer d.finish()
	}

	box, err := d.Election.Box()
	if err != nil {
		return err
	}
	mixes, err := d.Election.Mixes()
	if err != nil {
		return err
	}

	last := mixes[len(mixes)-1].Ballots
	points := make([]kyber.Point, len(box.Ballots))
	for i := range points {
		points[i] = lib.Decrypt(d.Secret.V, last[i].Alpha, last[i].Beta)
	}

	flag := Verify(d.Election.Key, box, mixes)
	partial := &lib.Partial{Points: points, Flag: flag, Node: d.Name()}
	transaction := lib.NewTransaction(partial, d.User, d.Signature)
	if err = lib.Store(d.Election.ID, d.Election.Roster, transaction); err != nil {
		return err
	}

	if d.IsLeaf() {
		return d.SendTo(d.Root(), &TerminateDecrypt{})
	}
	return d.SendToChildren(&PromptDecrypt{})
}

// finish terminates the protocol within onet.
func (d *Decrypt) finish() {
	d.Done()
	d.Finished <- true
}

// HandleTerminate concludes to the protocol.
func (d *Decrypt) HandleTerminate(terminate MessageTerminateDecrypt) error {
	d.finish()
	return nil
}

// Verify iteratively checks the integrity of each mix.
func Verify(key kyber.Point, box *lib.Box, mixes []*lib.Mix) bool {
	x, y := lib.Split(box.Ballots)
	v, w := lib.Split(mixes[0].Ballots)
	if lib.Verify(mixes[0].Proof, key, x, y, v, w) != nil {
		return false
	}

	for i := 0; i < len(mixes)-1; i++ {
		x, y = lib.Split(mixes[i].Ballots)
		v, w = lib.Split(mixes[i+1].Ballots)
		if lib.Verify(mixes[i+1].Proof, key, x, y, v, w) != nil {
			return false
		}
	}
	return true
}
