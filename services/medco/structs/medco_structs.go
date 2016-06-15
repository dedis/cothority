package medco_structs

import (
	"strings"
	"github.com/dedis/cothority/services/medco/store"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/cothority/lib/sda"
	"github.com/satori/go.uuid"
)

const MAX_GROUP_ATTR int = 2
const PROOF = false

type GroupingAttributes DeterministCipherVector
type GroupingKey string
type TempID uint64

type ClientResponse struct {
	ProbabilisticGroupingAttributes CipherVector
	AggregatingAttributes           CipherVector
}

type SurveyID struct {
	*uuid.UUID
}

type Survey struct {
	*store.SurveyStore
	ID SurveyID
	EntityList   *sda.EntityList
	SurveyPHKey  abstract.Secret
	ClientPublic abstract.Point
	SurveyDescription SurveyDescription
}

type SurveyDescription struct {
	GroupingAttributesCount uint
	AggregatingAttributesCount uint
}

func (ga *GroupingAttributes) Key() GroupingKey {
	var key []string
	for _, a := range DeterministCipherVector(*ga) {
		key = append(key, a.C.String())
	}
	return GroupingKey(strings.Join(key,""))
}

func (ga *GroupingAttributes) Equal(ga2 *GroupingAttributes) bool{
	if ga == nil || ga2 == nil {
		return ga == ga2
	}
	return ga.Equal(ga2)
}

func GroupingAttributesToDeterministicCipherVector(ga *map[TempID]GroupingAttributes) *map[TempID]DeterministCipherVector {
	deterministicCipherVector := make(map[TempID]DeterministCipherVector, len(*ga))
	for k := range *ga {
		deterministicCipherVector[k] = DeterministCipherVector((*ga)[k])
	}
	return &deterministicCipherVector
}

func DeterministicCipherVectorToGroupingAttributes(dcv *map[TempID]DeterministCipherVector) *map[TempID]GroupingAttributes {
	deterministicGroupAttributes := make(map[TempID]GroupingAttributes, len(*dcv))
	for k := range *dcv {
		deterministicGroupAttributes[k] = GroupingAttributes((*dcv)[k])
	}
	return &deterministicGroupAttributes
}