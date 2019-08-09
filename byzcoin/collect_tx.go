package byzcoin

import (
	"errors"
	"time"

	"go.dedis.ch/cothority/v3/byzcoinx"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
)

const defaultMaxNumTxs = 100

type getTxsCallback func(*network.ServerIdentity, *onet.Roster, skipchain.SkipBlockID, skipchain.SkipBlockID, int) []ClientTransaction

func init() {
	network.RegisterMessages(CollectTxRequest{}, CollectTxResponse{})
}

// CollectTxProtocol is a protocol for collecting pending transactions.
type CollectTxProtocol struct {
	*onet.TreeNodeInstance
	TxsChan           chan []ClientTransaction
	CommonVersionChan chan Version
	SkipchainID       skipchain.SkipBlockID
	LatestID          skipchain.SkipBlockID
	MaxNumTxs         int
	requestChan       chan structCollectTxRequest
	responseChan      chan structCollectTxResponse
	getTxs            getTxsCallback
	Finish            chan bool
	closing           chan bool
	version           int
}

// CollectTxRequest is the request message that asks the receiver to send their
// pending transactions back to the leader.
type CollectTxRequest struct {
	SkipchainID skipchain.SkipBlockID
	LatestID    skipchain.SkipBlockID
	MaxNumTxs   int
	Version     int
}

// CollectTxResponse is the response message that contains all the pending
// transactions on the node.
type CollectTxResponse struct {
	Txs            []ClientTransaction
	ByzcoinVersion Version
}

type structCollectTxRequest struct {
	*onet.TreeNode
	CollectTxRequest
}

type structCollectTxResponse struct {
	*onet.TreeNode
	CollectTxResponse
}

// NewCollectTxProtocol is used for registering the protocol.
func NewCollectTxProtocol(getTxs getTxsCallback) func(*onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	return func(node *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
		c := &CollectTxProtocol{
			TreeNodeInstance: node,
			// If we do not buffer this channel then the protocol
			// might be blocked from stopping when the receiver
			// stops reading from this channel.
			TxsChan:           make(chan []ClientTransaction, len(node.List())),
			CommonVersionChan: make(chan Version, len(node.List())),
			MaxNumTxs:         defaultMaxNumTxs,
			getTxs:            getTxs,
			Finish:            make(chan bool),
			closing:           make(chan bool),
			version:           1,
		}
		if err := node.RegisterChannels(&c.requestChan, &c.responseChan); err != nil {
			return c, err
		}
		return c, nil
	}
}

// Start starts the protocol, it should only be called on the root node.
func (p *CollectTxProtocol) Start() error {
	if !p.IsRoot() {
		return errors.New("only the root should call start")
	}
	if len(p.SkipchainID) == 0 {
		return errors.New("missing skipchain ID")
	}
	if len(p.LatestID) == 0 {
		return errors.New("missing latest skipblock ID")
	}
	req := &CollectTxRequest{
		SkipchainID: p.SkipchainID,
		LatestID:    p.LatestID,
		MaxNumTxs:   p.MaxNumTxs,
		Version:     p.version,
	}
	// send to myself and the children
	if err := p.SendTo(p.TreeNode(), req); err != nil {
		return err
	}
	// do not return an error if we fail to send to some children
	if errs := p.SendToChildrenInParallel(req); len(errs) > 0 {
		for _, err := range errs {
			log.Error(p.ServerIdentity(), err)
		}
	}
	return nil
}

// Dispatch runs the protocol.
func (p *CollectTxProtocol) Dispatch() error {
	defer p.Done()

	var req structCollectTxRequest
	select {
	case req = <-p.requestChan:
	case <-p.Finish:
		return nil
	case <-time.After(time.Second):
		// This timeout checks whether the root started the protocol,
		// it is not like our usual timeout that detect failures.
		return errors.New("did not receive request")
	case <-p.closing:
		return errors.New("closing down system")
	}

	maxOut := -1
	if req.Version >= 1 {
		// Leader with older version will send a maximum value of 0 which
		// is the default value as the field is unknown.
		maxOut = req.MaxNumTxs
	}

	// send the result of the callback to the root
	resp := &CollectTxResponse{
		Txs:            p.getTxs(req.ServerIdentity, p.Roster(), req.SkipchainID, req.LatestID, maxOut),
		ByzcoinVersion: p.getByzcoinVersion(),
	}
	log.Lvl3(p.ServerIdentity(), "sends back", len(resp.Txs), "transactions")
	if p.IsRoot() {
		if err := p.SendTo(p.TreeNode(), resp); err != nil {
			return err
		}
	} else {
		if err := p.SendToParent(resp); err != nil {
			return err
		}
	}

	// wait for the results to come back and write to the channel
	defer close(p.TxsChan)
	if p.IsRoot() {
		vb := newVersionBuffer(len(p.Children()) + 1)

		leaderVersion := p.getByzcoinVersion()
		vb.add(p.ServerIdentity(), leaderVersion)

		finish := false

		for i := 0; i < len(p.List()) && !finish; i++ {
			select {
			case resp := <-p.responseChan:
				vb.add(resp.ServerIdentity, resp.ByzcoinVersion)

				// If more than the limit is sent, we simply drop all of them
				// as the conode is not behaving correctly.
				if p.version == 0 || len(resp.Txs) <= p.MaxNumTxs {
					p.TxsChan <- resp.Txs
				}
			case <-p.Finish:
				finish = true
			case <-p.closing:
				finish = true
			}
		}

		if vb.hasThresholdFor(leaderVersion) {
			p.CommonVersionChan <- leaderVersion
		}
	}
	return nil
}

// Shutdown closes the closing channel to abort any waiting on messages.
func (p *CollectTxProtocol) Shutdown() error {
	close(p.closing)
	return nil
}

func (p *CollectTxProtocol) getByzcoinVersion() Version {
	srv := p.Host().Service(ServiceName)
	if srv == nil {
		panic("Byzcoin should always be available as a service for this protocol")
	}

	return srv.(*Service).GetProtocolVersion()
}

type versionBuffer struct {
	versions   map[Version]int
	identities map[network.ServerIdentityID]bool
	threshold  int
}

func newVersionBuffer(n int) versionBuffer {
	return versionBuffer{
		versions:   make(map[Version]int),
		identities: make(map[network.ServerIdentityID]bool),
		threshold:  byzcoinx.Threshold(n),
	}
}

func (vb versionBuffer) add(si *network.ServerIdentity, v Version) {
	if vb.identities[si.ID] {
		// Make sure a malicious conode cannot send multiple version
		// upgrade request.
		return
	}
	vb.identities[si.ID] = true
	vb.versions[v]++
}

func (vb versionBuffer) hasThresholdFor(version Version) bool {
	sum := 0
	for k, v := range vb.versions {
		if k >= version {
			sum += v
		}
	}

	return sum >= vb.threshold
}
