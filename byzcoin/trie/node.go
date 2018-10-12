package trie

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"hash"

	"github.com/dedis/protobuf"
)

type nodeType int

const (
	typeInterior nodeType = iota
	typeEmpty
	typeLeaf
)

func (ty nodeType) toBytes() []byte {
	switch ty {
	case typeInterior:
		return []byte{0}
	case typeEmpty:
		return []byte{1}
	case typeLeaf:
		return []byte{2}
	default:
		panic("no such type")
	}
}

type interiorNode struct {
	Left  []byte
	Right []byte
}

func (n *interiorNode) hash() []byte {
	h := sha256.New()
	h.Write(n.Left)
	h.Write(n.Right)
	return h.Sum(nil)
}

func (n *interiorNode) encode() ([]byte, error) {
	buf, err := protobuf.Encode(n)
	if err != nil {
		return nil, err
	}
	return append(typeInterior.toBytes(), buf...), nil
}

func newInteriorNode(left, right []byte) interiorNode {
	return interiorNode{
		Left:  left,
		Right: right,
	}
}

func decodeInteriorNode(buf []byte) (interiorNode, error) {
	if len(buf) == 0 {
		return interiorNode{}, errors.New("empty buffer")
	}
	if nodeType(buf[0]) != typeInterior {
		return interiorNode{}, errors.New("wrong node type")
	}
	var node interiorNode
	if err := protobuf.Decode(buf[1:], &node); err != nil {
		return interiorNode{}, err
	}
	return node, nil
}

type emptyNode struct {
	Prefix []bool
}

func newEmptyNode(prefix []bool) emptyNode {
	return emptyNode{
		Prefix: prefix,
	}
}

func (n *emptyNode) hash(nonce []byte) []byte {
	h := n.hashInterface(nonce)
	return h.Sum(nil)
}

func (n *emptyNode) hashInterface(nonce []byte) hash.Hash {
	h := sha256.New()
	h.Write(typeEmpty.toBytes())
	h.Write(nonce)
	h.Write(toByteSlice(n.Prefix))

	lBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(lBuf, uint32(len(n.Prefix)))
	h.Write(lBuf)

	return h
}

func (n *emptyNode) encode() ([]byte, error) {
	buf, err := protobuf.Encode(n)
	if err != nil {
		return nil, err
	}
	return append(typeEmpty.toBytes(), buf...), nil
}

func decodeEmptyNode(buf []byte) (emptyNode, error) {
	if len(buf) == 0 {
		return emptyNode{}, errors.New("empty buffer")
	}
	if nodeType(buf[0]) != typeEmpty {
		return emptyNode{}, errors.New("wrong node type")
	}
	var node emptyNode
	if err := protobuf.Decode(buf[1:], &node); err != nil {
		return emptyNode{}, err
	}
	return node, nil
}

type leafNode struct {
	emptyNode
	Key   []byte
	Value []byte
}

func (n *leafNode) hash(nonce []byte) []byte {
	h := n.hashInterface(nonce)
	h.Write(n.Key)
	h.Write(n.Value)
	return h.Sum(nil)
}

func newLeafNode(prefix []bool, key []byte, value []byte) leafNode {
	return leafNode{
		emptyNode: newEmptyNode(prefix),
		Key:       key,
		Value:     value,
	}
}

func (n *leafNode) encode() ([]byte, error) {
	buf, err := protobuf.Encode(n)
	if err != nil {
		return nil, err
	}
	return append(typeLeaf.toBytes(), buf...), nil
}

func decodeLeafNode(buf []byte) (leafNode, error) {
	if len(buf) == 0 {
		return leafNode{}, errors.New("empty buffer")
	}
	if nodeType(buf[0]) != typeLeaf {
		return leafNode{}, errors.New("wrong node type")
	}
	var node leafNode
	if err := protobuf.Decode(buf[1:], &node); err != nil {
		return leafNode{}, err
	}
	return node, nil
}
