package sign_test

import
(
	"bytes"
	"reflect"
	"testing"

	"log"

	"github.com/dedis/cothority/lib/hashid"
	"github.com/dedis/cothority/lib/proof"
	"github.com/dedis/cothority/proto/sign"
	"github.com/dedis/crypto/edwards"
)

func init() {
	log.SetFlags(log.Lshortfile)
}

func TestErrorMessage(t *testing.T) {
	sm := &sign.SigningMessage{Type: sign.Error, Err: &sign.ErrorMessage{Err: "random error"}}
	b, e := sm.MarshalBinary()
	if e != nil {
		t.Fatal(e)
	}
	sm2 := &sign.SigningMessage{}
	e = sm2.UnmarshalBinary(b)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(sm, sm2) {
		t.Fatal("sm != sm2: ", sm, sm2, sm.Am, sm2.Am)
	}
}

// test marshalling and unmarshalling for
// the various types of signing messages

func TestMUAnnouncement(t *testing.T) {
	Message := []byte("Hello World")
	sm := &sign.SigningMessage{Type: sign.Announcement, Am: &sign.AnnouncementMessage{Message: Message}}
	dataBytes, err := sm.MarshalBinary()
	if err != nil {
		t.Error("Marshaling didn't work")
	}

	sm2 := &sign.SigningMessage{}
	sm2.UnmarshalBinary(dataBytes)
	if err != nil {
		t.Error("Unmarshaling didn't work")
	}
	if !reflect.DeepEqual(sm, sm2) {
		t.Fatal("sm != sm2: ", sm, sm2, sm.Am, sm2.Am)
	}
}

// Test for Marshalling and Unmarshalling Challenge Messages
// Important: when making empty HashIds len should be set to HASH_SIZE
func TestMUChallenge(t *testing.T) {
	nHashIds := 3

	var err error
	suite := edwards.NewAES128SHA256Ed25519(false)
	//suite := nist.NewAES128SHA256P256()
	rand := suite.Cipher([]byte("example"))

	cm := &sign.ChallengeMessage{}
	cm.C = suite.Secret().Pick(rand)
	cm.MTRoot = make([]byte, hashid.Size)
	cm.Proof = proof.Proof(make([]hashid.HashId, nHashIds))
	for i := 0; i < nHashIds; i++ {
		cm.Proof[i] = make([]byte, hashid.Size)
	}
	sm := &sign.SigningMessage{Type: sign.Challenge, Chm: cm}
	smBytes, err := sm.MarshalBinary()
	if err != nil {
		t.Error(err)
	}

	messg := &sign.SigningMessage{}
	err = messg.UnmarshalBinary(smBytes)
	cm2 := messg.Chm

	// test for equality after marshal and unmarshal
	if !cm2.C.Equal(cm.C) ||
		bytes.Compare(cm2.MTRoot, cm.MTRoot) != 0 ||
		!byteArrayEqual(cm2.Proof, cm.Proof) {
		t.Error("challenge message MU failed")
	}
}

// Test for Marshalling and Unmarshalling Comit Messages
// Important: when making empty HashIds len should be set to HASH_SIZE
func TestMUCommit(t *testing.T) {
	var err error
	suite := edwards.NewAES128SHA256Ed25519(false)
	//suite := nist.NewAES128SHA256P256()
	rand := suite.Cipher([]byte("exampfsdjkhujgkjsgfjgle"))
	rand2 := suite.Cipher([]byte("examplsfhsjedgjhsge2"))

	cm := &sign.CommitmentMessage{}
	cm.V, _ = suite.Point().Pick(nil, rand)
	cm.V_hat, _ = suite.Point().Pick(nil, rand2)

	cm.MTRoot = make([]byte, hashid.Size)
	sm := sign.SigningMessage{Type: sign.Commitment, Com: cm}
	smBytes, err := sm.MarshalBinary()
	if err != nil {
		t.Error(err)
	}

	messg := &sign.SigningMessage{}
	err = messg.UnmarshalBinary(smBytes)
	cm2 := messg.Com

	// test for equality after marshal and unmarshal
	if !cm2.V.Equal(cm.V) ||
		!cm2.V_hat.Equal(cm.V_hat) ||
		bytes.Compare(cm2.MTRoot, cm.MTRoot) != 0 {
		t.Error("commit message MU failed")
	}

}

func byteArrayEqual(a proof.Proof, b proof.Proof) bool {
	n := len(a)
	if n != len(b) {
		return false
	}

	for i := 0; i < n; i++ {
		if bytes.Compare(a[i], b[i]) != 0 {
			return false
		}
	}

	return true
}
