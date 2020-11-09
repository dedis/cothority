package connectivity

import (
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
	"golang.org/x/xerrors"
)

func init() {
	network.RegisterMessage(Ping{})
	network.RegisterMessage(Pong{})
	_, err := onet.GlobalProtocolRegister(Name, NewProtocol)
	if err != nil {
		panic(err)
	}
}

type ConnectivityProtocol struct {
	*onet.TreeNodeInstance
	pingChan chan pingWrapper
	pongChan chan []pongWrapper
	failure  chan error
}

func NewProtocol(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	t := &ConnectivityProtocol{
		TreeNodeInstance: n,
		failure:          make(chan error),
	}

	if err := n.RegisterChannels(&t.pingChan, &t.pingChan); err != nil {
		return nil, xerrors.Errorf("error registering channel: %v", err)
	}

	t.RegisterHandlers(t.onPong)

	return t, nil
}

func (p *ConnectivityProtocol) Start() error {
	log.Lvl3(p.ServerIdentity(), "Starting ConnectivityProtocol")
	return p.SendTo(p.TreeNode(), &Ping{})
}

func (p *ConnectivityProtocol) Dispatch() error {
	defer p.Done()
	defer close(p.failure)

	ping := <-p.pingChan
	if p.IsRoot() {
		errs := p.SendToChildrenInParallel(&ping.Ping)
		for _, err := range errs {
			p.failure <- err
		}
		return nil
	} else {
		return p.SendToParent(&Pong{})
	}
}

func (p *ConnectivityProtocol) onPong(pong pongWrapper) error {
	return nil
}
