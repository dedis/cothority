package libmedco

import (
	"strings"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/cothority/sda"
)


//PROOF is true if we use protocols with proofs (ZKPs)
const PROOF = false

type GroupingAttributes DeterministCipherVector
type GroupingKey string
type TempID uint64

type ClientResponse struct {
	ProbabilisticGroupingAttributes CipherVector
	AggregatingAttributes           CipherVector
}

type SurveyID string

//Survey represents a survey with the corresponding params. Each node should have one PH contribution per survey.
type Survey struct {
	*SurveyStore
	ID                SurveyID
	Roster            sda.Roster
	SurveyPHKey       abstract.Scalar
	ClientPublic      abstract.Point
	SurveyDescription SurveyDescription
}

//SurveyDescription permits to define a survey by describing a client response format
type SurveyDescription struct {
	GroupingAttributesCount    int32
	AggregatingAttributesCount uint32
}


//Key permits to derive a key from grouping attributes (encrypted deterministically)
func (ga *GroupingAttributes) Key() GroupingKey {
	var key []string
	for _, a := range DeterministCipherVector(*ga) {
		key = append(key, a.String())
	}
	return GroupingKey(strings.Join(key, ""))
}

//Equal verifies equality between deterministic grouping attributes
func (ga *GroupingAttributes) Equal(ga2 *GroupingAttributes) bool {
	if ga == nil || ga2 == nil {
		return ga == ga2
	}
	for i, v := range DeterministCipherVector(*ga) {
		temp := (*ga2)[i]
		if !v.Equal(&temp) {
			return false
		}
	}
	return true
}

//GroupingAttributesToDeterministicCipherVector converses grouping attributes to a deterministic vector object
func GroupingAttributesToDeterministicCipherVector(ga *map[TempID]GroupingAttributes) *map[TempID]DeterministCipherVector {
	deterministicCipherVector := make(map[TempID]DeterministCipherVector, len(*ga))
	for k := range *ga {
		deterministicCipherVector[k] = DeterministCipherVector((*ga)[k])
	}
	return &deterministicCipherVector
}

//DeterministicCipherVectorToGroupingAttributes converses deterministic ciphervector to grouping attributes
func DeterministicCipherVectorToGroupingAttributes(dcv *map[TempID]DeterministCipherVector) *map[TempID]GroupingAttributes {
	deterministicGroupAttributes := make(map[TempID]GroupingAttributes, len(*dcv))
	for k := range *dcv {
		deterministicGroupAttributes[k] = GroupingAttributes((*dcv)[k])
	}
	return &deterministicGroupAttributes
}
