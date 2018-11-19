package protocol

import (
	"errors"
	"fmt"
	"time"

	"github.com/dedis/kyber"
	"github.com/dedis/kyber/pairing"
	"github.com/dedis/kyber/pairing/bn256"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
)

// init is done at startup. It defines every messages that is handled by the network
// and registers the protocols.
func init() {
	onet.GlobalProtocolRegister(DefaultKDProtocolName, NewBlsKeyDist)
}

type BlsKeyDist struct {
	*onet.TreeNodeInstance
	nodes          []*onet.TreeNode
	PairingPublic  kyber.Point
	PairingPublics chan []kyber.Point
	Timeout        time.Duration
	RequestChannel chan structRequest
	RepliesChannel chan structReply
	pairingSuite   pairing.Suite
}

func NewBlsKeyDist(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	t := &BlsKeyDist{
		TreeNodeInstance: n,
		nodes:            n.List(),
		PairingPublics:   make(chan []kyber.Point, 1),
		pairingSuite:     bn256.NewSuite(),
	}
	err := t.RegisterChannels(&t.RequestChannel, &t.RepliesChannel)
	err = t.RegisterHandlers(t.getPubKeys)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (p *BlsKeyDist) Dispatch() error {
	request, _ := <-p.RequestChannel

	go func() {
		if errs := p.SendToChildrenInParallel(&request.Request); len(errs) > 0 {
			log.Error(p.ServerIdentity(), "failed to send request to all children: %v", errs)
		}
	}()

	pairingPublicByte, err := PublicKeyToByteSlice(p.pairingSuite, p.PairingPublic)
	if err != nil {
		return err
	}

	err = p.SendTo(p.Root(), &Reply{Public: pairingPublicByte})
	if err != nil {
		return err
	}

	if p.IsRoot() {
		defer p.Done()

		timeout := time.After(p.Timeout)
		publics := make([]kyber.Point, len(p.nodes))
		publicsByte := make([][]byte, len(p.nodes))

		receivedKeys := 0
		for receivedKeys < len(p.nodes) {
			select {
			case reply := <-p.RepliesChannel:
				index, _ := p.Roster().Search(reply.ServerIdentity.ID)
				if index < 0 {
					return errors.New("unknown serverIdentity")
				}
				publicPoint, err := publicByteSliceToPoint(p.pairingSuite, reply.Public)
				if err != nil {
					return err
				}
				publics[index] = publicPoint
				publicsByte[index] = reply.Public
				receivedKeys += 1

			case <-timeout:
				return fmt.Errorf("Timed out while receiving keys from all nodes")
			}
		}

		log.Lvl3("Received all public keys. Distributing them now")
		errs := p.SendToChildrenInParallel(&Distribute{Publics: publicsByte})
		if len(errs) != 0 {
			return fmt.Errorf("%v failed to distribute keys to all children: %v", p.ServerIdentity(), errs)
		}
		p.PairingPublics <- publics
	}
	return nil
}

func (p *BlsKeyDist) Start() error {
	// Root requests children to send their public keys
	request := structRequest{p.TreeNode(), Request{}}
	p.RequestChannel <- request
	return nil
}

func (p *BlsKeyDist) getPubKeys(dist structDistribute) error {
	defer p.Done()
	errs := p.SendToChildrenInParallel(&dist.Distribute)
	if len(errs) != 0 {
		return fmt.Errorf("%v failed to distribute keys to all children: %v", p.ServerIdentity(), errs)
	}
	publics := make([]kyber.Point, len(p.nodes))
	for index, public := range dist.Publics {
		publicPoint, err := publicByteSliceToPoint(p.pairingSuite, public)
		if err != nil {
			return err
		}
		publics[index] = publicPoint
	}
	p.PairingPublics <- publics
	return nil
}
