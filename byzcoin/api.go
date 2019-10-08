package byzcoin

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"time"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/cothority/v3/darc/expression"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/kyber/v3/sign/schnorr"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
	"go.dedis.ch/protobuf"
	"golang.org/x/xerrors"
)

// ServiceName is used for registration on the onet.
const ServiceName = "ByzCoin"

// Client is a structure to communicate with the ByzCoin service.
type Client struct {
	*onet.Client
	ID     skipchain.SkipBlockID
	Roster onet.Roster
	// Genesis is required when a full proof is sent by the server
	// to verify the roster provided.
	Genesis *skipchain.SkipBlock
	// Latest keeps track of the most recent known block for the client.
	Latest *skipchain.SkipBlock
	// Keeps the server identities that replied first to a DownloadState request
	noncesSI map[uint64]*network.ServerIdentity
	// Used for SendProtobufParallel. If it is nil, default values will be used.
	options *onet.ParallelOptions
}

// NewClient instantiates a new ByzCoin client.
func NewClient(ID skipchain.SkipBlockID, Roster onet.Roster) *Client {
	return &Client{
		Client:   onet.NewClient(cothority.Suite, ServiceName),
		ID:       ID,
		Roster:   Roster,
		noncesSI: make(map[uint64]*network.ServerIdentity),
	}
}

// NewClientKeep is like NewClient, but does not close the connection when
// sending requests to the same conode.
func NewClientKeep(ID skipchain.SkipBlockID, Roster onet.Roster) *Client {
	c := NewClient(ID, Roster)
	c.Client = onet.NewClientKeep(cothority.Suite, ServiceName)
	return c
}

// NewLedger sets up a new ByzCoin ledger.
func NewLedger(msg *CreateGenesisBlock, keep bool) (*Client, *CreateGenesisBlockResponse, error) {
	var c *Client
	if keep {
		c = NewClientKeep(nil, msg.Roster)
	} else {
		c = NewClient(nil, msg.Roster)
	}

	reply, err := newLedgerWithClient(msg, c)
	if err != nil {
		return nil, nil, err
	}

	c.ID = reply.Skipblock.Hash
	c.Genesis = reply.Skipblock
	c.Latest = c.Genesis
	return c, reply, nil
}

func (c *Client) getLatestKnownBlock() *skipchain.SkipBlock {
	if c.Latest == nil {
		return c.Genesis
	}

	return c.Latest
}

// UseNode sets the options so that only the given node will be contacted
func (c *Client) UseNode(n int) error {
	if n < 0 || n >= len(c.Roster.List) {
		return errors.New("index of node points past roster-list")
	}
	c.options = &onet.ParallelOptions{
		DontShuffle: true,
		StartNode:   n,
		AskNodes:    1,
		Parallel:    1,
	}
	return nil
}

// DontContact adds the given serverIdentity to the list of nodes that will
// not be contacted.
func (c *Client) DontContact(si *network.ServerIdentity) {
	if c.options == nil {
		c.options = &onet.ParallelOptions{}
	}
	c.options.IgnoreNodes = []*network.ServerIdentity{si}
}

func newLedgerWithClient(msg *CreateGenesisBlock, c *Client) (*CreateGenesisBlockResponse, error) {
	reply := &CreateGenesisBlockResponse{}
	if err := c.SendProtobuf(msg.Roster.List[0], msg, reply); err != nil {
		return nil, err
	}

	// checks if the returned genesis block has the same parameters
	if err := verifyGenesisBlock(reply.Skipblock, msg); err != nil {
		return nil, err
	}

	return reply, nil
}

// GetAllByzCoinIDs returns the list of Byzcoin chains known by the server given in
// parameter.
func (c *Client) GetAllByzCoinIDs(si *network.ServerIdentity) (*GetAllByzCoinIDsResponse, error) {
	reply := &GetAllByzCoinIDsResponse{}
	if err := c.SendProtobuf(si, &GetAllByzCoinIDsRequest{}, reply); err != nil {
		return nil, err
	}

	return reply, nil
}

