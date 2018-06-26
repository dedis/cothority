package service

import (
	"errors"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/dedis/onet/network"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/omniledger/darc"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/onet"
)

// ServiceName is used for registration on the onet.
const ServiceName = "OmniLedger"

// Client is a structure to communicate with the OmniLedger service.
type Client struct {
	*onet.Client
	ID      skipchain.SkipBlockID
	Roster  *onet.Roster
	OwnerID darc.Identity
}

// NewClient instantiates a new Omniledger client.
func NewClient() *Client {
	return &Client{Client: onet.NewClient(cothority.Suite, ServiceName)}
}

// NewClientFromConfig instantiates a new Omniledger client.
func NewClientFromConfig(fn string) (*Client, error) {
	cfg, err := loadConfig(fn)
	if err != nil {
		return nil, err
	}

	c := NewClient()
	c.Roster = &cfg.Roster
	c.ID = cfg.ID
	c.OwnerID = cfg.OwnerID
	return c, nil
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
// was committed. The Client's Roster and ID should be initialized before
// calling this method (see NewClientFromConfig).
func (c *Client) AddTransaction(tx ClientTransaction) (*AddTxResponse, error) {
	reply := &AddTxResponse{}
	err := c.SendProtobuf(c.Roster.List[0], &AddTxRequest{
		Version:     CurrentVersion,
		SkipchainID: c.ID,
		Transaction: tx,
	}, reply)
	if err != nil {
		return nil, err
	}
	return reply, nil
}

// GetProof returns a proof for the key stored in the skipchain.  The proof can
// be verified with the genesis skipblock and can prove the existence or the
// absence of the key. The Client's Roster and ID should be initialized before
// calling this method (see NewClientFromConfig).
func (c *Client) GetProof(key []byte) (*GetProofResponse, error) {
	reply := &GetProofResponse{}
	err := c.SendProtobuf(c.Roster.List[0], &GetProof{
		Version: CurrentVersion,
		ID:      c.ID,
		Key:     key,
	}, reply)
	if err != nil {
		return nil, err
	}
	return reply, nil
}

// A Config gathers all the information a client needs to know to talk to
// an OmniLedger instance.
type Config struct {
	ID     skipchain.SkipBlockID
	Roster onet.Roster
	// OwnerID is the identity that can sign evolutions of the genesis Darc.
	OwnerID darc.Identity
}

func init() { network.RegisterMessages(&Config{}) }

func loadConfig(fn string) (*Config, error) {
	buf, err := ioutil.ReadFile(fn)
	if err != nil {
		return nil, err
	}

	_, val, err := network.Unmarshal(buf, cothority.Suite)
	if err != nil {
		return nil, err
	}
	if cfg, ok := val.(*Config); ok {
		return cfg, nil
	}

	return nil, fmt.Errorf("unexpected config format: %T", val)
}

func (c *Config) String() string {
	var r []string
	for _, x := range c.Roster.List {
		r = append(r, x.Address.NetworkAddress())
	}

	return fmt.Sprintf("ID: %x\nRoster: %v", c.ID, strings.Join(r, ", "))
}

// DefaultGenesisMsg creates the message that is used to for creating the
// genesis Darc and block.
func DefaultGenesisMsg(v Version, r *onet.Roster, rules []string, ids ...darc.Identity) (*CreateGenesisBlock, error) {
	if len(ids) == 0 {
		return nil, errors.New("no identities ")
	}
	d := darc.NewDarc(darc.InitRulesWith(ids, ids, invokeEvolve), []byte("genesis darc"))
	for _, r := range rules {
		d.Rules.AddRule(darc.Action(r), d.Rules.GetSignExpr())
	}
	m := CreateGenesisBlock{
		Version:       v,
		Roster:        *r,
		GenesisDarc:   *d,
		BlockInterval: defaultInterval,
	}
	return &m, nil
}
