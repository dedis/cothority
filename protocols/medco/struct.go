package medco

import (
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
)

type VisitorMessageI interface {
	SetVisited(n *sda.TreeNode, tree *sda.Tree)
	AlreadyVisited(n *sda.TreeNode, tree *sda.Tree) bool
}

type VisitorMessage struct {
	VisitedSet NodeSet
}

type QueryMessage struct {
	*VisitorMessage
	Filter             CipherText
	BucketsDescription []int64
	QuerierPublicKey   abstract.Point
}

type QueryStruct struct {
	*sda.TreeNode
	QueryMessage
}

type ProcessableQueryMessage struct {
	Query DeterministCipherText
	Buckets []int64
	Public abstract.Point
}

type ProcessableQueryStruct struct {
	*sda.TreeNode
	ProcessableQueryMessage
}

type HolderResponseData struct {
	Buckets CipherVector
	Code CipherText
}

type HolderResponseDataMessage struct {
	*VisitorMessage
	HolderResponseData
}

type HolderResponseDataStruct struct {
	*sda.TreeNode
	HolderResponseDataMessage
}

type ResultMessage struct {
	Result CipherVector
}

type ResultStruct struct {
	*sda.TreeNode
	ResultMessage
}

func (vm *VisitorMessage) SetVisited(n *sda.TreeNode, tree *sda.Tree) {
	vm.VisitedSet.Add(n, tree)
}

func (vm *VisitorMessage) AlreadyVisited(n *sda.TreeNode, tree *sda.Tree) bool {
	return vm.VisitedSet.Contains(n, tree)
}