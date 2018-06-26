package eventlog

import (
	"bytes"
	"errors"
	"time"

	"github.com/dedis/cothority/omniledger/darc"
	"github.com/dedis/cothority/omniledger/darc/expression"
	omniledger "github.com/dedis/cothority/omniledger/service"
	"github.com/dedis/protobuf"

	"github.com/dedis/cothority"
	"github.com/dedis/onet"
)

// Client is a structure to communicate with the eventlog service
type Client struct {
	olClient *omniledger.Client
	elClient *onet.Client
	// Signers are the Darc signers that will sign events sent with this client.
	Signers []darc.Signer
	// Darc is the current Darc associated with this skipchain. Use it as a base
	// in case you need to evolve the permissions on the EventLog.
	Darc       *darc.Darc
	InstanceID omniledger.InstanceID
}

// NewClient creates a new client to talk to the eventlog service.
func NewClient(ol *omniledger.Client) *Client {
	return &Client{
		olClient: ol,
		elClient: onet.NewClient(cothority.Suite, ServiceName),
	}
}

// AddWriter modifies the given darc.Rules to use expr as the authorized writer
// to add new Event Logs. If expr is nil, the current evolution expression is
// used instead.
func AddWriter(r darc.Rules, expr expression.Expr) darc.Rules {
	if expr == nil {
		expr = r.GetEvolutionExpr()
	}
	r["spawn:eventlog"] = expr
	r["invoke:eventlog"] = expr
	return r
}

// Init initialises an event logging skipchain. A sucessful call updates the
// ID, Signer and Darc fields of the Client. The new skipchain has a Darc that
// requires one signature from owner.
// TODO this is a hack, usually this is *not* how you'd initialise event logs.
// The proper way would be to initialise the genesis block on omniledger and
// have omniledger evolve/add darcs to grant the "spawn:eventlog" and
// "invoke:eventlog" permissions.
func (c *Client) Init(owner darc.Signer, blockInterval time.Duration) (*omniledger.InstanceID, error) {
	rules := darc.InitRules([]darc.Identity{owner.Identity()}, []darc.Identity{})
	d := darc.NewDarc(AddWriter(rules, nil), []byte("eventlog owner"))

	req := &omniledger.CreateGenesisBlock{
		Version:       omniledger.CurrentVersion,
		Roster:        *c.olClient.Roster,
		GenesisDarc:   *d,
		BlockInterval: blockInterval,
	}
	reply, err := c.olClient.CreateGenesisBlock(c.olClient.Roster, req)
	if err != nil {
		return nil, err
	}
	c.Darc = d
	c.Signers = []darc.Signer{owner}
	c.olClient.ID = reply.Skipblock.SkipChainID()

	// When we have a genesis block, we need to initialise one eventlog and
	// store its ID.
	var instID *omniledger.InstanceID
	instID, err = c.initEventLog()
	if err != nil {
		return nil, err
	}
	c.InstanceID = *instID
	return &c.InstanceID, nil
}

func (c *Client) initEventLog() (*omniledger.InstanceID, error) {
	instr := omniledger.Instruction{
		InstanceID: omniledger.InstanceID{
			DarcID: c.Darc.GetBaseID(),
		},
		Nonce:  omniledger.GenNonce(),
		Index:  0,
		Length: 1,
		Spawn:  &omniledger.Spawn{ContractID: contractName},
	}
	if err := instr.SignBy(c.Signers...); err != nil {
		return nil, err
	}
	tx := omniledger.ClientTransaction{
		Instructions: []omniledger.Instruction{instr},
	}
	if _, err := c.olClient.AddTransaction(tx); err != nil {
		return nil, err
	}
	var subID omniledger.SubID
	copy(subID[:], instr.Hash())
	objID := omniledger.InstanceID{
		DarcID: c.Darc.GetBaseID(),
		SubID:  subID,
	}
	return &objID, nil
}

