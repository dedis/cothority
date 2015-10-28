package conode_test

import (
	"github.com/dedis/cothority/lib/conode"
	"testing"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/cothority/lib/app"
	"encoding/base64"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"bytes"
	"io"
	"github.com/dedis/cothority/lib/hashid"
	"encoding/binary"
)

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

var test_sig_1 = test_sig{
	"25519",
	"wuFmm+eMZX/6x8cYOCvIDgecdaQBMWuvBMbhvwqLbkE=",
	"stamp",
	1446032576,
	"Qn5aBmYu3RBxJHfGe68l3WP0vz4ysWHYNYpZEacFean4sFsFHsHtmQ1GRUfDBp90/JWicH4qCcUtkO1fhqx+9Q==",
	"NUGJljmFLn+/4+2V8tNTlHOrBFQOLkjg5xg/HGkke2c=",
	[]string {"shAx/3sE0uUuIZXVrZmNxlgKSzbgT4YX/HdW2fXX4/Y=", "5jylR6gm5C4jDNyyHx6/qXC4swf9X/4WKlD8iVcKTeI=", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=", "mJ3wmCIaQNZq3gZiXI0caWzAi8RpjLvzn6T6vMg9GAA=", "XvOCOzocSj3zhqgj06pmf1cda9QIwRLNn3+SB0EusLA=", "mJ3wmCIaQNZq3gZiXI0caWzAi8RpjLvzn6T6vMg9GAA=", "+QVbJpGl2PwmfVJccLa+FPT3WBrH0bw3PCpEvSDdoOU=", "+QVbJpGl2PwmfVJccLa+FPT3WBrH0bw3PCpEvSDdoOU="},
	"PQOr+JwoBg6040oed86yoKsLrn/XRU8REtePY+8L7Oc=",
	"TFnyRSAKJ595MUcR8UV8y/yKGjvHz4yHcIWaNtBpTrg=",
	"ixyyZ3kryOm4TLJU29wUzB1tEP0v3EkXP1W7bAGf/4E=",
}

var reply conode.StampReply
var X0 abstract.Point
var suite abstract.Suite

func init(){
	dbg.DebugVisible = 5
}

func TestVerifySignature(t *testing.T){
	readTestSig(test_sig_1)

	if conode.VerifySignature(suite, &reply, X0, reply.MerkleRoot){
		dbg.Lvl2("Verification passed")
	} else {
		dbg.Fatal("Verification failed")
	}
}

func TestVerifySchnorr(t *testing.T) {
	readTestSig(test_sig_1)
	var b bytes.Buffer
	if err := binary.Write(&b, binary.LittleEndian, reply.Timestamp); err != nil {
		dbg.Lvl1("Error marshaling the timestamp for signature verification")
	}
	msg := append(b.Bytes(), []byte(reply.MerkleRoot)...)
	err := conode.VerifySchnorr(suite, msg, X0, reply.SigBroad.C, reply.SigBroad.R0_hat);
	if err != nil{
		dbg.Fatal("Schnorr verification failed")
	} else {
		dbg.Lvl2("Schnorr OK")
	}
}

func TestReadTestSig(t *testing.T){
	readTestSig(test_sig_1)
	if reply.SigBroad.X0_hat.Equal(X0){
		dbg.Lvl2("X0 is OK")
	} else {
		dbg.Fatal("X0 is not equal")
	}
}

func readTestSig(sig test_sig) {
	suite = app.GetSuite(sig.Suite)
	suite.Read(get64R(sig.AggPubKey), &X0)

	reply.SuiteStr = sig.Suite
	reply.Timestamp = int64(sig.Timestamp)
	reply.MerkleRoot = get64(sig.Hash)
	reply.PrfLen = 0
	var proof []hashid.HashId
	for _, p := range(sig.Proof){
		proof = append(proof, get64(p))
	}
	reply.Prf = proof

	suite.Read(get64R(sig.Challenge), &reply.SigBroad.C)
	suite.Read(get64R(sig.Response), &reply.SigBroad.R0_hat)
	suite.Read(get64R(sig.Commitment), &reply.SigBroad.V0_hat)
	suite.Read(get64R(sig.AggPubKey), &reply.SigBroad.X0_hat)

	dbg.Lvl3("Challenge", reply.SigBroad.C)
	dbg.Lvl3("Response", reply.SigBroad.R0_hat)
	dbg.Lvl3("Commitment", reply.SigBroad.V0_hat)
	dbg.Lvl3("AggPubKey", reply.SigBroad.X0_hat)
}

func get64R(str string) io.Reader{
	return bytes.NewReader(get64(str))
}

func get64(str string) []byte{
	ret, err := base64.StdEncoding.DecodeString(str)
	if err != nil{
		dbg.Fatal("Couldn't decode", str)
	}
	return ret
}