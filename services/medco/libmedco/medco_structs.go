package libmedco

import (
	"strings"

	"github.com/dedis/cothority/sda"
	"github.com/dedis/crypto/abstract"
)

// PROOF is true if we use protocols with proofs (ZKPs).
const PROOF = false

// GroupingAttributes are attributes involved in grouping.
type GroupingAttributes DeterministCipherVector

// GroupingKey is an ID corresponding to grouping attributes.
type GroupingKey string

// TempID unique ID used in related maps which is used when we split a map in two associated maps.
type TempID uint64

// ClientResponse represents a client response.
type ClientResponse struct {
	ProbabilisticGroupingAttributes CipherVector
	AggregatingAttributes           CipherVector
}

// SurveyID unique ID for each survey.
type SurveyID string

// Survey represents a survey with the corresponding params where PH key is different for each server.
type Survey struct {
	*SurveyStore
	ID                SurveyID
	Roster            sda.Roster
	SurveyPHKey       abstract.Scalar
	ClientPublic      abstract.Point
	SurveyDescription SurveyDescription
}

// SurveyDescription is currently only used to define a client response format.
type SurveyDescription struct {
	GroupingAttributesCount    int32
	AggregatingAttributesCount uint32
}

// Key is used in order to get a map-friendly representation of grouping attributes to be used as keys.
func (ga *GroupingAttributes) Key() GroupingKey {
	var key []string
	for _, a := range DeterministCipherVector(*ga) {
		key = append(key, a.String())
	}
	return GroupingKey(strings.Join(key, ""))
}

// Equal checks deterministic grouping attributes for equality.
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

// GroupingAttributesToDeterministicCipherVector converts grouping attributes to a deterministic vector object.
func GroupingAttributesToDeterministicCipherVector(ga *map[TempID]GroupingAttributes) *map[TempID]DeterministCipherVector {
	deterministicCipherVector := make(map[TempID]DeterministCipherVector, len(*ga))
	for k := range *ga {
		deterministicCipherVector[k] = DeterministCipherVector((*ga)[k])
	}
	return &deterministicCipherVector
}

// DeterministicCipherVectorToGroupingAttributes converts deterministic ciphervector to grouping attributes.
func DeterministicCipherVectorToGroupingAttributes(dcv *map[TempID]DeterministCipherVector) *map[TempID]GroupingAttributes {
	deterministicGroupAttributes := make(map[TempID]GroupingAttributes, len(*dcv))
	for k := range *dcv {
		deterministicGroupAttributes[k] = GroupingAttributes((*dcv)[k])
	}
	return &deterministicGroupAttributes
}
