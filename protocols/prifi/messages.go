package prifi

import "github.com/dedis/cothority/lib/sda"

type DataUp struct {
	Data int
}

type StructDataUp struct {
	*sda.TreeNode
	DataUp
}

type DataDown struct {
	Data int
}

type StructDataDown struct {
	*sda.TreeNode
	DataDown
}
