package protocol

/*
The onchain-protocol implements the key-reencryption described in Lefteris'
paper-draft about onchain-secrets (called BlockMage).
*/

import (
	"crypto/sha256"
	"errors"

	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/crypto.v0/random"
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
	Poly   *share.PubPoly // Represents all public keys
	U      abstract.Point // U is the encrypted secret
	Xc     abstract.Point // The client's public key
	// Done receives a 'true'-value when the protocol finished successfully.
	Done chan bool
	Uis  []*share.PubShare // re-encrypted shares
}

// NewOCS initialises the structure for use in one round
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

// Reencrypt is received by every node to give his part of
// the share
func (o *OCS) reencrypt(r structReencrypt) error {
	log.Lvl3(o.Name())
	ui, err := o.getUI(r.U, r.Xc)
	if err != nil {
		return nil
	}

	// Calculating proofs
	si := network.Suite.Scalar().Pick(random.Stream)
	uiHat := network.Suite.Point().Mul(network.Suite.Point().Add(r.U, r.Xc), si)
	hiHat := network.Suite.Point().Mul(nil, si)
	hash := sha256.New()
	ui.V.MarshalTo(hash)
	uiHat.MarshalTo(hash)
	hiHat.MarshalTo(hash)
	ei := network.Suite.Scalar().SetBytes(hash.Sum(nil))

	return o.SendToParent(&ReencryptReply{
		Ui: ui,
		Ei: ei,
		Fi: network.Suite.Scalar().Add(si, network.Suite.Scalar().Mul(o.Shared.V, ei)),
	})
}

// ReencryptReply is the root-node waiting for all replies and generating
// the reencryption key.
func (o *OCS) reencryptReply(rr []structReencryptReply) error {
	o.Uis = make([]*share.PubShare, len(o.List()))
	var err error
	o.Uis[0], err = o.getUI(o.U, o.Xc)
	if err != nil {
		return err
	}
	for _, r := range rr {
		// Verify proofs
		ufi := network.Suite.Point().Mul(network.Suite.Point().Add(o.U, o.Xc), r.Fi)
		uiei := network.Suite.Point().Mul(r.Ui.V, network.Suite.Scalar().Neg(r.Ei))
		uiHat := network.Suite.Point().Add(ufi, uiei)

		gfi := network.Suite.Point().Mul(nil, r.Fi)
		gxi := o.Poly.Eval(r.Ui.I).V
		hiei := network.Suite.Point().Mul(gxi, network.Suite.Scalar().Neg(r.Ei))
		hiHat := network.Suite.Point().Add(gfi, hiei)
		hash := sha256.New()
		r.Ui.V.MarshalTo(hash)
		uiHat.MarshalTo(hash)
		hiHat.MarshalTo(hash)
		e := network.Suite.Scalar().SetBytes(hash.Sum(nil))
		if e.Equal(r.Ei) {
			o.Uis[r.Ui.I] = r.Ui
		} else {
			log.Lvl1("Received invalid share from node", r.Ui.I)
		}
	}
	o.Done <- true
	return nil
}

func (o *OCS) getUI(U, Xc abstract.Point) (*share.PubShare, error) {
	v := network.Suite.Point().Mul(U, o.Shared.V)
	v.Add(v, network.Suite.Point().Mul(Xc, o.Shared.V))
	return &share.PubShare{
		I: o.Shared.Index,
		V: v,
	}, nil
}
