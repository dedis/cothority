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
func (c *Client) CreateGenesisBlock(r *onet.Roster, msg *CreateGenesisBlock) (*CreateGenesisBlockResponse, error) {
	reply := &CreateGenesisBlockResponse{}
	if err := c.SendProtobuf(r.List[0], msg, reply); err != nil {
		return nil, err
	}
	return reply, nil
}

// AddTransaction adds a transaction. It does not return any feedback
// on the transaction. Use GetProof to find out if the transaction
// was committed.
func (c *Client) AddTransaction(r *onet.Roster, id skipchain.SkipBlockID,
	tx ClientTransaction) (*AddTxResponse, error) {
	reply := &AddTxResponse{}
	err := c.SendProtobuf(r.List[0], &AddTxRequest{
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
// can prove the existence or the absence of the key.
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
func DefaultGenesisMsg(v Version, r *onet.Roster, rules []string, ids ...*darc.Identity) (*CreateGenesisBlock, error) {
	if len(ids) == 0 {
		return nil, errors.New("no identities ")
	}
	d := darc.NewDarc(darc.InitRules(ids, ids), []byte("genesis darc"))
	for _, r := range rules {
		d.Rules.AddRule(darc.Action(r), d.Rules.GetSignExpr())
	}

	m := CreateGenesisBlock{
		Version:     v,
		Roster:      *r,
		GenesisDarc: *d,
	}
	return &m, nil
}