// CreateTransaction creates a transaction from a list of instructions.
func (c *Client) CreateTransaction(instrs ...Instruction) (ClientTransaction, error) {
	if c.Latest == nil {
		if _, err := c.GetChainConfig(); err != nil {
			return ClientTransaction{}, err
		}
	}

	h, err := decodeBlockHeader(c.Latest)
	if err != nil {
		return ClientTransaction{}, err
	}

	tx := NewClientTransaction(h.Version, instrs...)
	return tx, nil
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
	if c.Genesis == nil {
		if err := c.fetchGenesis(); err != nil {
			return nil, xerrors.Errorf("fetching genesis: %w", err)
		}
	}

	// As we fetch the genesis if required, this will never be
	// nil but either the genesis or the latest.
	latest := c.getLatestKnownBlock()

	reply := &AddTxResponse{}
	_, err := c.SendProtobufParallel(c.Roster.List, &AddTxRequest{
		Version:       CurrentVersion,
		SkipchainID:   c.ID,
		Transaction:   tx,
		InclusionWait: wait,
		ProofFrom:     latest.Hash,
	}, reply, c.options)
	if err != nil {
		return nil, xerrors.Errorf("sending: %w", err)
	}

	if reply.Error != "" {
		return reply, xerrors.New(reply.Error)
	}

	if reply.Proof != nil {
		if err := reply.Proof.VerifyFromBlock(latest); err != nil {
			return reply, xerrors.Errorf("proof verification: %+v", err)
		}

		if c.Latest == nil || c.Latest.Index < reply.Proof.Latest.Index {
			c.Latest = &reply.Proof.Latest
		}
	}

	return reply, nil
}

// GetProof returns a proof for the key stored in the skipchain starting from
// the genesis block. The proof can prove the existence or the absence of the
// key. Note that the integrity of the proof is verified.
// The Client's Roster and ID should be initialized before calling this method
// (see NewClientFromConfig).
func (c *Client) GetProof(key []byte) (*GetProofResponse, error) {
	if c.Genesis == nil {
		if err := c.fetchGenesis(); err != nil {
			return nil, err
		}
	}

	return c.GetProofFrom(key, c.Genesis)
}

// GetProofFromLatest returns a proof for the key stored in the skipchain
// starting from the latest known block by this client. The proof
// can prove the existence or the absence of the key. Note that the integrity
// of the proof is verified.
// Caution: the proof will be verifiable only by client/service that knows the
// state of the chain up to the block. If you need to pass the Proof onwards to
// another server, you must use GetProof in order to create a complete standalone
// proof starting from the genesis block.
func (c *Client) GetProofFromLatest(key []byte) (*GetProofResponse, error) {
	if c.Latest == nil {
		return c.GetProof(key)
	}

	return c.GetProofFrom(key, c.Latest)
}

// GetProofFrom returns a proof for the key stored in the skipchain starting
// from the block given in parameter. The proof can prove the existence or
// the absence of the key. Note that the integrity of the proof is verified.
// Caution: the proof will be verifiable only by client/service that know the
// state of the chain up to the block. If you need to pass the Proof onwards to
// another server, you must use GetProof in order to create a complete standalone
// proof starting from the genesis block.
func (c *Client) GetProofFrom(key []byte, from *skipchain.SkipBlock) (*GetProofResponse, error) {
	return c.getProofRaw(key, from, nil)
}

// GetProofAfter returns a proof for the key stored in the skipchain
// starting from the latest known block by this client. The proof will always
// be newer than the barrier or it will return an error.
//
// 	key - Instance ID to be included in the proof.
//	full - When true, the proof returned will start from the genesis block.
//	block - The latest block won't be older than the barrier.
//
func (c *Client) GetProofAfter(key []byte, full bool, block *skipchain.SkipBlock) (*GetProofResponse, error) {
	if full {
		return c.getProofRaw(key, c.Genesis, block)
	}

	return c.getProofRaw(key, c.getLatestKnownBlock(), block)
}

