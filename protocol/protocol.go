package protocol

/*
The onchain-protocol implements the key-reencryption described in Lefteris'
paper-draft about onchain-secrets (called BlockMage).
*/

import (
	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/crypto.v0/config"
	"gopkg.in/dedis/crypto.v0/share/dkg"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
)

func init() {
	onet.GlobalProtocolRegister(Name, NewProtocol)
}

// OnchainSecrets can give the DKG that can be used to get the shared public key.
type OnchainSecrets struct {
	*onet.TreeNodeInstance
	DKG *dkg.DistKeyGenerator

	init              chan chanInit
	initReply         chan []chanInitReply
	startDeal         chan chanStartDeal
	deal              chan chanDeal
	response          chan chanResponse
	secretCommit      chan chanSecretCommit
	verification      chan chanVerification
	verificationReply chan []chanVerificationReply
	keypair           config.KeyPair
	publics           []abstract.Point
}

// NewProtocol initialises the structure for use in one round
func NewProtocol(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	o := &OnchainSecrets{
		TreeNodeInstance: n,
		keypair:          config.NewKeyPair(network.Suite),
	}

	err := o.RegisterChannels(&o.init, &o.initReply, &o.startDeal, &o.deal,
		&o.response, &o.secretCommit, &o.verification,
		&o.verificationReply)

	if err != nil {
		return nil, err
	}
	return o, nil
}

// Start sends the Announce-message to all children
func (o *OnchainSecrets) Start() error {
	log.Lvl3("Starting Template")
	return nil
}

// Dispatch ensures the protocol is done in the correct order.

func (o *OnchainSecrets) Dispatch() {
	if !o.IsRoot() {
		<-o.init
		o.SendToParent(&InitReply{Public: o.keypair.Public})
	} else {
		replies := <-o.initReply
		o.publics = []abstract.Point{o.keypair.Public}
		// The order of the replies is not the same as the order in
		// the roster-list, this might be confusing when debugging!
		for _, r := range replies {
			o.publics = append(o.publics, r.Public)
		}
		o.SendToChildrenInParallel(&StartDeal{
			Publics: o.publics,
		})
	}

}
