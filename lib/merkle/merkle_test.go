package merkle_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/dedis/cothority/lib/merkle"
	"github.com/dedis/crypto/nist"
)

func TestCommitHash(t *testing.T) {
	suite := nist.NewAES128SHA256P256()
	m := merkle.NewMerkle(suite)
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
