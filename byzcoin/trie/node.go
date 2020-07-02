package trie

import (
	"crypto/sha256"
	"encoding/binary"

	"errors"
	"go.dedis.ch/protobuf"
)

type nodeType int

const (
	typeInterior nodeType = iota + 1
	typeEmpty
	typeLeaf
)

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
	return append([]byte{byte(typeInterior)}, buf...), nil
}

func newInteriorNode(left, right []byte) interiorNode {
	return interiorNode{
		Left:  left,
		Right: right,
	}
}

func decodeInteriorNode(buf []byte) (node interiorNode, err error) {
	if len(buf) == 0 {
		err = errors.New("empty buffer")
		return
	}
	if nodeType(buf[0]) != typeInterior {
		err = errors.New("wrong node type")
		return
	}
	if err = protobuf.Decode(buf[1:], &node); err != nil {
		return
	}
	return
}

func newEmptyNode(prefix []bool) emptyNode {
	return emptyNode{
		Prefix: prefix,
	}
}

func (n *emptyNode) hash(nonce []byte) []byte {
	h := sha256.New()
	h.Write([]byte{byte(typeEmpty)})
	h.Write(nonce)
	h.Write(toByteSlice(n.Prefix))

	lBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(lBuf, uint32(len(n.Prefix)))
	h.Write(lBuf)
	return h.Sum(nil)
}

func (n *emptyNode) encode() ([]byte, error) {
	buf, err := protobuf.Encode(n)
	if err != nil {
		return nil, err
	}
	return append([]byte{byte(typeEmpty)}, buf...), nil
}

func decodeEmptyNode(buf []byte) (node emptyNode, err error) {
	if len(buf) == 0 {
		err = errors.New("empty buffer")
		return
	}
	if nodeType(buf[0]) != typeEmpty {
		err = errors.New("wrong node type")
		return
	}
	if err = protobuf.Decode(buf[1:], &node); err != nil {
		return
	}
	return
}

func (n *leafNode) hash(nonce []byte) []byte {
	h := sha256.New()
	h.Write([]byte{byte(typeLeaf)})
	h.Write(nonce)
	h.Write(toByteSlice(n.Prefix))

	lBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(lBuf, uint32(len(n.Prefix)))
	h.Write(lBuf)

	h.Write(n.Key)
	h.Write(n.Value)
	return h.Sum(nil)
}

func newLeafNode(prefix []bool, key []byte, value []byte) leafNode {
	return leafNode{
		Prefix: prefix,
		Key:    key,
		Value:  value,
	}
}

func (n *leafNode) encode() ([]byte, error) {
	buf, err := protobuf.Encode(n)
	if err != nil {
		return nil, err
	}
	return append([]byte{byte(typeLeaf)}, buf...), nil
}

func decodeLeafNode(buf []byte) (node leafNode, err error) {
	if len(buf) == 0 {
		err = errors.New("empty buffer")
		return
	}
	if nodeType(buf[0]) != typeLeaf {
		err = errors.New("wrong node type")
		return
	}
	if err = protobuf.Decode(buf[1:], &node); err != nil {
		return
	}
	return
}