func (c *Client) getProofRaw(key []byte, from, include *skipchain.SkipBlock) (*GetProofResponse, error) {
	decoder := func(buf []byte, msg interface{}) error {
		err := protobuf.Decode(buf, msg)
		if err != nil {
			return xerrors.Errorf("decoding: %+v", err)
		}

		gpr, ok := msg.(*GetProofResponse)
		if !ok {
			return xerrors.New("couldn't cast msg")
		}

		if err := gpr.Proof.VerifyFromBlock(from); err != nil {
			return xerrors.Errorf("proof verification: %+v", err)
		}

		if include != nil && gpr.Proof.Latest.Index < include.Index {
			return xerrors.New("latest block in proof is too old")
		}

		return nil
	}

	req := &GetProof{
		Version: CurrentVersion,
		Key:     key,
		ID:      from.Hash,
	}

	if include != nil {
		req.MustContainBlock = include.Hash
	}

	reply := &GetProofResponse{}
	_, err := c.SendProtobufParallelWithDecoder(c.Roster.List, req, reply, c.options, decoder)
	if err != nil {
		return nil, xerrors.Errorf("sending: %+v", err)
	}

	if c.Latest == nil || c.Latest.Index < reply.Proof.Latest.Index {
		c.Latest = &reply.Proof.Latest
	}

	return reply, nil
}

// GetDeferredData makes a request to retrieve the deferred instruction data
// and return the reply if the proof can be verified.
func (c *Client) GetDeferredData(instrID InstanceID) (*DeferredData, error) {
	return c.GetDeferredDataAfter(instrID, nil)
}

// GetDeferredDataAfter makes a request to retrieve the deferred instruction data
// and returns the reply if the proof can be verified and the block is not
// older than the barrier.
func (c *Client) GetDeferredDataAfter(instrID InstanceID, barrier *skipchain.SkipBlock) (*DeferredData, error) {
	pr, err := c.getProofRaw(instrID.Slice(), c.getLatestKnownBlock(), barrier)
	if err != nil {
		return nil, xerrors.Errorf("getting proof: %w", err)
	}

	if !pr.Proof.InclusionProof.Match(instrID.Slice()) {
		return nil, xerrors.New("key not set")
	}

	dataBuf, _, _, err := pr.Proof.Get(instrID.Slice())
	if err != nil {
		return nil, xerrors.Errorf("getting proof value: %w", err)
	}
	var result DeferredData
	if err = protobuf.Decode(dataBuf, &result); err != nil {
		return nil, xerrors.Errorf("decoding data: %w", err)
	}

	header, err := decodeBlockHeader(c.Latest)
	if err != nil {
		return nil, xerrors.Errorf("decoding header: %w")
	}

	result.ProposedTransaction.Instructions.SetVersion(header.Version)

	return &result, nil
}

// CheckAuthorization verifies which actions the given set of identities can
// execute in the given darc.
func (c *Client) CheckAuthorization(dID darc.ID, ids ...darc.Identity) ([]darc.Action, error) {
	reply := &CheckAuthorizationResponse{}
	_, err := c.SendProtobufParallel(c.Roster.List, &CheckAuthorization{
		Version:    CurrentVersion,
		ByzCoinID:  c.ID,
		DarcID:     dID,
		Identities: ids,
	}, reply, c.options)
	if err != nil {
		return nil, err
	}
	var ret []darc.Action
	for _, a := range reply.Actions {
		ret = append(ret, darc.Action(a))
	}
	return ret, nil
}

// GetGenDarc uses the GetProof method to fetch the latest version of the
// Genesis Darc from ByzCoin and parses it.
func (c *Client) GetGenDarc() (*darc.Darc, error) {
	// Get proof of the genesis darc.
	p, err := c.GetProofFromLatest(NewInstanceID(nil).Slice())
	if err != nil {
		return nil, err
	}
	ok, err := p.Proof.InclusionProof.Exists(NewInstanceID(nil).Slice())
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("cannot find genesis Darc ID")
	}

	// Sanity check the values.
	_, _, contract, darcID, err := p.Proof.KeyValue()
	if contract != ContractConfigID {
		return nil, errors.New("expected contract to be config but got: " + contract)
	}
	if len(darcID) != 32 {
		return nil, errors.New("genesis darc ID is wrong length")
	}

	// Find the actual darc.
	p, err = c.GetProofFromLatest(darcID)
	if err != nil {
		return nil, err
	}
	ok, err = p.Proof.InclusionProof.Exists(darcID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("cannot find genesis Darc")
	}

	// Check and parse the darc.
	_, darcBuf, contract, _, err := p.Proof.KeyValue()
	if err != nil {
		return nil, err
	}
	if contract != ContractDarcID {
		return nil, errors.New("expected contract to be darc but got: " + contract)
	}
	d, err := darc.NewFromProtobuf(darcBuf)
	if err != nil {
		return nil, err
	}
	return d, nil
}

