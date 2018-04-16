package collection

import (
	"encoding/binary"
	"errors"
)

// Enums

const (
	// Left is a constant representing the case where the Navigator should go to the left child.
	Left = false
	// Right is a constant representing the case where the Navigator should go to the right child.
	Right = true
)

// Field describes a type.
// It describes how the type is encoded, how it is decoded,
// how it propagates up the tree and other needed functionality.
type Field interface {
	Encode(generic interface{}) []byte      // Describes how the type is encoded into bytes
	Decode(raw []byte) (interface{}, error) // Inverse of Encode. Describes how to decode the byte to the original type
	Placeholder() []byte                    // returns the byte representation of a placeholder (no value)

	// returns the representation in byte of a parent value given its two children values.
	// Used to store data on non-leaf node.
	// Can return an empty array if this operation is not supported.
	Parent(left []byte, right []byte) ([]byte, error)

	// Navigate returns a navigation boolean indicating the direction a Navigator should go to
	// from a parent node with a given query.
	Navigate(query []byte, parent []byte, left []byte, right []byte) (bool, error)
}

// Structures

// Data

// Data is the generic type of Field.
// It can contain any interface convertible to bytes, for example string or []uint8.
type Data struct {
}

// Encode returns the basic conversion into a byte array of the parameter.
func (d Data) Encode(generic interface{}) []byte {
	value := generic.([]byte)
	return value
}

// Decode returns the bytes without modifying them.
// It can never returns an error
func (d Data) Decode(raw []byte) (interface{}, error) {
	return raw, nil
}

// Placeholder defines the placeholder value as an empty array of byte.
func (d Data) Placeholder() []byte {
	return []byte{}
}

// Parent returns an empty array of byte.
// Indeed with this generic type, the tree doesn't need a propagation up the tree.
func (d Data) Parent(left []byte, right []byte) ([]byte, error) {
	return []byte{}, nil
}

// Navigate will return an error as the Data values cannot be navigated
func (d Data) Navigate(query []byte, parent []byte, left []byte, right []byte) (bool, error) {
	return false, errors.New("data values cannot be navigated")
}

// Stake64 represents stakes.
// Each stake is stored in a final leaf and the intermediary nodes contain the sum of its children.
// This structure allows an easy random selection of a stake from the root proportionally to the stake size.
type Stake64 struct {
}

// Encode returns array of bytes representation of the the uint64 value of the stake, in big endian.
func (s Stake64) Encode(generic interface{}) []byte {
	value := generic.(uint64)
	raw := make([]byte, 8)

	binary.BigEndian.PutUint64(raw, value)
	return raw
}

// Decode is the inverse of Encode. It returns the uint64 value of the encoded bytes.
// It may return an error if the parameter raw has a number of bytes different from four.
func (s Stake64) Decode(raw []byte) (interface{}, error) {
	if len(raw) != 8 {
		return 0, errors.New("wrong buffer length")
	}
	return binary.BigEndian.Uint64(raw), nil
}

// Placeholder returns the placeholder value for stakes.
// It is represented by the number 0 encoded into bytes, which represents an empty stake.
func (s Stake64) Placeholder() []byte {
	return s.Encode(uint64(0))
}

// Parent returns the value each non-leaf node should hold.
// The returned value is the sum of stakes of its children or an error if a decoding error occurred.
func (s Stake64) Parent(left []byte, right []byte) ([]byte, error) {
	leftValue, leftError := s.Decode(left)

	if leftError != nil {
		return []byte{}, leftError
	}

	rightValue, rightError := s.Decode(right)

	if rightError != nil {
		return []byte{}, rightError
	}

	return s.Encode(leftValue.(uint64) + rightValue.(uint64)), nil
}

// Navigate returns a navigation boolean indicating the direction a Navigator should go to with a given query.
// The first parameter is the query, representing a unsigned integer between 0 and the value of the parent parameter.
// the function will see if the query value is greater or equal than the left value and return a left navigation boolean if so
// and a right navigation boolean otherwise.
// This behavior allows to find a stake randomly and proportionally to the stake value of each leaf.
// To do this, one must select a random stake between 0 and the value of the root and input it repeatedly to the function to navigate down the tree.
func (s Stake64) Navigate(query []byte, parent []byte, left []byte, right []byte) (bool, error) {
	queryValue, queryError := s.Decode(query)

	if queryError != nil {
		return false, queryError
	}

	parentValue, parentError := s.Decode(parent)

	if parentError != nil {
		return false, parentError
	}

	if queryValue.(uint64) >= parentValue.(uint64) {
		return false, errors.New("query exceeds parent stake")
	}

	leftValue, leftError := s.Decode(left)

	if leftError != nil {
		return false, leftError
	}

	if queryValue.(uint64) >= leftValue.(uint64) {
		copy(query, s.Encode(queryValue.(uint64)-leftValue.(uint64)))
		return Right, nil
	}
	return Left, nil
}
