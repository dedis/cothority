package service

import (
	"errors"
	"time"

	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
)

func init() {
	network.RegisterMessages(CollectTxRequest{}, CollectTxResponse{})
}

// CollectTxProtocol is a protocol for collecting pending transactions.
type CollectTxProtocol struct {
	*onet.TreeNodeInstance
	TxsChan      chan ClientTransactions
	SkipchainID  skipchain.SkipBlockID
	requestChan  chan structCollectTxRequest
	responseChan chan structCollectTxResponse
	getTxs       func(*network.ServerIdentity, skipchain.SkipBlockID) ClientTransactions
	Finish       chan bool
}

// CollectTxRequest is the request message that asks the receiver to send their
// pending transactions back to the leader.
type CollectTxRequest struct {
	SkipchainID skipchain.SkipBlockID
}

// CollectTxResponse is the response message that contains all the pending
// transactions on the node.
type CollectTxResponse struct {
	Txs ClientTransactions
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
func NewCollectTxProtocol(getTxs func(*network.ServerIdentity, skipchain.SkipBlockID) ClientTransactions) func(*onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	return func(node *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
		c := &CollectTxProtocol{
			TreeNodeInstance: node,
			// If we do not buffer this channel then the protocol
			// might be blocked from stopping when the receiver
			// stops reading from this channel.
			TxsChan: make(chan ClientTransactions, len(node.List())),
			getTxs:  getTxs,
			Finish:  make(chan bool),
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
	req := &CollectTxRequest{
		SkipchainID: p.SkipchainID,
	}
	// send to myself and the children
	if err := p.SendTo(p.TreeNode(), req); err != nil {
		return err
	}
	// do not return an error if we fail to send to some children
	if errs := p.SendToChildrenInParallel(req); len(errs) > 0 {
		for _, err := range errs {
			log.Error(err)
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
	}

	// send the result of the callback to the root
	resp := &CollectTxResponse{
		Txs: p.getTxs(req.ServerIdentity, req.SkipchainID),
	}
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
		for range p.List() {
			select {
			case resp := <-p.responseChan:
				p.TxsChan <- resp.Txs
			case <-p.Finish:
				return nil
			}
		}
	}
	return nil
}
