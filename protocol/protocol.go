package protocol

/*
The onchain-protocol implements the key-reencryption described in Lefteris'
paper-draft about onchain-secrets (called BlockMage).
*/

import (
	"fmt"

	"errors"

	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/crypto.v0/config"
	"gopkg.in/dedis/crypto.v0/random"
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
	DKG       *dkg.DistKeyGenerator
	Threshold uint32

	nodes   []*onet.TreeNode
	keypair *config.KeyPair
	publics []abstract.Point
	// Whether we started the `DKG.SecretCommits`
	commit bool
	done   chan bool

	structStartDeal    chan structStartDeal
	structDeal         chan structDeal
	structResponse     chan structResponse
	structSecretCommit chan structSecretCommit
}

// NewProtocol initialises the structure for use in one round
func NewProtocol(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	o := &OnchainSecrets{
		TreeNodeInstance: n,
		Threshold:        2,
		keypair:          config.NewKeyPair(network.Suite),
		done:             make(chan bool, 1),
		nodes:            n.List(),
	}

	err := o.RegisterHandlers(o.childInit, o.rootStartDeal)
	if err != nil {
		return nil, err
	}
	err = o.RegisterChannels(&o.structStartDeal, &o.structDeal, &o.structResponse,
		&o.structSecretCommit)
	if err != nil {
		return nil, err
	}
	o.publics = make([]abstract.Point, len(n.List()))
	return o, nil
}

// Start sends the Announce-message to all children
func (o *OnchainSecrets) Start() error {
	log.Lvl3("Starting Protocol")
	// 1a - root asks children to send their public key
	return o.Broadcast(&Init{})
}

// Dispatch takes care for channel-messages that need to be treated in the correct order.
func (o *OnchainSecrets) Dispatch() error {
	o.allStartDeal(<-o.structStartDeal)
	for _ = range o.publics[1:] {
		o.allDeal(<-o.structDeal)
	}
	l := len(o.publics)
	for i := 0; i < l*(l-1); i++ {
		o.allResponse(<-o.structResponse)
	}
	for i := 0; i < l; i++ {
		o.allSecretCommit(<-o.structSecretCommit)
	}
	if o.DKG.Finished() {
		o.done <- true
	} else {
		log.Error("protocol is finished but dkg is not!")
	}
	return nil
}

// Children reactions

func (o *OnchainSecrets) childInit(structInit) error {
	log.Lvl3(o.Name())
	return o.SendToParent(&InitReply{Public: o.keypair.Public})
}

// Root-node messages

func (o *OnchainSecrets) rootStartDeal(replies []structInitReply) error {
	log.Lvl3(o.Name(), replies)
	o.publics[0] = o.keypair.Public
	for _, r := range replies {
		index, _ := o.Roster().Search(r.ServerIdentity.ID)
		if index < 0 {
			return errors.New("unknown serverIdentity")
		}
		o.publics[index] = r.Public
	}
	return o.fullBroadcast(&StartDeal{
		Publics:   o.publics,
		Threshold: o.Threshold,
	})
}

// Messages for both

func (o *OnchainSecrets) allStartDeal(ssd structStartDeal) error {
	log.Lvl3(o.Name(), "received startDeal from:", ssd.ServerIdentity)
	var err error
	o.DKG, err = dkg.NewDistKeyGenerator(network.Suite, o.keypair.Secret,
		ssd.Publics, random.Stream, int(ssd.Threshold))
	o.publics = ssd.Publics
	if err != nil {
		return err
	}
	deals, err := o.DKG.Deals()
	if err != nil {
		return err
	}
	log.Lvl3(o.Name(), "sending out deals", len(deals))
	for i, d := range deals {
		if err := o.SendTo(o.nodes[i], &Deal{d}); err != nil {
			return err
		}
	}
	return nil
}

func (o *OnchainSecrets) allDeal(sd structDeal) error {
	log.Lvl3(o.Name(), sd.ServerIdentity)
	resp, err := o.DKG.ProcessDeal(sd.Deal.Deal)
	if err != nil {
		log.Error(o.Name(), err)
		return err
	}
	return o.fullBroadcast(&Response{resp})
}

func (o *OnchainSecrets) allResponse(resp structResponse) error {
	log.Lvl3(o.Name(), resp.ServerIdentity)
	just, err := o.DKG.ProcessResponse(resp.Response.Response)
	if err != nil {
		return err
	}
	if just != nil {
		return fmt.Errorf("Got a justification: %v", just)
	}

	commit, err := o.DKG.SecretCommits()
	if !o.commit && err == nil {
		o.commit = true
		return o.fullBroadcast(&SecretCommit{commit})
	}
	return errors.New("not enough responses yet")
}

func (o *OnchainSecrets) allSecretCommit(comm structSecretCommit) error {
	log.Lvl3(o.Name(), comm)
	compl, err := o.DKG.ProcessSecretCommits(comm.SecretCommit.SecretCommit)
	if err != nil {
		return err
	}
	if compl != nil {
		return fmt.Errorf("got a complaint: %v", compl)
	}
	return nil
}

// Convenience functions
func (o *OnchainSecrets) fullBroadcast(msg interface{}) error {
	return o.Multicast(msg, o.List()...)
}
