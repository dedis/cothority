package store

import (
	"fmt"
	. "github.com/dedis/cothority/services/medco/structs"
	"github.com/satori/go.uuid"
)

const AGGREGATION_ID int = 0

type Survey_Database []Survey

type Survey struct {
	Id                 uuid.UUID
	ClientResponses    []ClientResponse //a
	DeliverableResults []SurveyResult   //d & 6

	ProbabilisticGroupingAttributes map[TempID]CipherVector //1
	AggregatingAttributes           map[TempID]CipherVector //2

	LocGroupingAggregating map[GroupingKey]CipherVector //b & c
	AfterAggrProto         map[GroupingKey]CipherVector
	LocGroupingGroups      map[GroupingKey]GroupingAttributes

	GroupedDeterministicGroupingAttributes map[TempID]GroupingAttributes //4
	GroupedAggregatingAttributes           map[TempID]CipherVector       // 5

	lastId uint64
}

//construct survey
func NewSurvey() *Survey {
	return &Survey{

		Id: uuid.NewV4(),

		ProbabilisticGroupingAttributes: make(map[TempID]CipherVector),
		AggregatingAttributes:           make(map[TempID]CipherVector),

		LocGroupingAggregating: make(map[GroupingKey]CipherVector),
		LocGroupingGroups:      make(map[GroupingKey]GroupingAttributes),
		AfterAggrProto:         make(map[GroupingKey]CipherVector),

		GroupedDeterministicGroupingAttributes: make(map[TempID]GroupingAttributes),
		GroupedAggregatingAttributes:           make(map[TempID]CipherVector),
	}
}

func (s *Survey) InsertClientResponse(cr ClientResponse) {
	if cr.ProbabilisticGroupingAttributes == nil { //only aggregation, no grouping
		if len(s.DeliverableResults) != 0 {
			s.DeliverableResults[AGGREGATION_ID].AggregatingAttributes.Add(s.DeliverableResults[AGGREGATION_ID].AggregatingAttributes, cr.AggregatingAttributes)
		} else {
			s.DeliverableResults = append(s.DeliverableResults, SurveyResult{nil, cr.AggregatingAttributes})
		}
	} else { //grouping
		s.ClientResponses = append(s.ClientResponses, cr)
	}
}

func (s *Survey) PollProbabilisticGroupingAttributes() *map[TempID]CipherVector {
	for _, v := range s.ClientResponses {
		newId := s.nextId()
		s.AggregatingAttributes[newId] = v.AggregatingAttributes
		s.ProbabilisticGroupingAttributes[newId] = v.ProbabilisticGroupingAttributes
	}
	s.ClientResponses = s.ClientResponses[:0] //clear table

	return &s.ProbabilisticGroupingAttributes
}

func (s *Survey) PushDeterministicGroupingAttributes(detGroupAttr map[TempID]GroupingAttributes) {

	for k, v := range detGroupAttr {
		AddInMapping(s.LocGroupingAggregating, v.Key(), s.AggregatingAttributes[k])
		s.LocGroupingGroups[v.Key()] = v
	}

	s.AggregatingAttributes = make(map[TempID]CipherVector) //clear maps
	s.ProbabilisticGroupingAttributes = make(map[TempID]CipherVector)
}

func (s *Survey) PollLocallyAggregatedResponses() (*map[GroupingKey]GroupingAttributes, *map[GroupingKey]CipherVector) {
	return &s.LocGroupingGroups, &s.LocGroupingAggregating
}

func (s *Survey) nextId() TempID {
	s.lastId += 1
	return TempID(s.lastId)
}

func AddInMapping(s map[GroupingKey]CipherVector, key GroupingKey, added CipherVector) {
	//var tempPointer *CipherVector
	//tempPointer = nil
	if _, ok := s[key]; !ok {
		s[key] = added
	} else {
		result := Add2(s[key], added)
		s[key] = result
	}
}

func (s *Survey) PushCothorityAggregatedGroups(gNew map[GroupingKey]GroupingAttributes, sNew map[GroupingKey]CipherVector) {
	for key, value := range sNew {
		_ = value
		AddInMapping(s.AfterAggrProto, key, value)
		if _, ok := s.LocGroupingGroups[key]; !ok {
			s.LocGroupingGroups[key] = gNew[key]
		}
	}
	s.LocGroupingAggregating = make(map[GroupingKey]CipherVector)
}

func (s *Survey) PollCothorityAggregatedGroups() (*map[TempID]GroupingAttributes, *map[TempID]CipherVector) {
	for key, value := range s.AfterAggrProto {
		newId := s.nextId()
		s.GroupedDeterministicGroupingAttributes[newId] = s.LocGroupingGroups[key]
		s.GroupedAggregatingAttributes[newId] = value
	}
	s.AfterAggrProto = make(map[GroupingKey]CipherVector)
	s.LocGroupingGroups = make(map[GroupingKey]GroupingAttributes)
	return &s.GroupedDeterministicGroupingAttributes, &s.GroupedAggregatingAttributes
}

func (s *Survey) PushQuerierKeyEncryptedData(groupingAttributes map[TempID]CipherVector, aggregatingAttributes map[TempID]CipherVector) {
	for key, value := range groupingAttributes {
		s.DeliverableResults = append(s.DeliverableResults, SurveyResult{value, aggregatingAttributes[key]})
	}
	s.GroupedDeterministicGroupingAttributes = make(map[TempID]GroupingAttributes)
	s.GroupedAggregatingAttributes = make(map[TempID]CipherVector)
}

func (s *Survey) PollDeliverableResults() []SurveyResult {
	return s.DeliverableResults
}

func (s *Survey) DisplayResults() {
	for _, v := range s.DeliverableResults {
		fmt.Println("[ ", v.GroupingAttributes, " ] : ", v.AggregatingAttributes, ")")
	}
}
