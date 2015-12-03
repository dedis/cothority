package conode_test

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"github.com/dedis/cothority/lib/app"
	"github.com/dedis/cothority/lib/conode"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/hashid"
	"github.com/dedis/crypto/abstract"
	"io"
	"testing"
)

var reply conode.StampSignature
var X0 abstract.Point
var suite abstract.Suite
var hash []byte

func init() {
	dbg.DebugVisible = 1
}

// Verifies whether the Challenge is correct
func TestVerifyChallenge(t *testing.T) {
	setupTestSig()

	err := conode.VerifyChallenge(suite, &reply)
	if err != nil {
		t.Error("Verification failed")
	} else {
		dbg.Lvl2("Verification passed")
	}
}

// Verifies whether the X0 and hash is correct
func TestVerifySignature(t *testing.T) {
	setupTestSig()

	if !conode.VerifySignature(suite, &reply, X0, hash) {
		t.Error("Verification failed")
	} else {
		dbg.Lvl2("Verification passed")
	}
}

// Verifies whether the Schnorr signature is correct
func TestVerifySchnorr(t *testing.T) {
	setupTestSig()
	var b bytes.Buffer
	if err := binary.Write(&b, binary.LittleEndian, reply.Timestamp); err != nil {
		dbg.Lvl1("Error marshaling the timestamp for signature verification")
	}
	msg := append(b.Bytes(), []byte(reply.MerkleRoot)...)
	err := conode.VerifySchnorr(suite, msg, X0, reply.Challenge, reply.Response)
	if err != nil {
		dbg.Fatal("Schnorr verification failed")
	} else {
		dbg.Lvl2("Schnorr OK")
	}
}

// Checks the correct setup of the signature
func TestsetupTestSig(t *testing.T) {
	setupTestSig()
	if !reply.AggPublic.Equal(X0) {
		t.Error("X0 is not equal")
	} else {
		dbg.Lvl2("X0 is OK")
	}
}

type test_sig struct {
	Suite      string
	AggPubKey  string
	Name       string
	Timestamp  int
	Hash       string
	Root       string
	Proof      []string
	Challenge  string
	Response   string
	Commitment string
}

func setupTestSig() {
	var sig = test_sig{
		Suite:      "Ed25519",
		Name:       "stamp.sig",
		Timestamp:  1448637057,
		Proof:      []string{"fN1GPbpXUqLGh20Ls1JmiFncbWcnvai4pt2ufJnUcIo=", "ehvna7oGGqwZsCgLVP1GvEHxCbYl2Bv8fS0EgGEvmB4=", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=", "cFol1fkwjOXyXLNM8Bhu3Bafs1L8GnrWWswE19YDh/E=", "cFol1fkwjOXyXLNM8Bhu3Bafs1L8GnrWWswE19YDh/E="},
		Root:       "vu2wvZDMc5ZlCNspMRWKZCI0ShYZ8zyLfy2cuZFf54A=",
		Challenge:  "ClSG6pa3uklYndfBztJ9yAD8aY2g/GzTq0rs8nQC12w=",
		Response:   "AJXkEEiyvWdoidjjUMUi5nryHRzJSywXoFY/fIT/8FM=",
		Commitment: "LbMi53pBXpHN1IYa7pRaC953KAox+NvzQSzAOsK1CEQ=",
		AggPubKey:  "dkp32QL4viiR0EiMtnLIVYLgk6PXTcSQlNXFNwpnLiI=",
		Hash:       "RnZyRnItjXQBSMYLfY/f8WDgiYJI9Yh4lQXa6+VwWxc=",
	}

	suite = app.GetSuite(sig.Suite)
	suite.Read(get64R(sig.AggPubKey), &X0)

	reply.SuiteStr = sig.Suite
	reply.Timestamp = int64(sig.Timestamp)
	reply.MerkleRoot = get64(sig.Root)
	var proof []hashid.HashId
	for _, p := range sig.Proof {
		proof = append(proof, get64(p))
	}
	reply.Prf = proof

	suite.Read(get64R(sig.Challenge), &reply.Challenge)
	suite.Read(get64R(sig.Response), &reply.Response)
	suite.Read(get64R(sig.Commitment), &reply.AggCommit)
	suite.Read(get64R(sig.AggPubKey), &reply.AggPublic)

	hash = get64(sig.Hash)

	dbg.Lvl3("Challenge", reply.Challenge)
	dbg.Lvl3("Response", reply.Response)
	dbg.Lvl3("Commitment", reply.AggCommit)
	dbg.Lvl3("AggPubKey", reply.AggPublic)
}

func get64R(str string) io.Reader {
	return bytes.NewReader(get64(str))
}

func get64(str string) []byte {
	ret, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		dbg.Fatal("Couldn't decode", str)
	}
	return ret
}
