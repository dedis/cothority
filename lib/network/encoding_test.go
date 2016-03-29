package network

import "testing"

func TestSliceInt(t *testing.T) {
	type SliceInt struct {
		Indexes []int
	}
	a := SliceInt{
		Indexes: make([]int, 100),
	}

	mt := RegisterMessageType(SliceInt{})
	b, err := MarshalRegisteredType(a)
	if err != nil {
		t.Fatal("Couldn't marshal SliceInt:", err)
	}

	mt_rcv, msg, err := UnmarshalRegisteredType(b, DefaultConstructors(Suite))
	if err != nil {
		t.Fatal("Couldn't unmarshal:", err)
	}
	if mt_rcv != mt {
		t.Fatal("Received message is not SliceInt:", err)
	}
	if len(msg.(SliceInt).Indexes) != 100 {
		t.Fatal("Length of slice int is not 100")
	}
}
