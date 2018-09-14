package byzcoin

import (
	"bytes"
	"errors"
	"time"

	"github.com/dedis/onet/network"
	"github.com/dedis/protobuf"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/byzcoin/darc"
	"github.com/dedis/cothority/byzcoin/darc/expression"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/onet"
)

// ServiceName is used for registration on the onet.
const ServiceName = "ByzCoin"

// Client is a structure to communicate with the ByzCoin service.
type Client struct {
	*onet.Client
	ID     skipchain.SkipBlockID
	Roster onet.Roster
}

// NewClient instantiates a new ByzCoin client.
func NewClient(ID skipchain.SkipBlockID, Roster onet.Roster) *Client {
	return &Client{
		Client: onet.NewClient(cothority.Suite, ServiceName),
		ID:     ID,
		Roster: Roster,
	}
}

// NewClientKeep is like NewClient, but does not close the connection when
// sending requests to the same conode.
func NewClientKeep(ID skipchain.SkipBlockID, Roster onet.Roster) *Client {
	return &Client{
		Client: onet.NewClientKeep(cothority.Suite, ServiceName),
		ID:     ID,
		Roster: Roster,
	}
}

// NewLedger sets up a new ByzCoin ledger.
func NewLedger(msg *CreateGenesisBlock, keep bool) (*Client, *CreateGenesisBlockResponse, error) {
	var c *Client
	if keep {
		c = NewClientKeep(nil, msg.Roster)
	} else {
		c = NewClient(nil, msg.Roster)
	}
	reply := &CreateGenesisBlockResponse{}
	if err := c.SendProtobuf(msg.Roster.List[0], msg, reply); err != nil {
		return nil, nil, err
	}
	c.ID = reply.Skipblock.CalculateHash()
	return c, reply, nil
}

// AddTransaction adds a transaction. It does not return any feedback
// on the transaction. Use GetProof to find out if the transaction
// was committed. The Client's Roster and ID should be initialized before
// calling this method (see NewClientFromConfig).
func (c *Client) AddTransaction(tx ClientTransaction) (*AddTxResponse, error) {
	return c.AddTransactionAndWait(tx, 0)
}

// AddTransactionAndWait adds a transaction and will wait for it to be included
// in the ledger, up to a maximum of wait block intervals. It does not return
// any feedback on the transaction. The Client's Roster and ID should be
// initialized before calling this method (see NewClientFromConfig).
func (c *Client) AddTransactionAndWait(tx ClientTransaction, wait int) (*AddTxResponse, error) {
	reply := &AddTxResponse{}
	err := c.SendProtobuf(c.Roster.List[0], &AddTxRequest{
		Version:       CurrentVersion,
		SkipchainID:   c.ID,
		Transaction:   tx,
		InclusionWait: wait,
	}, reply)
	if err != nil {
		return nil, err
	}
	return reply, nil
}

// GetProof returns a proof for the key stored in the skipchain by sending a
// message to the node on index 0 of the roster. The proof can be verified with
// the genesis skipblock and can prove the existence or the absence of the key.
// The Client's Roster and ID should be initialized before calling this method
// (see NewClientFromConfig).
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
// Genesis Darc from ByzCoin and parses it.
func (c *Client) GetGenDarc() (*darc.Darc, error) {
	p, err := c.GetProof(NewInstanceID(nil).Slice())
	if err != nil {
		return nil, err
	}
	if !p.Proof.InclusionProof.Match() {
		return nil, errors.New("cannot find genesis Darc ID")
	}

	_, vs, err := p.Proof.KeyValue()

	if len(vs) < 3 {
		return nil, errors.New("not enough records")
	}
	contractBuf := vs[1]
	if string(contractBuf) != ContractConfigID {
		return nil, errors.New("expected contract to be config but got: " + string(contractBuf))
	}
	darcID := vs[2]
	if len(darcID) != 32 {
		return nil, errors.New("genesis darc ID is wrong length")
	}

	p, err = c.GetProof(darcID)
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
	if string(contractBuf) != ContractDarcID {
		return nil, errors.New("expected contract to be darc but got: " + string(contractBuf))
	}
	d, err := darc.NewFromProtobuf(vs[0])
	if err != nil {
		return nil, err
	}
	return d, nil
}

// GetChainConfig uses the GetProof method to fetch the chain config
// from ByzCoin.
func (c *Client) GetChainConfig() (*ChainConfig, error) {
	p, err := c.GetProof(NewInstanceID(nil).Slice())
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
	if string(contractBuf) != ContractConfigID {
		return nil, errors.New("expected contract to be config but got: " + string(contractBuf))
	}
	config := &ChainConfig{}
	err = protobuf.DecodeWithConstructors(vs[0], config, network.DefaultConstructors(cothority.Suite))
	if err != nil {
		return nil, err
	}
	return config, nil
}

// WaitProof will poll ByzCoin until a given instanceID exists.
// It will return the proof of the instance created. If value is
// non-nil, it will wait for the value of the proof to be equal to
// the value.
// If the timeout is reached before the proof returns 'Match' or matches
// the value, it will return an error.
// TODO: remove interval and take it directly from the Client-structure.
func (c *Client) WaitProof(id InstanceID, interval time.Duration, value []byte) (*Proof, error) {
	var pr Proof
	for i := 0; i < 10; i++ {
		// try to get the darc back, we should get the genesis back instead
		resp, err := c.GetProof(id.Slice())
		if err != nil {
			return nil, err
		}
		pr = resp.Proof
		if pr.InclusionProof.Match() {
			if value == nil {
				return &pr, nil
			}
			vs, err := pr.InclusionProof.RawValues()
			if err != nil {
				return nil, err
			}
			if bytes.Compare(vs[0], value) == 0 {
				return &pr, nil
			}
		}

		// wait for the block to be processed
		time.Sleep(interval / 5)
	}

	return nil, errors.New("timeout reached and inclusion not found")
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

	// Add an additional rule that allows nodes in the roster to update the
	// genesis configuration, so that we can change the leader if one
	// fails.
	rosterPubs := make([]string, len(r.List))
	for i, sid := range r.List {
		rosterPubs[i] = darc.NewIdentityEd25519(sid.Public).String()
	}
	d.Rules.AddRule(darc.Action("invoke:view_change"), expression.InitOrExpr(rosterPubs...))

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
func SignInstruction(inst *Instruction, darcID darc.ID, signers ...darc.Signer) error {
	inst.Signatures = make([]darc.Signature, 0)
	var action string
	switch inst.GetType() {
	case SpawnType:
		action = "spawn:" + inst.Spawn.ContractID
	case InvokeType:
		action = "invoke:" + inst.Invoke.Command
	case DeleteType:
		action = "delete"
	}
	req, err := darc.InitAndSignRequest(darcID, darc.Action(action),
		inst.Hash(), signers...)
	if err != nil {
		return err
	}
	inst.Signatures = make([]darc.Signature, len(req.Signatures))
	for i, sig := range req.Signatures {
		inst.Signatures[i] = darc.Signature{
			Signature: sig,
			Signer:    signers[i].Identity(),
		}
	}
	return nil
}
