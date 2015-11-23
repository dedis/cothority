package coconet

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/protobuf"
)

type NetworkMessg struct {
	Data BinaryUnmarshaler // data
	From string            // name of server data came from
	Err  error
}

func (nm *NetworkMessg) MarshalBinary() ([]byte, error) {
	return protobuf.Encode(nm)
}

func (nm *NetworkMessg) UnmarshalBinary(data []byte) error {
	dbg.Print("UnmarshalBinary : ", len(data), " bytes")
	return protobuf.Decode(data, nm)
}
