package medco

import (
	"errors"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	. "github.com/dedis/cothority/services/medco/structs"
)

const PRIVATE_AGGREGATE_PROTOCOL_NAME = "PrivateAggregate"

type DataReferenceMessage struct {
}

type DataReferenceStruct struct {
	*sda.TreeNode
	DataReferenceMessage
}

type ChildAggregatedDataMessage struct {
	ChildData   map[GroupingKey]CipherVector
	ChildGroups map[GroupingKey]GroupingAttributes
}

type ChildAggregatedDataStruct struct {
	*sda.TreeNode
	ChildAggregatedDataMessage
}

type CothorityAggregatedData struct {
	Groups      map[GroupingKey]GroupingAttributes
	GroupedData map[GroupingKey]CipherVector
}

func init() {
	network.RegisterMessageType(DataReferenceMessage{})
	network.RegisterMessageType(ChildAggregatedDataMessage{})
	sda.ProtocolRegisterName(PRIVATE_AGGREGATE_PROTOCOL_NAME, NewPrivateAggregate)
}

// ProtocolExampleChannels just holds a message that is passed to all children.
// It also defines a channel that will receive the number of children. Only the
// root-node will write to the channel.
type PrivateAggregateProtocol struct {
	*sda.TreeNodeInstance

	// Protocol feedback channel
	FeedbackChannel chan CothorityAggregatedData

	// Protocol communication channels
	DataReferenceChannel chan DataReferenceStruct
	ChildDataChannel     chan []ChildAggregatedDataStruct

	// Protocol state data
	GroupedData *map[GroupingKey]CipherVector
	Groups      *map[GroupingKey]GroupingAttributes
}

// NewExampleChannels initialises the structure for use in one round
func NewPrivateAggregate(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
	privateAggregateProtocol := &PrivateAggregateProtocol{
		TreeNodeInstance: n,
		FeedbackChannel:  make(chan CothorityAggregatedData),
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
	if p.GroupedData == nil {
		return errors.New("No data reference provided for aggregation.")
	}

	dbg.Lvl1(p.Entity(), "started a Private Aggregate Protocol")

	p.SendToChildren(&DataReferenceMessage{})

	return nil
}

// Dispatch is an infinite loop to handle messages from channels
func (p *PrivateAggregateProtocol) Dispatch() error {

	// 1. Aggregation announcement phase
	if !p.IsRoot() {
		p.aggregationAnnouncementPhase()
	}

	// 2. Ascending aggregation phase
	groups, aggregatedData := p.ascendingAggregationPhase()
	dbg.Lvl1(p.Entity(), "completed aggregation phase.")

	// 3. Result reporting
	if p.IsRoot() {
		p.FeedbackChannel <- CothorityAggregatedData{*groups, *aggregatedData}
	}
	return nil
}

func (p *PrivateAggregateProtocol) aggregationAnnouncementPhase() {
	dataReferenceMessage := <-p.DataReferenceChannel
	if !p.IsLeaf() {
		p.SendToChildren(&dataReferenceMessage.DataReferenceMessage)
	}
}

func (p *PrivateAggregateProtocol) ascendingAggregationPhase() (
	*map[GroupingKey]GroupingAttributes, *map[GroupingKey]CipherVector) {

	if p.GroupedData == nil { // TODO: Aggregate a zero contribution
		emptyMap := make(map[GroupingKey]CipherVector, 0)
		emptyGroupMap := make(map[GroupingKey]GroupingAttributes, 0)
		p.GroupedData = &emptyMap
		p.Groups = &emptyGroupMap
	}

	if !p.IsLeaf() {
		for _, childrenContribution := range <-p.ChildDataChannel {
			for group := range childrenContribution.ChildData {
				(*p.Groups)[group] = childrenContribution.ChildGroups[group]
				if aggr, ok := (*p.GroupedData)[group]; ok {
					localAggr := (*p.GroupedData)[group]
					localAggr.Add(localAggr, aggr)
				} else {
					(*p.GroupedData)[group] = aggr
				}
			}
		}
	}
	if !p.IsRoot() {
		p.SendToParent(&ChildAggregatedDataMessage{*p.GroupedData, *p.Groups})
	}
	return p.Groups, p.GroupedData
}
