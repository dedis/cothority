package byzcoin

import (
	"bytes"
	"errors"
	"math"
	"time"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/darc"
	"github.com/dedis/cothority/darc/expression"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"github.com/dedis/protobuf"
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
	err := c.SendProtobuf(c.Roster.RandomServerIdentity(), &GetProof{
		Version: CurrentVersion,
		ID:      c.ID,
		Key:     key,
	}, reply)
	if err != nil {
		return nil, err
	}
	return reply, nil
}

// CheckAuthorization verifies which actions the given set of identities can
// execute in the given darc.
func (c *Client) CheckAuthorization(dID darc.ID, ids ...darc.Identity) ([]darc.Action, error) {
	reply := &CheckAuthorizationResponse{}
	err := c.SendProtobuf(c.Roster.RandomServerIdentity(), &CheckAuthorization{
		Version:    CurrentVersion,
		ByzCoinID:  c.ID,
		DarcID:     dID,
		Identities: ids,
	}, reply)
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
	p, err := c.GetProof(NewInstanceID(nil).Slice())
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
	if err != nil {
		return nil, errors.New("couldn't get keyvalue: " + err.Error())
	}
	if contract != ContractConfigID {
		return nil, errors.New("expected contract to be config but got: " + contract)
	}
	if len(darcID) != 32 {
		return nil, errors.New("genesis darc ID is wrong length")
	}

	// Find the actual darc.
	p, err = c.GetProof(darcID)
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
	p, err := c.GetProof(NewInstanceID(nil).Slice())
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
			return nil, err
		}
		pr = resp.Proof
		ok, err := pr.InclusionProof.Exists(id.Slice())
		if err != nil {
			return nil, err
		}
		if ok {
			if value == nil {
				return &pr, nil
			}
			_, buf, _, _, err := pr.KeyValue()
			if err != nil {
				return nil, err
			}
			if bytes.Compare(buf, value) == 0 {
				return &pr, nil
			}
		}

		// wait for the block to be processed
		time.Sleep(interval / 5)
	}

	return nil, errors.New("timeout reached and inclusion not found")
}

// StreamTransactions sends a streaming request to the service. If successful,
// the handler will be called whenever a new response (a new block) is
// available. This function blocks, the streaming stops if the client or the
// service stops.
func (c *Client) StreamTransactions(handler func(StreamingResponse, error)) error {
	req := StreamingRequest{
		ID: c.ID,
	}
	conn, err := c.Stream(c.Roster.RandomServerIdentity(), &req)
	if err != nil {
		return err
	}
	for {
		resp := StreamingResponse{}
		if err := conn.ReadMessage(&resp); err != nil {
			handler(StreamingResponse{}, err)
			return nil
		}
		handler(resp, nil)
	}
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
	err := c.SendProtobuf(c.Roster.RandomServerIdentity(), &req, &reply)
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
	index := l - 1
	if l > 2 {
		// This is the leader plus the subleaders, don't contact them
		index = 1 + int(math.Ceil(math.Pow(float64(l), 1./3.)))
	}

	// Try to download from the nodes, starting with the first non-subleader.
	// Because the last elements of the roster might be a view-changed,
	// defective old leader, we start from the first non-subleader.
	for index < l {
		err = c.SendProtobuf(c.Roster.List[index], &DownloadState{
			ByzCoinID: byzcoinID,
			Nonce:     nonce,
			Length:    length,
		}, reply)
		if err == nil {
			return reply, nil
		}
		log.Error("Couldn't download from", c.Roster.List[index], ":", err)
		index++
	}
	return nil, errors.New("Error while downloading state from nodes")
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
