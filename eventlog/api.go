package eventlog

import (
	"bytes"
	"errors"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/protobuf"
)

// Client is a structure to communicate with the eventlog service
type Client struct {
	ByzCoin *byzcoin.Client
	// The DarcID with "invoke:eventlog.log" permission on it.
	DarcID darc.ID
	// Signers are the Darc signers that will sign transactions sent with this client.
	Signers    []darc.Signer
	Instance   byzcoin.InstanceID
	c          *onet.Client
	signerCtrs []uint64
}

// NewClient creates a new client to talk to the eventlog service.
// Fields DarcID, Instance, and Signers must be filled in before use.
func NewClient(ol *byzcoin.Client) *Client {
	return &Client{
		ByzCoin:    ol,
		c:          onet.NewClient(cothority.Suite, ServiceName),
		signerCtrs: nil,
	}
}

// Create creates a new event log. This method is synchronous: it will only
// return once the new eventlog has been committed into the ledger (or after
// a timeout). Upon non-error return, c.Instance will be correctly set.
func (c *Client) Create() error {
	if c.signerCtrs == nil {
		c.RefreshSignerCounters()
	}

	instr := byzcoin.Instruction{
		InstanceID:    byzcoin.NewInstanceID(c.DarcID),
		Spawn:         &byzcoin.Spawn{ContractID: contractName},
		SignerCounter: c.nextCtrs(),
	}
	tx := byzcoin.ClientTransaction{
		Instructions: []byzcoin.Instruction{instr},
	}
	if err := tx.SignWith(c.Signers...); err != nil {
		return err
	}
	if _, err := c.ByzCoin.AddTransactionAndWait(tx, 2); err != nil {
		return err
	}

	c.incrementCtrs()
	c.Instance = tx.Instructions[0].DeriveID("")
	return nil
}

// RefreshSignerCounters talks to the service to get the latest signer
// counters, the client should call this function if the internal counters
// become de-synchronised.
func (c *Client) RefreshSignerCounters() {
	signerIDs := make([]string, len(c.Signers))
	for i := range c.Signers {
		signerIDs[i] = c.Signers[i].Identity().String()
	}
	signerCtrs, err := c.ByzCoin.GetSignerCounters(signerIDs...)
	if err != nil {
		log.Error(err)
		return
	}
	c.signerCtrs = signerCtrs.Counters
}

// incrementCtrs will update the client state
func (c *Client) incrementCtrs() []uint64 {
	out := make([]uint64, len(c.signerCtrs))
	for i := range out {
		c.signerCtrs[i]++
		out[i] = c.signerCtrs[i]
	}
	return out
}

// nextCtrs will not update client state
func (c *Client) nextCtrs() []uint64 {
	out := make([]uint64, len(c.signerCtrs))
	for i := range out {
		out[i] = c.signerCtrs[i] + 1
	}
	return out
}

// A LogID is an opaque unique identifier useful to find a given log message later
// via GetEvent.
type LogID []byte

// Log asks the service to log events.
func (c *Client) Log(ev ...Event) ([]LogID, error) {
	return c.LogAndWait(0, ev...)
}

// LogAndWait sends a request to log the events and waits for N block intervals
// that the events are added to the ledger
func (c *Client) LogAndWait(numInterval int, ev ...Event) ([]LogID, error) {
	if c.signerCtrs == nil {
		c.RefreshSignerCounters()
	}

	tx, keys, err := c.prepareTx(ev)
	if err != nil {
		return nil, err
	}
	if _, err := c.ByzCoin.AddTransactionAndWait(*tx, numInterval); err != nil {
		return nil, err
	}
	return keys, nil
}

// GetEvent asks the service to retrieve an event.
func (c *Client) GetEvent(key []byte) (*Event, error) {
	reply, err := c.ByzCoin.GetProof(key)
	if err != nil {
		return nil, err
	}
	if !reply.Proof.InclusionProof.Match(key) {
		return nil, errors.New("not an inclusion proof")
	}
	k, v0, _, _, err := reply.Proof.KeyValue()
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(k, key) {
		return nil, errors.New("wrong key")
	}
	e := Event{}
	err = protobuf.Decode(v0, &e)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (c *Client) prepareTx(events []Event) (*byzcoin.ClientTransaction, []LogID, error) {
	// We need the identity part of the signatures before
	// calling ToDarcRequest() below, because the identities
	// go into the message digest.
	sigs := make([]darc.Signature, len(c.Signers))
	for i, x := range c.Signers {
		sigs[i].Signer = x.Identity()
	}

	keys := make([]LogID, len(events))

	tx := byzcoin.ClientTransaction{
		Instructions: make([]byzcoin.Instruction, len(events)),
	}
	for i, msg := range events {
		eventBuf, err := protobuf.Encode(&msg)
		if err != nil {
			return nil, nil, err
		}
		argEvent := byzcoin.Argument{
			Name:  "event",
			Value: eventBuf,
		}
		tx.Instructions[i] = byzcoin.Instruction{
			InstanceID: c.Instance,
			Invoke: &byzcoin.Invoke{
				ContractID: contractName,
				Command:    logCmd,
				Args:       []byzcoin.Argument{argEvent},
			},
			SignerCounter: c.incrementCtrs(),
		}
	}
	if err := tx.SignWith(c.Signers...); err != nil {
		return nil, nil, err
	}
	for i := range tx.Instructions {
		keys[i] = LogID(tx.Instructions[i].DeriveID("").Slice())
	}
	return &tx, keys, nil
}

// Search executes a search on the filter in req. See the definition of
// type SearchRequest for additional details about how the filter is interpreted.
// The ID and Instance fields of the SearchRequest will be filled in from c.
func (c *Client) Search(req *SearchRequest) (*SearchResponse, error) {
	req.ID = c.ByzCoin.ID
	req.Instance = c.Instance

	reply := &SearchResponse{}
	if err := c.c.SendProtobuf(c.ByzCoin.Roster.List[0], req, reply); err != nil {
		return nil, err
	}
	return reply, nil
}
