package medco_structs

import (
	"fmt"
)

const AGGREGATION_ID int = 0

type Survey_Database []SurveyStore

type SurveyResult struct{
	GroupingAttributes CipherVector
	AggregatingAttributes CipherVector
}

type SurveyStore struct {
	ClientResponses    []ClientResponse //a
	DeliverableResults []SurveyResult   //d & 6

	//ProbabilisticGroupingAttributes map[TempID]CipherVector //1
	AggregatingAttributes           map[TempID]CipherVector //2

	LocGroupingAggregating map[GroupingKey]CipherVector //b & c
	AfterAggrProto         map[GroupingKey]CipherVector
	LocGroupingGroups      map[GroupingKey]GroupingAttributes

	GroupedDeterministicGroupingAttributes map[TempID]GroupingAttributes //4
	GroupedAggregatingAttributes           map[TempID]CipherVector       // 5

	lastId uint64
}

//construct survey
func NewSurveyStore() *SurveyStore {
	return &SurveyStore{
		//ProbabilisticGroupingAttributes: make(map[TempID]CipherVector),
		AggregatingAttributes:           make(map[TempID]CipherVector),

		LocGroupingAggregating: make(map[GroupingKey]CipherVector),
		LocGroupingGroups:      make(map[GroupingKey]GroupingAttributes),
		AfterAggrProto:         make(map[GroupingKey]CipherVector),

		GroupedDeterministicGroupingAttributes: make(map[TempID]GroupingAttributes),
		GroupedAggregatingAttributes:           make(map[TempID]CipherVector),
	}
}

func (s *SurveyStore) InsertClientResponse(cr ClientResponse) {
	if len(cr.ProbabilisticGroupingAttributes) == 0 { //only aggregation, no grouping
		if len(s.DeliverableResults) != 0 {
			s.DeliverableResults[AGGREGATION_ID].AggregatingAttributes.Add(s.DeliverableResults[AGGREGATION_ID].AggregatingAttributes, cr.AggregatingAttributes)
		} else {
			s.DeliverableResults = append(s.DeliverableResults, SurveyResult{CipherVector{}, cr.AggregatingAttributes})
		}
	} else { //grouping
		s.ClientResponses = append(s.ClientResponses, cr)
	}
}

func (s *SurveyStore) HasNextClientResponses() bool {
	return (len(s.ClientResponses) == 0)
}

func (s *SurveyStore) PollProbabilisticGroupingAttributes() map[TempID]CipherVector {
	polledProbGroupAttr := make(map[TempID]CipherVector)
	for _, v := range s.ClientResponses {
		newId := s.nextId()
		s.AggregatingAttributes[newId] = v.AggregatingAttributes
		polledProbGroupAttr[newId] = v.ProbabilisticGroupingAttributes
	}
	s.ClientResponses = s.ClientResponses[:0] //clear table

	return polledProbGroupAttr
}

func (s *SurveyStore) PushDeterministicGroupingAttributes(detGroupAttr map[TempID]GroupingAttributes) {

	for k, v := range detGroupAttr {
		AddInMapping(s.LocGroupingAggregating, v.Key(), s.AggregatingAttributes[k])
		s.LocGroupingGroups[v.Key()] = v
	}

	s.AggregatingAttributes = make(map[TempID]CipherVector) //clear maps
	//s.ProbabilisticGroupingAttributes = make(map[TempID]CipherVector)
}

func (s *SurveyStore) HasNextAggregatedResponses() bool {
	return (len(s.LocGroupingGroups) == 0)
}

func (s *SurveyStore) PollLocallyAggregatedResponses() (map[GroupingKey]GroupingAttributes, map[GroupingKey]CipherVector) {
	LocGroupingAggregatingReturn := s.LocGroupingAggregating
	s.LocGroupingAggregating = make(map[GroupingKey]CipherVector)
	return s.LocGroupingGroups, LocGroupingAggregatingReturn

}

func (s *SurveyStore) nextId() TempID {
	s.lastId += 1
	return TempID(s.lastId)
}

func AddInMapping(s map[GroupingKey]CipherVector, key GroupingKey, added CipherVector) {
	if _, ok := s[key]; !ok {
		s[key] = added
	} else {
		result := added.AddNoReplace(s[key], added)
		s[key] = result
	}
}

func (s *SurveyStore) PushCothorityAggregatedGroups(gNew map[GroupingKey]GroupingAttributes, sNew map[GroupingKey]CipherVector) {
	for key, value := range sNew {
		_ = value
		AddInMapping(s.AfterAggrProto, key, value)
		if _, ok := s.LocGroupingGroups[key]; !ok {
			s.LocGroupingGroups[key] = gNew[key]
		}
	}
	//s.LocGroupingAggregating = make(map[GroupingKey]CipherVector)
}

func (s *SurveyStore) HasNextAggregatedGroupsId() bool {
	return (len(s.GroupedDeterministicGroupingAttributes) == 0)
}
func (s *SurveyStore) PollCothorityAggregatedGroupsId() map[TempID]GroupingAttributes{
	if len(s.AfterAggrProto) != 0 {
		for key, value := range s.AfterAggrProto {
			newId := s.nextId()
			s.GroupedDeterministicGroupingAttributes[newId] = s.LocGroupingGroups[key]
			s.GroupedAggregatingAttributes[newId] = value
		}
		s.AfterAggrProto = make(map[GroupingKey]CipherVector)
		s.LocGroupingGroups = make(map[GroupingKey]GroupingAttributes)
	}
	groupIds := s.GroupedDeterministicGroupingAttributes
	s.GroupedDeterministicGroupingAttributes = make(map[TempID]GroupingAttributes)
	return groupIds
}

func (s *SurveyStore) HasNextAggregatedGroupsAttr() bool {
	return (len(s.GroupedAggregatingAttributes) == 0)
}
func (s *SurveyStore) PollCothorityAggregatedGroupsAttr() map[TempID]CipherVector {
	if len(s.AfterAggrProto) != 0 {
		for key, value := range s.AfterAggrProto {
			newId := s.nextId()
			s.GroupedDeterministicGroupingAttributes[newId] = s.LocGroupingGroups[key]
			s.GroupedAggregatingAttributes[newId] = value
		}
		s.AfterAggrProto = make(map[GroupingKey]CipherVector)
		s.LocGroupingGroups = make(map[GroupingKey]GroupingAttributes)
	}
	groupAttrs:= s.GroupedAggregatingAttributes
	s.GroupedAggregatingAttributes = make(map[TempID]CipherVector)
	return groupAttrs
}

func (s *SurveyStore) PushQuerierKeyEncryptedData(groupingAttributes map[TempID]CipherVector, aggregatingAttributes map[TempID]CipherVector) {
	for key, value := range groupingAttributes {
		s.DeliverableResults = append(s.DeliverableResults, SurveyResult{value, aggregatingAttributes[key]})
	}
	//s.GroupedDeterministicGroupingAttributes = make(map[TempID]GroupingAttributes)
	//s.GroupedAggregatingAttributes = make(map[TempID]CipherVector)
}

func (s *SurveyStore) PollDeliverableResults() []SurveyResult {
	results := s.DeliverableResults
	s.DeliverableResults = s.DeliverableResults[:0]
	return results
}


func (s *SurveyStore) DisplayResults() {
	for _, v := range s.DeliverableResults {
		fmt.Println("[ ", v.GroupingAttributes, " ] : ", v.AggregatingAttributes, ")")
	}
}
