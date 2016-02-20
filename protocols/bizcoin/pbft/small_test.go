package pbft

import (
	"fmt"
	"testing"
	"time"

	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
)

var FinishTestMsg = network.RegisterMessageType(Finish{})

// func TestDoneEncDec(t *testing.T) {
// 	fin := Finish{"Done: true"}
// 	b, err := protobuf.Encode(&fin)
// 	if err != nil {
// 		t.Fatal("Enc failed", err)
// 	}
// 	fmt.Println("FYI encoded", hex.Dump(b))
// 	finDec := &Finish{}
// 	err = protobuf.DecodeWithConstructors(b, finDec, nil)
// 	if err != nil {
// 		t.Fatal("Decoding failed", err)
// 	}
// 	fmt.Println(finDec)
// }

func TestDoneEncDecWithSDA(t *testing.T) {
	h1, h2 := SetupTwoHosts(t, false)
	fin := &Finish{"Done: true"}
	fmt.Println("Now we are really sending sth")
	err := h1.SendRaw(h2.Entity, fin)
	if err != nil {
		t.Fatal("Couldn't send from h2 -> h1:", err)
	}
	msg := h2.Receive()
	if msg.MsgType != FinishTestMsg {
		t.Fatal("Wrong type")
	}
	msgDec := msg.Msg.(Finish)
	if msgDec.Done != "Done: true" {
		t.Fatal("Received message from h2 -> h1 is wrong")
	}
	time.Sleep(2 * time.Second)
	h1.Close()
	h2.Close()
}

func SetupTwoHosts(t *testing.T, h2process bool) (*sda.Host, *sda.Host) {
	hosts := sda.GenLocalHosts(2, true, false)
	if h2process {
		go hosts[1].ProcessMessages()
	}
	return hosts[0], hosts[1]
}
