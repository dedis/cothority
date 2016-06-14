package medco_structs

import (
	"fmt"
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

func (ga *GroupingAttributes) Key() GroupingKey {
	return GroupingKey(fmt.Sprint(ga))
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