package libmedco

import (
	"fmt"
)

//DEFAULT_GROUP defines the default grouping key. Used when a survey consists of an aggregation only (no grouping)
const DefaultGroup = GroupingKey("DefaultGroup")

//SurveyResult represents a result from a survey which is made of two ciphervector encrypted for the querier
type SurveyResult struct {
	GroupingAttributes    CipherVector
	AggregatingAttributes CipherVector
}

//SurveyStore contains all the elements of a survey, it consists of the data structure that each cothority has to
//maintain locally to perform a collective survey
type SurveyStore struct {
	ClientResponses    []ClientResponse
	DeliverableResults []SurveyResult

	//AggregatingAttributes is a map containing all the aggregating attributes of the server clients and a
	//corresponding unique ID
	AggregatingAttributes map[TempID]CipherVector

	//LocGroupingAggregating contains the results of the local aggregation
	LocGroupingAggregating map[GroupingKey]CipherVector

	//AfterAggrProto & LocGroupingGroups are the results of the split of LocGroupingAggregating, first one contains
	//aggregating attributes and the secondm the grouping attributes
	AfterAggrProto    map[GroupingKey]CipherVector
	LocGroupingGroups map[GroupingKey]GroupingAttributes

	//GroupedDeterministicGroupingAttributes & GroupedAggregatingAttributes contain results of the grouping
	//before they are key switched and combined in the last step (key switching)
	GroupedDeterministicGroupingAttributes map[TempID]GroupingAttributes
	GroupedAggregatingAttributes           map[TempID]CipherVector

	lastID uint64
}

//NewSurveyStore constructor
func NewSurveyStore() *SurveyStore {
	return &SurveyStore{
		AggregatingAttributes: make(map[TempID]CipherVector),

		LocGroupingAggregating: make(map[GroupingKey]CipherVector),
		LocGroupingGroups:      make(map[GroupingKey]GroupingAttributes),
		AfterAggrProto:         make(map[GroupingKey]CipherVector),

		GroupedDeterministicGroupingAttributes: make(map[TempID]GroupingAttributes),
		GroupedAggregatingAttributes:           make(map[TempID]CipherVector),
	}
}

//InsertClientResponse handles the local storage of a new client response
func (s *SurveyStore) InsertClientResponse(cr ClientResponse) {
	if len(cr.ProbabilisticGroupingAttributes) == 0 { //only aggregation, no grouping
		mapValue, ok := s.LocGroupingAggregating[DefaultGroup]
		if !ok {
			mapValue = cr.AggregatingAttributes
		} else {
			mapValue = *NewCipherVector(len(mapValue)).Add(mapValue, cr.AggregatingAttributes)
		}
		s.LocGroupingAggregating[DefaultGroup] = mapValue
	} else { //grouping
		s.ClientResponses = append(s.ClientResponses, cr)
	}
}

//HasNextClientResponses permits to verify if there are new client responses to process
func (s *SurveyStore) HasNextClientResponses() bool {
	return (len(s.ClientResponses) == 0)
}

//PollProbabilisticGroupingAttributes processes the client responses to construct Id -> aggr. attributes and Id -> gr.
//attributes maps
func (s *SurveyStore) PollProbabilisticGroupingAttributes() map[TempID]CipherVector {
	polledProbGroupAttr := make(map[TempID]CipherVector)
	for _, v := range s.ClientResponses {
		newID := s.nextID()
		s.AggregatingAttributes[newID] = v.AggregatingAttributes
		polledProbGroupAttr[newID] = v.ProbabilisticGroupingAttributes
	}
	s.ClientResponses = s.ClientResponses[:0] //clear table

	return polledProbGroupAttr
}

//PushDeterministicGroupingAttributes handles the reception of the switched to deterministic grouping attributes
func (s *SurveyStore) PushDeterministicGroupingAttributes(detGroupAttr map[TempID]GroupingAttributes) {

	for k, v := range detGroupAttr {
		addInMapping(s.LocGroupingAggregating, v.Key(), s.AggregatingAttributes[k])
		s.LocGroupingGroups[v.Key()] = v
	}

	s.AggregatingAttributes = make(map[TempID]CipherVector) //clear map
}

//HasNextAggregatedResponses verifies the presence of locally aggregated results
func (s *SurveyStore) HasNextAggregatedResponses() bool {
	return (len(s.LocGroupingAggregating) == 0)
}

