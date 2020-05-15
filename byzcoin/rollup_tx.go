package byzcoin

import (
	"errors"
	"time"

	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
	"golang.org/x/xerrors"
)

func init() {
	network.RegisterMessages(&AddTxRequest{}, &RequestAdded{})
	_, err := onet.GlobalProtocolRegister(rollupTxProtocol, NewRollupTxProtocol)
	log.ErrFatal(err)
}

type getTxsCallback func(*network.ServerIdentity, *onet.Roster, skipchain.SkipBlockID, skipchain.SkipBlockID, int) []ClientTransaction

const rollupTxProtocol = "RollupTxProtocol"
const defaultMaxNumTxs = 100

// RollupTxProtocol is a protocol for collecting pending transactions.
type RollupTxProtocol struct {
	*onet.TreeNodeInstance
	TxsChan chan []ClientTransaction
	NewTx   *AddTxRequest
	CtxChan chan ClientTransaction
	CommonVersionChan chan Version
	SkipchainID       skipchain.SkipBlockID
	LatestID          skipchain.SkipBlockID
	MaxNumTxs         int
	DoneChan          chan error
	getTxs            getTxsCallback
	Finish            chan bool
	closing           chan bool
	version           int

	addRequestChan   chan structAddTxRequest
	requestAddedChan chan structRequestAdded
}

type structAddTxRequest struct {
	*onet.TreeNode
	AddTxRequest
}

type structRequestAdded struct {
	*onet.TreeNode
	RequestAdded
}

// RequestAdded is the message that is sent in the requestAddedChan after a
// channel has been registered, in order for Dispatch() to become aware of
// the newly registered channel.
type RequestAdded struct {
}

// NewRollupTxProtocol is used for registering the protocol.
// This was in the signature before.
func NewRollupTxProtocol(node *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	c := &RollupTxProtocol{
		TreeNodeInstance: node,
		// If we do not buffer this channel then the protocol
		// might be blocked from stopping when the receiver
		// stops reading from this channel.
		TxsChan:           make(chan []ClientTransaction, len(node.List())),
		CommonVersionChan: make(chan Version, len(node.List())),
		MaxNumTxs:         defaultMaxNumTxs,
		Finish:            make(chan bool),
		DoneChan:          make(chan error),
		closing:           make(chan bool),
		version:           1,
	}
	if err := node.RegisterChannels(&c.addRequestChan, &c.requestAddedChan); err != nil {
		return c, xerrors.Errorf("registering channels: %v", err)
	}
	return c, nil
}

// Start starts the protocol, it should only be called on the root node.
func (p *RollupTxProtocol) Start() error {
	if !p.IsRoot() {
		return xerrors.New("only the root should call start")
	}
	if len(p.SkipchainID) == 0 {
		return xerrors.New("missing skipchain ID")
	}
	if len(p.LatestID) == 0 {
		return xerrors.New("missing latest skipblock ID")
	}
	err := p.SendTo(p.Children()[0], p.NewTx)
	if err != nil {
		p.Done()
		return err
	}

	return nil
}

// Dispatch runs the protocol.
func (p *RollupTxProtocol) Dispatch() error {
	defer p.Done()
	if p.IsRoot() {
		select {
		case <-p.requestAddedChan:
			p.DoneChan <- nil
			return nil
		case <-time.After(time.Second):
			err := errors.New("timeout while waiting for leader's reply")
			p.DoneChan <- err
			return err
		}
	}

	p.CtxChan <- (<-p.addRequestChan).Transaction
	return p.SendToParent(&RequestAdded{})
}

// Shutdown closes the closing channel to abort any waiting on messages.
func (p *RollupTxProtocol) Shutdown() error {
	close(p.closing)
	return nil
}

func (p *RollupTxProtocol) getByzcoinVersion() Version {
	srv := p.Host().Service(ServiceName)
	if srv == nil {
		panic("Byzcoin should always be available as a service for this protocol")
	}

	return srv.(*Service).GetProtocolVersion()
}
