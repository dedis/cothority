package store

import (
	"github.com/satori/go.uuid"
	"fmt"
	."github.com/dedis/cothority/services/medco/structs"
)

const AGGREGATION_ID int = 0
type Survey_Database []Survey

type Survey struct {
	Id uuid.UUID
	ClientResponses []ClientResponse //a
	DeliverableResults []SurveyResult //d & 6

	ProbabilisticGroupingAttributes map[uuid.UUID]CipherVector //1
	AggregatingAttributes map[uuid.UUID]CipherVector //2
	
	LocGroupingResults map[GroupingAttributes]CipherVector //b & c
	
	GroupedDeterministicGroupingAttributes map[uuid.UUID]GroupingAttributes //4
	GroupedAggregatingAttributes map[uuid.UUID]CipherVector // 5
}

//construct survey	
func NewSurvey() *Survey {
	return &Survey{

		Id : uuid.NewV4(),

		ProbabilisticGroupingAttributes : make(map[uuid.UUID]CipherVector),
		AggregatingAttributes : make(map[uuid.UUID]CipherVector),

		LocGroupingResults : make(map[GroupingAttributes]CipherVector),
	
		GroupedDeterministicGroupingAttributes : make(map[uuid.UUID]GroupingAttributes),
		GroupedAggregatingAttributes : make(map[uuid.UUID]CipherVector),
	}
}

func (s *Survey) InsertClientResponse(cr ClientResponse){
	if cr.ProbabilisticGroupingAttributes == nil { //only aggregation, no grouping
		if len(s.DeliverableResults) != 0 {
			s.DeliverableResults[AGGREGATION_ID].AggregatingAttributes.Add(s.DeliverableResults[AGGREGATION_ID].AggregatingAttributes, cr.AggregatingAttributes)
		} else  {
			s.DeliverableResults = append(s.DeliverableResults, SurveyResult{nil, cr.AggregatingAttributes})
		}
	} else {//grouping
		s.ClientResponses = append(s.ClientResponses, cr)
	}
}


func (s *Survey) PollProbabilisticGroupingAttributes() *map[uuid.UUID]CipherVector{
	for _,v := range s.ClientResponses{
		newId := uuid.NewV4()
		_,ok := s.ProbabilisticGroupingAttributes[newId]
		for  ok {
			newId = uuid.NewV4()
			_,ok = s.ProbabilisticGroupingAttributes[newId]
		}
		s.AggregatingAttributes[newId] = v.AggregatingAttributes
		s.ProbabilisticGroupingAttributes[newId] = v.ProbabilisticGroupingAttributes
	}
	s.ClientResponses = s.ClientResponses[:0] //clear table
	
	return &s.ProbabilisticGroupingAttributes
}


func (s *Survey) PushDeterministicGroupingAttributes(detGroupAttr map[uuid.UUID]GroupingAttributes) {
	for k,v := range detGroupAttr{
			AddInMapping(s.LocGroupingResults, v, s.AggregatingAttributes[k])
	}
	
	s.AggregatingAttributes = make(map[uuid.UUID]CipherVector) //clear maps
	s.ProbabilisticGroupingAttributes = make(map[uuid.UUID]CipherVector)
}

func (s *Survey) PollLocallyAggregatedResponses()  *map[GroupingAttributes]CipherVector {
	return &s.LocGroupingResults
}

func AddInMapping (s map[GroupingAttributes]CipherVector, key GroupingAttributes, added CipherVector){
	var tempPointer *CipherVector
	if _,ok := s[key]; !ok{
		s[key] = added
	} else {
		tempVar := s[key]
		tempPointer = &tempVar
		tempPointer.Add(*tempPointer,added)
		s[key] = *tempPointer
	}
}


func (s *Survey) PushCothorityAggregatedGroups(sNew map[GroupingAttributes]CipherVector ){
	for key, value := range sNew {
		AddInMapping(s.LocGroupingResults, key, value)
	}
}


func (s *Survey) PollCothorityAggregatedGroups() (*map[uuid.UUID]GroupingAttributes, *map[uuid.UUID]CipherVector) {
	for key,value := range s.LocGroupingResults{
		newId := uuid.NewV4()
		_,ok := s.GroupedDeterministicGroupingAttributes[newId]
		for  ok {
			newId = uuid.NewV4()
			_,ok = s.GroupedDeterministicGroupingAttributes[newId]
		}
		s.GroupedDeterministicGroupingAttributes[newId] = key
		s.GroupedAggregatingAttributes[newId] = value
	}
	s.LocGroupingResults = make(map[GroupingAttributes]CipherVector)
	
	return &s.GroupedDeterministicGroupingAttributes, &s.GroupedAggregatingAttributes
}


func (s *Survey) PushQuerierKeyEncryptedData(groupingAttributes map[uuid.UUID] CipherVector, aggregatingAttributes map[uuid.UUID]CipherVector){
	for key,value := range groupingAttributes {
		s.DeliverableResults = append(s.DeliverableResults, SurveyResult{value, aggregatingAttributes[key]})
	}
	s.GroupedDeterministicGroupingAttributes = make(map[uuid.UUID]GroupingAttributes)
	s.GroupedAggregatingAttributes = make(map[uuid.UUID]CipherVector)
}


func (s *Survey) PollDeliverableResults()[]SurveyResult{
	return s.DeliverableResults
}


func (s *Survey) DisplayResults(){
	for _,v := range s.DeliverableResults{
		fmt.Println("[ ", v.GroupingAttributes, " ] : ", v.AggregatingAttributes, ")")
	}
}