// GetChainConfig uses the GetProof method to fetch the chain config
// from ByzCoin.
func (c *Client) GetChainConfig() (*ChainConfig, error) {
	p, err := c.GetProofFromLatest(NewInstanceID(nil).Slice())
	if err != nil {
		return nil, err
	}
	ok, err := p.Proof.InclusionProof.Exists(NewInstanceID(nil).Slice())
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("cannot find config")
	}

	_, configBuf, contract, _, err := p.Proof.KeyValue()
	if contract != ContractConfigID {
		return nil, errors.New("expected contract to be config but got: " + contract)
	}
	config := &ChainConfig{}
	err = protobuf.DecodeWithConstructors(configBuf, config, network.DefaultConstructors(cothority.Suite))
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
			log.Warnf("Error while getting proof: %+v", err)
			continue
		}
		pr = resp.Proof
		ok, err := pr.InclusionProof.Exists(id.Slice())
		if err != nil {
			return nil, xerrors.Errorf(
				"inclusion proof couldn't be checked: %+v", err)
		}
		if ok {
			if value == nil {
				return &pr, nil
			}
			_, buf, _, _, err := pr.KeyValue()
			if err != nil {
				return nil, xerrors.Errorf("couldn't get keyvalue: %+v", err)
			}
			if bytes.Compare(buf, value) == 0 {
				return &pr, nil
			}
		}

		// wait for the block to be processed
		time.Sleep(interval / 5)
	}

	return nil, xerrors.New("timeout reached and inclusion not found")
}

// StreamTransactions sends a streaming request to the service. If successful,
// the handler will be called whenever a new response (a new block) is
// available. This function blocks, the streaming stops if the client or the
// service stops. Only the integrity of the new block is verified.
//
// It contacts any random node by default. A specific node can be chosen by
// using `c.UseNode`.
func (c *Client) StreamTransactions(handler func(StreamingResponse, error)) error {
	req := StreamingRequest{
		ID: c.ID,
	}
	n := int(rand.Int31n(int32(len(c.Roster.List))))
	if c.options != nil {
		if c.options.DontShuffle {
			n = c.options.StartNode
		}
	}

	conn, err := c.Stream(c.Roster.List[n], &req)
	if err != nil {
		handler(StreamingResponse{}, err)
		return err
	}
	for {
		resp := StreamingResponse{}
		if err := conn.ReadMessage(&resp); err != nil {
			handler(StreamingResponse{}, err)
			return nil
		}

		if resp.Block.CalculateHash().Equal(resp.Block.Hash) {
			// send the block only if the integrity is correct
			handler(resp, nil)
		} else {
			err := fmt.Errorf("got a corrupted block from %v", c.Roster.List[0])
			log.Warn(err.Error())
			handler(StreamingResponse{}, err)
		}
	}
}

func (c *Client) signerCounterDecoder(buf []byte, data interface{}) error {
	err := protobuf.Decode(buf, data)
	if err != nil {
		return xerrors.Errorf("couldn't decode the counters reply: %w", err)
	}

	reply, ok := data.(*GetSignerCountersResponse)
	if !ok {
		return xerrors.New("wrong type of response")
	}

	// This assumes the client is up-to-date with the latest block which
	// is usually right as it is updated after each GetProof. The goal is
	// to insure that we got the data from the latest block.
	// Note: using versioning as index 0 might cause troubles.
	if c.Latest != nil {
		header, err := decodeBlockHeader(c.Latest)
		if err != nil {
			return xerrors.Errorf("decoding header: %w", err)
		}

		if header.Version < 2 {
			// Skip the check for version as the trie index is available
			// for version 2+ only.
			return nil
		}

		if uint64(c.Latest.Index) > reply.Index {
			return xerrors.New("data coming from an old block")
		}
	}

	return nil
}

