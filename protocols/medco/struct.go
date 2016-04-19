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

type ElGamalQueryMessage struct {
	*VisitorMessage
	Query CipherText
	Buckets []int64
	Public abstract.Point
}

type ElGamalQueryStruct struct {
	*sda.TreeNode
	ElGamalQueryMessage
}

type PHQueryMessage struct {
	Query DeterministCipherText
	Buckets []int64
	Public abstract.Point
}

type PHQueryStruct struct {
	*sda.TreeNode
	PHQueryMessage
}

type ElGamalData struct {
	Buckets CipherVector
	Code CipherText
}

type ElGamalDataMessage struct {
	*VisitorMessage
	ElGamalData
}

type ElGamalDataStruct struct {
	*sda.TreeNode
	ElGamalDataMessage
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