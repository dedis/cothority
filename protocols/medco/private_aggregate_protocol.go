package medco

import (
	"errors"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/cothority/services/medco/store"
)

func init() {
	network.RegisterMessageType(DataReferenceMessage{})
	network.RegisterMessageType(ChildAggregatedDataMessage{})
	sda.ProtocolRegisterName("PrivateAggregate", NewPrivateAggregate)
}

// ProtocolExampleChannels just holds a message that is passed to all children.
// It also defines a channel that will receive the number of children. Only the
// root-node will write to the channel.
type PrivateAggregateProtocol struct {
	*sda.TreeNodeInstance

	// Protocol feedback channel
	FeedbackChannel      chan map[[store.MAX_GROUP_ATTR]DeterministCipherText]CipherVector

	// Protocol communication channels
	DataReferenceChannel chan DataReferenceStruct
	ChildDataChannel    chan []ChildAggregatedDataStruct

	// Protocol state data
	DataReference *map[[store.MAX_GROUP_ATTR]DeterministCipherText]CipherVector
}

// NewExampleChannels initialises the structure for use in one round
func NewPrivateAggregate(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
	privateAggregateProtocol := &PrivateAggregateProtocol{
		TreeNodeInstance:       n,
		FeedbackChannel: make(chan CipherVector),
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

	dbg.Lvl1(p.Entity(),"started a Private Aggregate Protocol")


	p.SendToChildren(&DataReferenceMessage{})
	return nil
}

// Dispatch is an infinite loop to handle messages from channels
func (p *PrivateAggregateProtocol) Dispatch() error {

	// 1. Aggregation announcement phase
	if !p.IsRoot() {
		p.aggregationAnnouncementPhase()
	}

	localContribution := *p.DataReference//p.getAggregatedDataFromReference(*p.DataReference)

	// 2. Ascending aggregation phase
	aggregatedContribution := p.ascendingAggregationPhase(localContribution)
	dbg.Lvl1(p.Entity(), "completed aggregation phase.")


	// 3. Result reporting
	if p.IsRoot() {
		p.FeedbackChannel <- *aggregatedContribution
	}

	return nil
}


func (p *PrivateAggregateProtocol) aggregationAnnouncementPhase() {
	dataReferenceMessage := <-p.DataReferenceChannel
	if !p.IsLeaf() {
		p.SendToChildren(&dataReferenceMessage.DataReferenceMessage)
	}
}

func (p *PrivateAggregateProtocol) ascendingAggregationPhase(localContribution *CipherVector) *CipherVector {
	if localContribution == nil {
		nullContrib := NullCipherVector(p.Suite(), 4, p.EntityList().Aggregate)
		localContribution = &nullContrib
	}
	if !p.IsLeaf() {
		for _,childrenContribution := range <- p.ChildDataChannel {
			localContribution.Add(*localContribution, childrenContribution.ChildData)
		}
	}
	if !p.IsRoot() {
		p.SendToParent(&ChildAggregatedDataMessage{*localContribution})
	}
	return localContribution
}