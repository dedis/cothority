package service

import (
	"errors"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/dedis/onet/network"
	"github.com/dedis/protobuf"

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

// GetGenDarc uses the GetProof method to fetch the latest version of the
// Genesis Darc from OmniLedger and parses it.
func (c *Client) GetGenDarc() (*darc.Darc, error) {
	p, err := c.GetProof(GenesisReferenceID.Slice())
	if err != nil {
		return nil, err
	}
	if !p.Proof.InclusionProof.Match() {
		return nil, errors.New("cannot find genesis Darc ID")
	}

	_, vs, err := p.Proof.KeyValue()

	if len(vs) < 2 {
		return nil, errors.New("not enough records")
	}
	contractBuf := vs[1]
	if string(contractBuf) != "config" {
		return nil, errors.New("expected contract to be config but got: " + string(contractBuf))
	}
	darcBuf := vs[0]
	if len(darcBuf) != 32 {
		return nil, errors.New("genesis darc ID is wrong length")
	}

	p, err = c.GetProof(InstanceID{DarcID: darcBuf}.Slice())
	if err != nil {
		return nil, err
	}
	if !p.Proof.InclusionProof.Match() {
		return nil, errors.New("cannot find genesis Darc")
	}

	_, vs, err = p.Proof.KeyValue()

	if len(vs) < 2 {
		return nil, errors.New("not enough records")
	}
	contractBuf = vs[1]
	if string(contractBuf) != "darc" {
		return nil, errors.New("expected contract to be darc but got: " + string(contractBuf))
	}
	d, err := darc.NewFromProtobuf(vs[0])
	if err != nil {
		return nil, err
	}
	return d, nil
}

// GetChainConfig uses the GetProof method to fetch the chain config
// from OmniLedger.
func (c *Client) GetChainConfig() (*ChainConfig, error) {
	d, err := c.GetGenDarc()

	cfid := InstanceID{d.GetBaseID(), one}
	p, err := c.GetProof(cfid.Slice())
	if err != nil {
		return nil, err
	}
	if !p.Proof.InclusionProof.Match() {
		return nil, errors.New("cannot find config")
	}

	_, vs, err := p.Proof.KeyValue()
	if len(vs) < 2 {
		return nil, errors.New("not enough records")
	}
	contractBuf := vs[1]
	if string(contractBuf) != "config" {
		return nil, errors.New("expected contract to be config but got: " + string(contractBuf))
	}
	config := &ChainConfig{}
	err = protobuf.Decode(vs[0], config)
	if err != nil {
		return nil, err
	}
	return config, nil
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

	return fmt.Sprintf("Skipchain ID: %x\nRoster: %v", c.ID, strings.Join(r, ", "))
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

// SignInstruction takes an instruction and one or more signers and adds
// a Signature to the instruction.
func SignInstruction(inst *Instruction, signers ...darc.Signer) error {
	inst.Signatures = make([]darc.Signature, 0)
	var action string
	switch {
	case inst.Spawn != nil:
		action = "spawn:" + inst.Spawn.ContractID
	case inst.Invoke != nil:
		action = "invoke:" + inst.Invoke.Command
	case inst.Delete != nil:
		action = "delete"
	}
	req, err := darc.InitAndSignRequest(inst.ObjectID.DarcID, darc.Action(action),
		inst.Hash(), signers...)
	if err != nil {
		return err
	}
	inst.Signatures = make([]darc.Signature, len(req.Signatures))
	for i, sig := range req.Signatures {
		inst.Signatures[i] = darc.Signature{sig, signers[i].Identity()}
	}
	return nil
}
