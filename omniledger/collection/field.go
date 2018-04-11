package collection

import "errors"
import "encoding/binary"

// Enums

type Navigation bool

const (
	Left  Navigation = false
	Right Navigation = true
)

// Interfaces

type Field interface {
	Encode(interface{}) []byte
	Decode([]byte) (interface{}, error)
	Placeholder() []byte
	Parent([]byte, []byte) ([]byte, error)
	Navigate([]byte, []byte, []byte, []byte) (Navigation, error)
}

// Structs

// Data

type Data struct {
}

// Interface

func (this Data) Encode(generic interface{}) []byte {
	value := generic.([]byte)
	return value
}

func (this Data) Decode(raw []byte) (interface{}, error) {
	return raw, nil
}

func (this Data) Placeholder() []byte {
	return []byte{}
}

func (this Data) Parent(left []byte, right []byte) ([]byte, error) {
	return []byte{}, nil
}

func (this Data) Navigate(query []byte, parent []byte, left []byte, right []byte) (Navigation, error) {
	return false, errors.New("Data values cannot be navigated.")
}

// Stake64

type Stake64 struct {
}

// Interface

func (this Stake64) Encode(generic interface{}) []byte {
	value := generic.(uint64)
	raw := make([]byte, 8)

	binary.BigEndian.PutUint64(raw, value)
	return raw
}

func (this Stake64) Decode(raw []byte) (interface{}, error) {
	if len(raw) != 8 {
		return 0, errors.New("Wrong buffer length.")
	} else {
		return binary.BigEndian.Uint64(raw), nil
	}
}

func (this Stake64) Placeholder() []byte {
	return this.Encode(uint64(0))
}

func (this Stake64) Parent(left []byte, right []byte) ([]byte, error) {
	leftvalue, lefterror := this.Decode(left)

	if lefterror != nil {
		return []byte{}, lefterror
	}

	rightvalue, righterror := this.Decode(right)

	if righterror != nil {
		return []byte{}, righterror
	}

	return this.Encode(leftvalue.(uint64) + rightvalue.(uint64)), nil
}

func (this Stake64) Navigate(query []byte, parent []byte, left []byte, right []byte) (Navigation, error) {
	queryvalue, queryerror := this.Decode(query)

	if queryerror != nil {
		return false, queryerror
	}

	parentvalue, parenterror := this.Decode(parent)

	if parenterror != nil {
		return false, parenterror
	}

	if queryvalue.(uint64) >= parentvalue.(uint64) {
		return false, errors.New("Query exceeds parent stake.")
	}

	leftvalue, lefterror := this.Decode(left)

	if lefterror != nil {
		return false, lefterror
	}

	if queryvalue.(uint64) >= leftvalue.(uint64) {
		copy(query, this.Encode(queryvalue.(uint64)-leftvalue.(uint64)))
		return Right, nil
	} else {
		return Left, nil
	}
}
