package service

/*
* The Sicpa service uses a CISC (https://gopkg.in/dedis/cothority.v2/cisc) to store
* key/value pairs on a skipchain.
 */

import (
	"errors"

	"github.com/dedis/student_18_omniledger/omniledger/darc"

	"gopkg.in/dedis/cothority.v2"
	"gopkg.in/dedis/cothority.v2/skipchain"
	"gopkg.in/dedis/onet.v2"
)

// ServiceName is used for registration on the onet.
const ServiceName = "OmniLedger"

// ActionAddDarc is the action name for adding a darc to OmniLedger.
const ActionAddDarc = darc.Action("add_darc")

// ActionAddGenesis is the action name for adding a genesis block to OmniLedger.
const ActionAddGenesis = darc.Action("add_genesis")

// Client is a structure to communicate with the CoSi
// service
type Client struct {
	*onet.Client
}

// NewClient instantiates a new cosi.Client
func NewClient() *Client {
	return &Client{Client: onet.NewClient(cothority.Suite, ServiceName)}
}

// CreateGenesisBlock sets up a new skipchain to hold the key/value pairs. If
// a key is given, it is used to authenticate towards the cothority.
func (c *Client) CreateGenesisBlock(r *onet.Roster, signers ...*darc.Signer) (*CreateGenesisBlockResponse, error) {
	reply := &CreateGenesisBlockResponse{}
	msg, err := DefaultGenesisMsg(CurrentVersion, r, signers...)
	if err != nil {
		return nil, err
	}
	if err := c.SendProtobuf(r.List[0], msg, reply); err != nil {
		return nil, err
	}
	return reply, nil
}

// SetKeyValue sets a key/value pair and returns the created skipblock.
func (c *Client) SetKeyValue(r *onet.Roster, id skipchain.SkipBlockID,
	tx Transaction) (*SetKeyValueResponse, error) {
	reply := &SetKeyValueResponse{}
	err := c.SendProtobuf(r.List[0], &SetKeyValue{
		Version:     CurrentVersion,
		SkipchainID: id,
		Transaction: tx,
	}, reply)
	if err != nil {
		return nil, err
	}
	return reply, nil
}

// GetProof returns a proof for the key stored in the skipchain.
// The proof can be verified with the genesis skipblock and
// can proof the existence or the absence of the key.
func (c *Client) GetProof(r *onet.Roster, id skipchain.SkipBlockID, key []byte) (*GetProofResponse, error) {
	reply := &GetProofResponse{}
	err := c.SendProtobuf(r.List[0], &GetProof{
		Version: CurrentVersion,
		ID:      id,
		Key:     key,
	}, reply)
	if err != nil {
		return nil, err
	}
	return reply, nil
}

// DefaultGenesisMsg creates the message that is used to for creating the
// genesis darc and block.
func DefaultGenesisMsg(v Version, r *onet.Roster, signers ...*darc.Signer) (*CreateGenesisBlock, error) {
	if len(signers) == 0 {
		return nil, errors.New("no signers")
	}
	ids := make([]*darc.Identity, len(signers))
	for i := range ids {
		ids[i] = signers[i].Identity()
	}
	d := darc.NewDarc(darc.InitRules(ids, ids), []byte("genesis darc"))
	d.Rules.AddRule(ActionAddDarc, d.Rules.GetSignExpr())
	d.Rules.AddRule(ActionAddGenesis, d.Rules.GetSignExpr())

	// This transaction doesn't have signatures yet, we populate it later.
	tx := Transaction{
		Key:   append(d.GetID(), make([]byte, 64)...),
		Kind:  []byte(ActionAddGenesis),
		Value: []byte{},
	}

	req, err := tx.ToDarcRequest()
	if err != nil {
		return nil, err
	}
	req.Identities = ids

	digest, err := req.Hash()
	if err != nil {
		return nil, err
	}

	tx.Signatures = make([]darc.Signature, len(signers))
	for i := range tx.Signatures {
		sig, err := signers[i].Sign(digest)
		if err != nil {
			return nil, err
		}
		tx.Signatures[i] = darc.Signature{
			Signature: sig,
			Signer:    *signers[i].Identity(),
		}
	}

	m := CreateGenesisBlock{
		Version:     v,
		Roster:      *r,
		GenesisDarc: *d,
		GenesisTx:   tx,
	}
	return &m, nil
}
