package eventlog

import (
	"bytes"
	"errors"
	"sync"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
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
	sc         *skipchain.Client
	signerCtrs []uint64
}

// NewClient creates a new client to talk to the eventlog service.
// Fields DarcID, Instance, and Signers must be filled in before use.
func NewClient(ol *byzcoin.Client) *Client {
	return &Client{
		ByzCoin:    ol,
		c:          onet.NewClient(cothority.Suite, ServiceName),
		sc:         skipchain.NewClient(),
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
	tx, err := c.ByzCoin.CreateTransaction(instr)
	if err != nil {
		return err
	}
	if err := tx.FillSignersAndSignWith(c.Signers...); err != nil {
		return err
	}
	if _, err := c.ByzCoin.AddTransactionAndWait(tx, 10); err != nil {
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

// Log asks the service to log events. The client needs to wait for the log to
// be included for the next log to be accepted.
func (c *Client) Log(ev ...Event) ([]LogID, error) {
	return c.LogAndWait(10, ev...)
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
	reply, err := c.ByzCoin.GetProofFromLatest(key)
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

	instrs := make([]byzcoin.Instruction, len(events))
	for i, msg := range events {
		eventBuf, err := protobuf.Encode(&msg)
		if err != nil {
			return nil, nil, err
		}
		argEvent := byzcoin.Argument{
			Name:  "event",
			Value: eventBuf,
		}
		instrs[i] = byzcoin.Instruction{
			InstanceID: c.Instance,
			Invoke: &byzcoin.Invoke{
				ContractID: contractName,
				Command:    logCmd,
				Args:       []byzcoin.Argument{argEvent},
			},
			SignerCounter: c.incrementCtrs(),
		}
	}
	tx, err := c.ByzCoin.CreateTransaction(instrs...)
	if err != nil {
		return nil, nil, err
	}
	if err := tx.FillSignersAndSignWith(c.Signers...); err != nil {
		return nil, nil, err
	}
	for i := range tx.Instructions {
		keys[i] = LogID(tx.Instructions[i].DeriveID("").Slice())
	}
	return &tx, keys, nil
}

// Search executes a search on the filter in req. See the definition of type
// SearchRequest for additional details about how the filter is interpreted.
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

// StreamHandler is the signature of the handler used when streaming events.
type StreamHandler func(event Event, blockID []byte, err error)

// Close closes all the websocket connections.
func (c *Client) Close() error {
	err := c.ByzCoin.Close()
	if err2 := c.sc.Close(); err2 != nil {
		err = err2
	}
	if err2 := c.c.Close(); err2 != nil {
		err = err2
	}
	return err
}

// StreamEvents is a blocking call where it calls the handler on every new
// event until the connection is closed or the server stops.
func (c *Client) StreamEvents(handler StreamHandler) error {
	h := func(resp byzcoin.StreamingResponse, err error) {
		if err != nil {
			handler(Event{}, nil, err)
			return
		}
		// don't need to handle error because it's given to the handler
		_ = handleBlocks(handler, resp.Block)
	}
	// the following blocks
	return c.ByzCoin.StreamTransactions(h)
}

// StreamEventsFrom is a blocking call where it calls the handler on even new
// event from (inclusive) the given block ID until the connection is closed or
// the server stops.
func (c *Client) StreamEventsFrom(handler StreamHandler, id []byte) error {
	// 1. stream to a buffer (because we don't know which ones will be duplicates yet)
	blockChan := make(chan blockOrErr, 100)
	// The done channel is also buffered if a panic occurs in the client handler which
	// would prevent the wait group to close correctly.
	streamDone := make(chan error, 1)
	wg := sync.WaitGroup{}
	defer wg.Wait()
	go func() {
		wg.Add(1)
		defer wg.Done()

		err := c.ByzCoin.StreamTransactions(func(resp byzcoin.StreamingResponse, err error) {
			blockChan <- blockOrErr{resp.Block, err}
		})
		streamDone <- err
	}()

	// 2. use GetUpdateChain to find the missing events and call handler
	blocks, err := c.sc.GetUpdateChainLevel(&c.ByzCoin.Roster, id, 0, -1)
	if err != nil {
		return err
	}
	for _, b := range blocks {
		// to keep the behaviour of the other streaming functions, we
		// don't return an error but let the handler decide what to do
		// with the error
		_ = handleBlocks(handler, b)
	}

	var latest *skipchain.SkipBlock
	if len(blocks) > 0 {
		latest = blocks[len(blocks)-1]
	}

	// 3. read from the buffer, remove duplicates and call the handler
	var foundLink bool
	for {
		select {
		case bOrErr := <-blockChan:
			if bOrErr.err != nil {
				handler(Event{}, nil, bOrErr.err)
				break
			}
			if !foundLink {
				if bOrErr.block.BackLinkIDs[0].Equal(latest.Hash) {
					foundLink = true
				}
			}
			if foundLink {
				_ = handleBlocks(handler, bOrErr.block)
			}
		case err := <-streamDone:
			return err
		}
	}
}

type blockOrErr struct {
	block *skipchain.SkipBlock
	err   error
}

// handleBlocks calls the handler on the events of the block
func handleBlocks(handler StreamHandler, sb *skipchain.SkipBlock) error {
	var err error
	var header byzcoin.DataHeader
	err = protobuf.DecodeWithConstructors(sb.Data, &header, network.DefaultConstructors(cothority.Suite))
	if err != nil {
		err = errors.New("could not unmarshal header while streaming events " + err.Error())
		handler(Event{}, nil, err)
		return err
	}

	var body byzcoin.DataBody
	err = protobuf.DecodeWithConstructors(sb.Payload, &body, network.DefaultConstructors(cothority.Suite))
	if err != nil {
		err = errors.New("could not unmarshal body while streaming events " + err.Error())
		handler(Event{}, nil, err)
		return err
	}

	for _, tx := range body.TxResults {
		if tx.Accepted {
			for _, instr := range tx.ClientTransaction.Instructions {
				if instr.Invoke == nil {
					continue
				}
				if instr.Invoke.ContractID != contractName || instr.Invoke.Command != logCmd {
					continue
				}
				eventBuf := instr.Invoke.Args.Search("event")
				if eventBuf == nil {
					continue
				}
				event := &Event{}
				if err := protobuf.Decode(eventBuf, event); err != nil {
					handler(Event{}, nil, errors.New("could not decode the event "+err.Error()))
					continue
				}
				handler(*event, sb.Hash, nil)
			}
		}
	}
	return nil
}