// LoadFromExisting expects the omniledger to already be initialised and the
// instance ID should refer to an eventlog contract.
func (c *Client) LoadFromExisting(owner darc.Signer, ol *omniledger.Client, instanceID omniledger.InstanceID) error {
	// we need to load a eventlog index...
	return errors.New("not implemented")
}

// A LogID is an opaque unique identifier useful to find a given log message later
// via omniledger.GetProof.
type LogID []byte

// Log asks the service to log events.
func (c *Client) Log(ev ...Event) ([]LogID, error) {
	tx, keys, err := makeTx(c.InstanceID, ev, c.Darc.GetBaseID(), c.Signers)
	if err != nil {
		return nil, err
	}
	if _, err := c.olClient.AddTransaction(*tx); err != nil {
		return nil, err
	}
	return keys, nil
}

// GetEvent asks the service to retrieve an event.
func (c *Client) GetEvent(key []byte) (*Event, error) {
	reply, err := c.olClient.GetProof(key)
	if err != nil {
		return nil, err
	}
	if !reply.Proof.InclusionProof.Match() {
		return nil, errors.New("not an inclusion proof")
	}
	k, vs, err := reply.Proof.KeyValue()
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(k, key) {
		return nil, errors.New("wrong key")
	}
	if len(vs) < 2 {
		return nil, errors.New("not enough values")
	}
	e := Event{}
	err = protobuf.Decode(vs[0], &e)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func makeTx(eventlogID omniledger.InstanceID, msgs []Event, darcID darc.ID, signers []darc.Signer) (*omniledger.ClientTransaction, []LogID, error) {
	// We need the identity part of the signatures before
	// calling ToDarcRequest() below, because the identities
	// go into the message digest.
	sigs := make([]darc.Signature, len(signers))
	for i, x := range signers {
		sigs[i].Signer = x.Identity()
	}

	keys := make([]LogID, len(msgs))

	instrNonce := omniledger.GenNonce()
	tx := omniledger.ClientTransaction{
		Instructions: make([]omniledger.Instruction, len(msgs)),
	}
	for i, msg := range msgs {
		eventBuf, err := protobuf.Encode(&msg)
		if err != nil {
			return nil, nil, err
		}
		argEvent := omniledger.Argument{
			Name:  "event",
			Value: eventBuf,
		}
		tx.Instructions[i] = omniledger.Instruction{
			InstanceID: eventlogID,
			Nonce:      instrNonce,
			Index:      i,
			Length:     len(msgs),
			Invoke: &omniledger.Invoke{
				Command: contractName,
				Args:    []omniledger.Argument{argEvent},
			},
			Signatures: append([]darc.Signature{}, sigs...),
		}
	}
	for i := range tx.Instructions {
		darcSigs := make([]darc.Signature, len(signers))
		for j, signer := range signers {
			dr, err := tx.Instructions[i].ToDarcRequest()
			if err != nil {
				return nil, nil, err
			}

			sig, err := signer.Sign(dr.Hash())
			if err != nil {
				return nil, nil, err
			}
			darcSigs[j] = darc.Signature{
				Signature: sig,
				Signer:    signer.Identity(),
			}
		}
		tx.Instructions[i].Signatures = darcSigs
		keys[i] = LogID(tx.Instructions[i].DeriveID("event").Slice())
	}
	return &tx, keys, nil
}

// Search executes a search on the filter in req. See the definition of
// type SearchRequest for additional details about how the filter is interpreted.
// The ID field of the SearchRequest will be filled in from c, if it is null.
func (c *Client) Search(req *SearchRequest) (*SearchResponse, error) {
	if req.ID.IsNull() {
		req.ID = c.olClient.ID
	}
	reply := &SearchResponse{}
	if err := c.elClient.SendProtobuf(c.olClient.Roster.List[0], req, reply); err != nil {
		return nil, err
	}
	return reply, nil
}