// GetSignerCounters gets the signer counters from ByzCoin. The counter must be
// set correctly in the instruction for it to be verified. Every counter maps
// to a signer, if the most recent instruction is signed by the signer at count
// n, then the next instruction that the same signer signs must be on counter
// n+1.
func (c *Client) GetSignerCounters(ids ...string) (*GetSignerCountersResponse, error) {
	req := GetSignerCounters{
		SkipchainID: c.ID,
		SignerIDs:   ids,
	}
	var reply GetSignerCountersResponse
	_, err := c.SendProtobufParallelWithDecoder(c.Roster.List, &req, &reply,
		c.options, c.signerCounterDecoder)
	if err != nil {
		return nil, err
	}
	return &reply, nil
}

// DownloadState is used by a new node to ask to download the global state.
// The first call to DownloadState needs to have start = 0, so that the
// service creates a snapshot of the current state which it will serve over
// multiple requests.
//
// Every subsequent request should have start incremented by 'len'.
// If start > than the number of StateChanges available, an empty slice of
// StateChanges is returned.
//
// If less than 'len' StateChanges are available, only the remaining
// StateChanges are returned.
//
// The first StateChange with start == 0 holds the metadata of the
// trie which can be `protobuf.Decode`d into a struct{map[string][]byte}.
func (c *Client) DownloadState(byzcoinID skipchain.SkipBlockID, nonce uint64, length int) (reply *DownloadStateResponse, err error) {
	if length <= 0 {
		return nil, errors.New("invalid parameter")
	}

	reply = &DownloadStateResponse{}
	l := len(c.Roster.List)
	indexStart := 0
	if l > 3 {
		// This is the leader plus the subleaders, don't contact them
		indexStart = 1 + int(math.Ceil(math.Pow(float64(l), 1./3.)))
	}

	msg := &DownloadState{
		ByzCoinID: byzcoinID,
		Nonce:     nonce,
		Length:    length,
	}
	si, ok := c.noncesSI[nonce]
	if ok {
		err = c.SendProtobuf(si, msg, reply)
	} else {
		var si *network.ServerIdentity
		var po onet.ParallelOptions
		if c.options != nil {
			po = *c.options
		}
		po.Parallel = 1
		po.StartNode = indexStart
		si, err = c.SendProtobufParallel(c.Roster.List, msg, reply, &po)
		c.noncesSI[reply.Nonce] = si
	}
	return
}

// ResolveInstanceID resolves the instance ID using the given darc ID and name.
// The name must be already set by calling the naming contract.
func (c *Client) ResolveInstanceID(darcID darc.ID, name string) (InstanceID, error) {
	req := ResolveInstanceID{
		SkipChainID: c.ID,
		DarcID:      darcID,
		Name:        name,
	}
	reply := ResolvedInstanceID{}

	if _, err := c.SendProtobufParallel(c.Roster.List, &req, &reply, c.options); err != nil {
		return InstanceID{}, err
	}
	return reply.InstanceID, nil
}

// Debug can be used to dump things from a byzcoin service. If byzcoinID is nil, it will return all
// existing byzcoin instances. If byzcoinID is given, it will return all instances for that ID.
func Debug(url string, byzcoinID *skipchain.SkipBlockID) (reply *DebugResponse, err error) {
	reply = &DebugResponse{}
	request := &DebugRequest{}
	if byzcoinID != nil {
		request.ByzCoinID = *byzcoinID
	}
	si := &network.ServerIdentity{URL: url}
	err = onet.NewClient(cothority.Suite, ServiceName).SendProtobuf(si, request, reply)
	return
}

// DebugRemove deletes an existing byzcoin-instance from the conode.
func DebugRemove(si *network.ServerIdentity, byzcoinID skipchain.SkipBlockID) error {
	sig, err := schnorr.Sign(cothority.Suite, si.GetPrivate(), byzcoinID)
	if err != nil {
		return err
	}
	request := &DebugRemoveRequest{
		ByzCoinID: byzcoinID,
		Signature: sig,
	}
	return onet.NewClient(cothority.Suite, ServiceName).SendProtobuf(si, request, nil)
}

