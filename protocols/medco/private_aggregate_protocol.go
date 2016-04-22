package medco

import (
	"errors"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
)


type Aggregatable interface {
	Aggregate(a1 Aggregatable, a2 Aggregatable) error
}

type DataRef int

func init() {
	network.RegisterMessageType(DataReferenceMessage{})
	network.RegisterMessageType(ChildAggregatedDataMessage{})
	sda.ProtocolRegisterName("PrivateAggregate", NewPrivateAggregate)
}

// ProtocolExampleChannels just holds a message that is passed to all children.
// It also defines a channel that will receive the number of children. Only the
// root-node will write to the channel.
type PrivateAggregateProtocol struct {
	*sda.Node

	// Protocol feedback channel
	FeedbackChannel      chan Aggregatable

	// Protocol communication channels
	DataReferenceChannel chan DataReferenceStruct
	ChildDataChannel    chan []ChildAggregatedDataStruct

	// Protocol state data
	DataReference *DataRef
}

// NewExampleChannels initialises the structure for use in one round
func NewPrivateAggregate(n *sda.Node) (sda.ProtocolInstance, error) {
	privateAggregateProtocol := &PrivateAggregateProtocol{
		Node:       n,
		FeedbackChannel: make(chan Aggregatable),
	}

	if err := privateAggregateProtocol.RegisterChannel(&privateAggregateProtocol.DataReferenceChannel); err != nil {
		return nil, errors.New("couldn't register data reference channel: " + err.Error())
	}
	if err := privateAggregateProtocol.RegisterChannel(&privateAggregateProtocol.ChildDataChannel); err != nil {
		return nil, errors.New("couldn't register child-data channel: " + err.Error())
	}

	return privateAggregateProtocol, nil
}

// Starts the protocol
func (p *PrivateAggregateProtocol) Start() error {

	if p.DataReference == nil {
		return errors.New("No data reference provided for aggregation.")
	}

	dbg.Lvl1(p.Name(),"started a Private Aggregate Protocol for data reference ", *p.DataReference)

	p.SendToChildren(&DataReferenceMessage{*p.DataReference})

	return nil
}

// Dispatch is an infinite loop to handle messages from channels
func (p *PrivateAggregateProtocol) Dispatch() error {

	// 1. Aggregation announcement phase
	if !p.IsRoot() {
		p.DataReference = p.aggregationAnnouncementPhase()
	}

	localContribution := p.getAggregatedDataFromReference(*p.DataReference)

	// 2. Ascending aggregation phase
	aggregatedContribution := p.ascendingAggregationPhase(localContribution)
	dbg.Lvl1(p.Name(), "completed aggregation phase.")


	// 3. Result reporting
	if p.IsRoot() {
		p.FeedbackChannel <- aggregatedContribution
	}

	return nil
}


func (p *PrivateAggregateProtocol) aggregationAnnouncementPhase() *DataRef {
	dataReferenceMessage := <-p.DataReferenceChannel
	if !p.IsLeaf() {
		// If we have children, send the same message to all of them
		p.SendToChildren(&dataReferenceMessage.DataReferenceMessage)
	}
	return &dataReferenceMessage.DataReference
}

func (p *PrivateAggregateProtocol) ascendingAggregationPhase(localContribution Aggregatable) Aggregatable {
	if !p.IsLeaf() {
		for _,childrenContribution := range <- p.ChildDataChannel {
			localContribution.Aggregate(localContribution, childrenContribution.ChildData)
		}
	}
	if !p.IsRoot() {
		p.SendToParent(&ChildAggregatedDataMessage{localContribution})
	}
	return localContribution
}

func (p *PrivateAggregateProtocol) getAggregatedDataFromReference(ref DataRef) Aggregatable {
	switch ref {
	case 0:
		nodeList := p.Tree().List()
		nullVect := NullCipherVector(p.Suite(), len(nodeList), p.Public())
		for i, node := range nodeList {
			if node.Equal(p.TreeNode()) {
				nullVect[i] = *EncryptInt(p.Suite(), p.Public(), 1)
				return &nullVect
			}
		}

	}
	return nil
}