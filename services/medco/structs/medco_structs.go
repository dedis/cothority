package medco_structs

import (
	"fmt"
)

const MAX_GROUP_ATTR int = 2
const PROOF = false

type GroupingAttributes []DeterministCipherText
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
	equal := true
	for i, attr := range *ga {
		equal = equal && attr.Equals(&(*ga2)[i])
	}
	return equal
}