// DefaultGenesisMsg creates the message that is used to for creating the
// genesis Darc and block. It will contain rules for spawning and evolving the
// darc contract.
func DefaultGenesisMsg(v Version, r *onet.Roster, rules []string, ids ...darc.Identity) (*CreateGenesisBlock, error) {
	if len(ids) == 0 {
		return nil, errors.New("no identities ")
	}

	rs := darc.NewRules()
	ownerIDs := make([]string, len(ids))
	for i, o := range ids {
		ownerIDs[i] = o.String()
	}
	ownerExpr := expression.InitAndExpr(ownerIDs...)
	if err := rs.AddRule("invoke:"+ContractConfigID+"."+"update_config", ownerExpr); err != nil {
		log.Error(err)
		return nil, err
	}
	if err := rs.AddRule("spawn:"+ContractDarcID, ownerExpr); err != nil {
		return nil, err
	}
	if err := rs.AddRule("invoke:"+ContractDarcID+"."+cmdDarcEvolve, ownerExpr); err != nil {
		return nil, err
	}
	if err := rs.AddRule("invoke:"+ContractDarcID+"."+cmdDarcEvolveUnrestriction, ownerExpr); err != nil {
		return nil, err
	}
	if err := rs.AddRule("_sign", ownerExpr); err != nil {
		return nil, err
	}
	if err := rs.AddRule("spawn:"+ContractNamingID, ownerExpr); err != nil {
		return nil, err
	}
	d := darc.NewDarc(rs, []byte("genesis darc"))

	// extra rules
	for _, r := range rules {
		if err := d.Rules.AddRule(darc.Action(r), ownerExpr); err != nil {
			return nil, err
		}
	}

	// Add an additional rule that allows nodes in the roster to update the
	// genesis configuration, so that we can change the leader if one
	// fails.
	rosterPubs := make([]string, len(r.List))
	for i, sid := range r.List {
		rosterPubs[i] = darc.NewIdentityEd25519(sid.Public).String()
	}
	d.Rules.AddRule(darc.Action("invoke:"+ContractConfigID+".view_change"), expression.InitOrExpr(rosterPubs...))

	m := CreateGenesisBlock{
		Version:         v,
		Roster:          *r,
		GenesisDarc:     *d,
		BlockInterval:   defaultInterval,
		DarcContractIDs: []string{ContractDarcID},
	}
	return &m, nil
}

func (c *Client) fetchGenesis() error {
	skClient := skipchain.NewClient()

	// Integrity check is done by the request function.
	sb, err := skClient.GetSingleBlock(&c.Roster, c.ID)
	if err != nil {
		return err
	}

	c.Genesis = sb
	c.Latest = sb
	return nil
}

func verifyGenesisBlock(actual *skipchain.SkipBlock, expected *CreateGenesisBlock) error {
	if !actual.CalculateHash().Equal(actual.Hash) {
		return errors.New("got a corrupted block")
	}

	// check the block is like the proposal
	ok, err := actual.Roster.Equal(&expected.Roster)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("wrong roster in genesis block")
	}

	darcID, err := extractDarcID(actual)
	if err != nil {
		return err
	}

	if !darcID.Equal(expected.GenesisDarc.GetID()) {
		return errors.New("wrong darc spawned")
	}

	return nil
}

func extractDarcID(sb *skipchain.SkipBlock) (darc.ID, error) {
	var data DataBody
	err := protobuf.Decode(sb.Payload, &data)
	if err != nil {
		return nil, fmt.Errorf("fail to decode data: %v", err)
	}

	if len(data.TxResults) != 1 {
		return nil, errors.New("genesis block should only have one transaction")
	}

	if len(data.TxResults[0].ClientTransaction.Instructions) != 1 {
		return nil, errors.New("genesis transaction should have exactly one instructions")
	}

	instr := data.TxResults[0].ClientTransaction.Instructions[0]
	if instr.Spawn == nil {
		return nil, errors.New("didn't get a spawn instruction")
	}

	var darc darc.Darc
	err = protobuf.Decode(instr.Spawn.Args.Search("darc"), &darc)
	if err != nil {
		return nil, fmt.Errorf("fail to decode the darc: %v", err)
	}

	return darc.GetID(), nil
}