//PollLocallyAggregatedResponses returns splitted (by group and aggr attributes) of local aggregated results
func (s *SurveyStore) PollLocallyAggregatedResponses() (map[GroupingKey]GroupingAttributes, map[GroupingKey]CipherVector) {
	LocGroupingAggregatingReturn := s.LocGroupingAggregating
	s.LocGroupingAggregating = make(map[GroupingKey]CipherVector)
	return s.LocGroupingGroups, LocGroupingAggregatingReturn

}

func (s *SurveyStore) nextID() TempID {
	s.lastID += 1
	return TempID(s.lastID)
}

func addInMapping(s map[GroupingKey]CipherVector, key GroupingKey, added CipherVector) {
	if localResult, ok := s[key]; !ok {
		s[key] = added
	} else {
		s[key] = *NewCipherVector(len(added)).Add(localResult, added)
	}
}

//PushCothorityAggregatedGroups handles the collective aggregation locally
func (s *SurveyStore) PushCothorityAggregatedGroups(gNew map[GroupingKey]GroupingAttributes, sNew map[GroupingKey]CipherVector) {
	for key, value := range sNew {
		addInMapping(s.AfterAggrProto, key, value)
		if _, ok := s.LocGroupingGroups[key]; !ok {
			s.LocGroupingGroups[key] = gNew[key]
		}
	}
}

//HasNextAggregatedGroupsId verifies that the server has local grouping results (group attributes)
func (s *SurveyStore) HasNextAggregatedGroupsID() bool {
	return (len(s.GroupedDeterministicGroupingAttributes) == 0)
}

//PollCothorityAggregatedGroupsId returns the local results of the grouping (group attributes)
func (s *SurveyStore) PollCothorityAggregatedGroupsID() map[TempID]GroupingAttributes {
	if len(s.AfterAggrProto) != 0 {
		for key, value := range s.AfterAggrProto {
			newID := s.nextID()
			s.GroupedDeterministicGroupingAttributes[newID] = s.LocGroupingGroups[key]
			s.GroupedAggregatingAttributes[newID] = value
		}
		s.AfterAggrProto = make(map[GroupingKey]CipherVector)
		s.LocGroupingGroups = make(map[GroupingKey]GroupingAttributes)
	}
	groupIDs := s.GroupedDeterministicGroupingAttributes
	s.GroupedDeterministicGroupingAttributes = make(map[TempID]GroupingAttributes)
	return groupIDs
}

//HasNextAggregatedGroupsAttr verifies that the server has local grouping results (aggregating attributes)
func (s *SurveyStore) HasNextAggregatedGroupsAttr() bool {
	return (len(s.GroupedAggregatingAttributes) == 0)
}

//PollCothorityAggregatedGroupsAttr returns the local results of the grouping (aggregating attributes)
func (s *SurveyStore) PollCothorityAggregatedGroupsAttr() map[TempID]CipherVector {
	if len(s.AfterAggrProto) != 0 {
		for key, value := range s.AfterAggrProto {
			newID := s.nextID()
			s.GroupedDeterministicGroupingAttributes[newID] = s.LocGroupingGroups[key]
			s.GroupedAggregatingAttributes[newID] = value
		}
		s.AfterAggrProto = make(map[GroupingKey]CipherVector)
		s.LocGroupingGroups = make(map[GroupingKey]GroupingAttributes)
	}
	groupAttrs := s.GroupedAggregatingAttributes
	s.GroupedAggregatingAttributes = make(map[TempID]CipherVector)
	return groupAttrs
}

//PushQuerierKeyEncryptedData handles the reception of the key switched (for the querier) results
func (s *SurveyStore) PushQuerierKeyEncryptedData(groupingAttributes map[TempID]CipherVector, aggregatingAttributes map[TempID]CipherVector) {
	for tempid, cv := range aggregatingAttributes {
		group, _ := groupingAttributes[tempid]
		s.DeliverableResults = append(s.DeliverableResults, SurveyResult{group, cv})
	}
}

//PollDeliverableResults gets the results
func (s *SurveyStore) PollDeliverableResults() []SurveyResult {
	results := s.DeliverableResults
	s.DeliverableResults = s.DeliverableResults[:0]
	return results
}

//DisplayResults shows results (debugging)
func (s *SurveyStore) DisplayResults() {
	for _, v := range s.DeliverableResults {
		fmt.Println("[ ", v.GroupingAttributes, " ] : ", v.AggregatingAttributes, ")")
	}
}
