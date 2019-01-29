package rabin

import (
	"errors"
	"fmt"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/kyber/v3"
	dkgrabin "go.dedis.ch/kyber/v3/share/dkg/rabin"
	"go.dedis.ch/kyber/v3/util/key"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
)

// Name is the protocol identifier string.
const Name = "Rabin_DKG"

func init() {
	onet.GlobalProtocolRegister(Name, NewSetup)
}

// Setup can give the DKG that can be used to get the shared public key.
type Setup struct {
	*onet.TreeNodeInstance
	DKG       *dkgrabin.DistKeyGenerator
	Threshold uint32

	nodes   []*onet.TreeNode
	keypair *key.Pair
	publics []kyber.Point
	// Whether we started the `DKG.SecretCommits`
	commit   bool
	Wait     bool
	Finished chan bool

	structStartDeal    chan structStartDeal
	structDeal         chan structDeal
	structResponse     chan structResponse
	structSecretCommit chan structSecretCommit
	structWaitSetup    chan structWaitSetup
	structWaitReply    chan []structWaitReply
}

// NewSetup initialises the structure for use in one round
func NewSetup(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	o := &Setup{
		TreeNodeInstance: n,
		keypair:          key.NewKeyPair(cothority.Suite),
		Finished:         make(chan bool, 1),
		Threshold:        uint32(len(n.Roster().List) - (len(n.Roster().List)-1)/3),
		nodes:            n.List(),
	}

	err := o.RegisterHandlers(o.childInit, o.rootStartDeal)
	if err != nil {
		return nil, err
	}
	err = o.RegisterChannels(&o.structStartDeal, &o.structDeal, &o.structResponse,
		&o.structSecretCommit, &o.structWaitSetup, &o.structWaitReply)
	if err != nil {
		return nil, err
	}
	o.publics = make([]kyber.Point, len(o.nodes))
	return o, nil
}

// Start sends the Announce-message to all children
func (o *Setup) Start() error {
	log.Lvl3("Starting Protocol")
	// 1a - root asks children to send their public key
	errs := o.Broadcast(&Init{Wait: o.Wait})
	if len(errs) != 0 {
		return fmt.Errorf("boradcast failed with error(s): %v", errs)
	}
	return nil
}

// Dispatch takes care for channel-messages that need to be treated in the correct order.
func (o *Setup) Dispatch() error {
	defer o.Done()
	err := o.allStartDeal(<-o.structStartDeal)
	if err != nil {
		return err
	}
	for range o.publics[1:] {
		err := o.allDeal(<-o.structDeal)
		if err != nil {
			return err
		}
	}
	l := len(o.publics)
	for i := 0; i < l*(l-1); i++ {
		// This is expected to return some errors, so do not stop on them.
		err := o.allResponse(<-o.structResponse)
		if err != nil && err.Error() != "vss: already existing response from same origin" &&
			err.Error() != "dkg: can't give SecretCommits if deal not certified" {
			return err
		}
	}
	for i := 0; i < l; i++ {
		err := o.allSecretCommit(<-o.structSecretCommit)
		if err != nil {
			return err
		}
	}

	if o.Wait {
		if o.IsRoot() {
			o.SendToChildren(&WaitSetup{})
			<-o.structWaitReply
		} else {
			<-o.structWaitSetup
			o.SendToParent(&WaitReply{})
		}
	}

	if o.DKG.Finished() {
		o.Finished <- true
		return nil
	}
	err = errors.New("protocol is finished but dkg is not")
	log.Error(err)
	return err
}

// SharedSecret returns the necessary information for doing shared
// encryption and decryption.
func (o *Setup) SharedSecret() (*SharedSecret, error) {
	return NewSharedSecret(o.DKG)
}

// Children reactions
func (o *Setup) childInit(i structInit) error {
	o.Wait = i.Wait
	log.Lvl3(o.Name(), o.Wait)
	return o.SendToParent(&InitReply{Public: o.keypair.Public})
}

// Root-node messages
func (o *Setup) rootStartDeal(replies []structInitReply) error {
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
func (o *Setup) allStartDeal(ssd structStartDeal) error {
	log.Lvl3(o.Name(), "received startDeal from:", ssd.ServerIdentity)
	var err error
	o.DKG, err = dkgrabin.NewDistKeyGenerator(cothority.Suite, o.keypair.Private,
		ssd.Publics, int(ssd.Threshold))
	if err != nil {
		return err
	}
	o.publics = ssd.Publics
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

func (o *Setup) allDeal(sd structDeal) error {
	log.Lvl3(o.Name(), sd.ServerIdentity)
	resp, err := o.DKG.ProcessDeal(sd.Deal.Deal)
	if err != nil {
		log.Error(o.Name(), err)
		return err
	}
	return o.fullBroadcast(&Response{resp})
}

func (o *Setup) allResponse(resp structResponse) error {
	log.Lvl3(o.Name(), resp.ServerIdentity)
	just, err := o.DKG.ProcessResponse(resp.Response.Response)
	if err != nil {
		return err
	}
	if just != nil {
		log.Warn(o.Name(), "Got a justification: ", just)
		return nil
	}

	commit, err := o.DKG.SecretCommits()
	if !o.commit && err == nil {
		o.commit = true
		return o.fullBroadcast(&SecretCommit{commit})
	}
	return err
}

func (o *Setup) allSecretCommit(comm structSecretCommit) error {
	log.Lvl3(o.Name(), comm)
	compl, err := o.DKG.ProcessSecretCommits(comm.SecretCommit.SecretCommit)
	if err != nil {
		log.Error(o.Name(), err)
		return err
	}
	if compl != nil {
		log.Warn("got a complaint: ", compl)
	}
	return nil
}

// Convenience functions
func (o *Setup) fullBroadcast(msg interface{}) error {
	errs := o.Multicast(msg, o.nodes...)
	if len(errs) != 0 {
		return fmt.Errorf("multicast failed with error(s): %v", errs)
	}
	return nil
}
