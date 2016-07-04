// The private aggregate protocol permits the cothority to collectively aggregate the local
// results of all the servers.
// It uses the tree structure of the cothority. The root sends down an aggregation trigger message. The leafs
// respond with their local result and other nodes aggregate what they receive before forwarding the
// aggregation result up the tree until the root can produce the final result.

package medco

import (
	"errors"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/cothority/services/medco/libmedco"
)

// PrivateAggregateProtocolName is the registered name for the private aggregate protocol.
const PrivateAggregateProtocolName = "PrivateAggregate"

// DataReferenceMessage message sent to trigger an aggregation protocol.
type DataReferenceMessage struct {
}

// ChildAggregatedDataMessage contains one node's aggregated data.
type ChildAggregatedDataMessage struct {
	ChildData   map[libmedco.GroupingKey]libmedco.CipherVector
	ChildGroups map[libmedco.GroupingKey]libmedco.GroupingAttributes
}

type dataReferenceStruct struct {
	*sda.TreeNode
	DataReferenceMessage
}

type childAggregatedDataStruct struct {
	*sda.TreeNode
	ChildAggregatedDataMessage
}

// CothorityAggregatedData is the collective aggregation result.
type CothorityAggregatedData struct {
	Groups      map[libmedco.GroupingKey]libmedco.GroupingAttributes
	GroupedData map[libmedco.GroupingKey]libmedco.CipherVector
}

func init() {
	network.RegisterMessageType(DataReferenceMessage{})
	network.RegisterMessageType(ChildAggregatedDataMessage{})
	sda.ProtocolRegisterName(PrivateAggregateProtocolName, NewPrivateAggregate)
}

// PrivateAggregateProtocol performs an aggregation of the data held by every node in the cothority.
type PrivateAggregateProtocol struct {
	*sda.TreeNodeInstance

	// Protocol feedback channel
	FeedbackChannel chan CothorityAggregatedData

	// Protocol communication channels
	DataReferenceChannel chan dataReferenceStruct
	ChildDataChannel     chan []childAggregatedDataStruct

	// Protocol state data
	GroupedData *map[libmedco.GroupingKey]libmedco.CipherVector
	Groups      *map[libmedco.GroupingKey]libmedco.GroupingAttributes
}

// NewPrivateAggregate initializes the protocol instance.
func NewPrivateAggregate(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
	pap := &PrivateAggregateProtocol{
		TreeNodeInstance: n,
		FeedbackChannel:  make(chan CothorityAggregatedData),
	}

	err := pap.RegisterChannel(&pap.DataReferenceChannel)
	if  err != nil {
		return nil, errors.New("couldn't register data reference channel: " + err.Error())
	}

	err = pap.RegisterChannel(&pap.ChildDataChannel)
	if  err != nil {
		return nil, errors.New("couldn't register child-data channel: " + err.Error())
	}

	return pap, nil
}

// Start is called at the root to begin the execution of the protocol.
func (p *PrivateAggregateProtocol) Start() error {
	if p.GroupedData == nil {
		return errors.New("No data reference provided for aggregation.")
	}

	log.Lvl1(p.ServerIdentity(), "started a Private Aggregate Protocol (", len(*p.GroupedData), " local groups )")

	p.SendToChildren(&DataReferenceMessage{})

	return nil
}

// Dispatch is called at each node and handle incoming messages.
func (p *PrivateAggregateProtocol) Dispatch() error {

	// 1. Aggregation announcement phase
	if !p.IsRoot() {
		p.aggregationAnnouncementPhase()
	}

	// 2. Ascending aggregation phase
	groups, aggregatedData := p.ascendingAggregationPhase()
	log.Lvl1(p.ServerIdentity(), "completed aggregation phase (", len(*aggregatedData), "groups )")

	// 3. Result reporting
	if p.IsRoot() {
		p.FeedbackChannel <- CothorityAggregatedData{*groups, *aggregatedData}
	}
	return nil
}

// Announce forwarding down the tree.
func (p *PrivateAggregateProtocol) aggregationAnnouncementPhase() {
	dataReferenceMessage := <-p.DataReferenceChannel
	if !p.IsLeaf() {
		p.SendToChildren(&dataReferenceMessage.DataReferenceMessage)
	}
}

// Results pushing up the tree containing aggregation results.
func (p *PrivateAggregateProtocol) ascendingAggregationPhase() (
	*map[libmedco.GroupingKey]libmedco.GroupingAttributes, *map[libmedco.GroupingKey]libmedco.CipherVector) {
	if p.GroupedData == nil {
		emptyMap := make(map[libmedco.GroupingKey]libmedco.CipherVector, 0)
		emptyGroupMap := make(map[libmedco.GroupingKey]libmedco.GroupingAttributes, 0)
		p.GroupedData = &emptyMap
		p.Groups = &emptyGroupMap
	}

	if !p.IsLeaf() {
		for _, childrenContribution := range <-p.ChildDataChannel {
			for group, aggr := range childrenContribution.ChildData {
				(*p.Groups)[group] = childrenContribution.ChildGroups[group]
				if localAggr, ok := (*p.GroupedData)[group]; ok {
					(*p.GroupedData)[group] = *libmedco.NewCipherVector(len(aggr)).Add(localAggr, aggr)
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
