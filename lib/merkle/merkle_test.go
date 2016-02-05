package merkle_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/dedis/cothority/lib/merkle"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/nist"
)

func TestCommitHash(t *testing.T) {
	suite := nist.NewAES128SHA256P256()
	m := merkle.NewMerkle(suite.Hash())
	V1 := suite.Point().Base()
	h1, err := m.HashCommits(V1)
	if err != nil {
		t.Fatalf("Couldn't compute hash of V1=%s", V1)
	}
	V2, _ := suite.Point().Pick([]byte("another point"), suite.Cipher([]byte("test key")))
	h2, err := m.HashCommits(V2)
	if err != nil {
		t.Fatalf("Couldn't compute hash of V2=%s", V2)
	}
	h3, err := m.HashCommits(V1, V2, h1, h2)
	if err != nil {
		t.Fatal("Couldn't compute hash of V1, V2, h1, h2")
	}
	if bytes.Equal(h1, h2) || bytes.Equal(h1, h3) || bytes.Equal(h2, h3) {
		t.Fatal("Collision")
	}
	fmt.Printf("FYI:\nh1=%v\nh2=%v\nh3=%v\n", h1, h2, h3)
}

// see if we can (un)marshal a point and its using the network lib
type TestMsg struct {
	Point abstract.Point
	H     merkle.Hash
}

var TestType = network.RegisterMessageType(TestMsg{})

func TestNetworkMarshaling(t *testing.T) {
	suite := nist.NewAES128SHA256P256()
	m := merkle.NewMerkle(suite.Hash())
	V1 := suite.Point().Base()
	h1, err := m.HashCommits(V1)
	if err != nil {
		t.Fatalf("Couldn't compute hash of V1=%s", V1)
	}

	tm := TestMsg{V1, h1}
	b, err := network.MarshalRegisteredType(&tm)
	if err != nil {
		t.Fatalf("Couldn't marhsal=%+v", tm)
	}
	cons := network.DefaultConstructors(suite)
	_, msg, err := network.UnmarshalRegisteredType(b, cons)
	if err != nil {
		t.Fatalf("Couldn't unmarhsal %+v from %s", tm, b)
	}
	msgStruct := msg.(TestMsg)
	if !bytes.Equal(msgStruct.H, tm.H) {
		t.Fatalf("Umarshaling didn't work")
	}
}
