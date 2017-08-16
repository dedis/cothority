package protocol

/*
The onchain-protocol implements the key-reencryption described in Lefteris'
paper-draft about onchain-secrets (called BlockMage).
*/

import (
	"errors"

	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/crypto.v0/share"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
)

func init() {
	onet.GlobalProtocolRegister(NameOCS, NewOCS)
}

// OCS is only used to re-encrypt a public point. Before calling `Start`,
// DKG and U must be initialized by the caller.
type OCS struct {
	*onet.TreeNodeInstance
	Shared *SharedSecret  // Shared represents the private key
	U      abstract.Point // U is the encrypted secret
	Xc     abstract.Point // The client's public key
	// Done receives a 'true'-value when the protocol finished successfully.
	Done chan bool
	Uis  []*share.PubShare // re-encrypted shares
}

// NewSetupDKG initialises the structure for use in one round
func NewOCS(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	o := &OCS{
		TreeNodeInstance: n,
		Done:             make(chan bool, 1),
	}

	err := o.RegisterHandlers(o.reencrypt, o.reencryptReply)
	if err != nil {
		return nil, err
	}
	return o, nil
}

// Start asks all children to reply with a shared reencryption
func (o *OCS) Start() error {
	log.Lvl3("Starting Protocol")
	if o.Shared == nil {
		return errors.New("please initialize Shared first")
	}
	if o.U == nil {
		return errors.New("please initialize U first")
	}
	return o.Broadcast(&Reencrypt{U: o.U, Xc: o.Xc})
}

func (o *OCS) reencrypt(r structReencrypt) error {
	log.Lvl3(o.Name())
	ui, err := o.getUi(r.U, r.Xc)
	if err != nil {
		return nil
	}
	return o.SendToParent(&ReencryptReply{Ui: ui})
}

func (o *OCS) reencryptReply(rr []structReencryptReply) error {
	o.Uis = make([]*share.PubShare, len(o.List()))
	var err error
	o.Uis[0], err = o.getUi(o.U, o.Xc)
	if err != nil {
		return err
	}
	for _, r := range rr {
		o.Uis[r.ReencryptReply.Ui.I] = r.ReencryptReply.Ui
	}
	o.Done <- true
	return nil
}

func (o *OCS) getUi(U, Xc abstract.Point) (*share.PubShare, error) {
	v := network.Suite.Point().Mul(U, o.Shared.V)
	v.Add(v, network.Suite.Point().Mul(Xc, o.Shared.V))
	return &share.PubShare{
		I: o.Shared.Index,
		V: v,
	}, nil
}
