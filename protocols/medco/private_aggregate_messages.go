package medco

import "github.com/dedis/cothority/lib/sda"

type DataReferenceMessage struct {
	DataReference DataRef
}

type DataReferenceStruct struct {
	*sda.TreeNode
	DataReferenceMessage
}

type ChildAggregatedDataMessage struct {
	ChildData CipherVector
}

type ChildAggregatedDataStruct struct {
	*sda.TreeNode
	ChildAggregatedDataMessage
}
