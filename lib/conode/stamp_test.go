package conode_test

import (
	"encoding/json"

	"github.com/dedis/cothority/lib/conode"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/hashid"
	"github.com/dedis/cothority/lib/proof"
	//"github.com/dedis/cothority/lib/sign"
	"strconv"
	"testing"
	"time"

	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/edwards"
)

// Runs two conodes and tests if the value returned is OK
func TestStamp(t *testing.T) {
	dbg.TestOutput(testing.Verbose(), 4)
	peer1, peer2 := createPeers()
	go peer1.LoopRounds(conode.RoundStamperListenerType, 4)
	go peer2.LoopRounds(conode.RoundStamperListenerType, 4)
	time.Sleep(2 * time.Second)

	s, err := conode.NewStamp("testdata/config.toml")
	if err != nil {
		t.Fatal("Couldn't open config-file:", err)
	}

	for _, port := range []int{7000, 7010} {
		stamper := "localhost:" + strconv.Itoa(port)
		dbg.Lvl2("Contacting stamper", stamper)
		tsm, err := s.GetStamp([]byte("test"), stamper)
		dbg.Lvl3("Evaluating results of", stamper)
		if err != nil {
			t.Fatal("Couldn't get stamp from server:", err)
		}

		if !tsm.Srep.AggPublic.Equal(s.X0) {
			t.Fatal("Not correct aggregate public key")
		}
	}

	dbg.Lvl2("Closing peer1")
	peer1.Close()
	dbg.Lvl2("Closing peer2")
	peer2.Close()
	dbg.Lvl3("Done with test")
}

// Runs two peers and looks for the exception-message in the stamp
func TestStampWithExceptionRaised(t *testing.T) {
	// dbg.TestOutput(testing.Verbose(), 4)
	//
	// peer1, peer2 := createPeers()
	// // 2nd node should fail:
	// sign.ExceptionForceFailure = peer2.Name()
	//
	// // root node:
	// dbg.Lvl2(peer1.Name(), "Stamp server in round", 1, "of", 2)
	// round, err := sign.NewRoundFromType(conode.RoundStamperListenerType, peer1.Node)
	// if err != nil {
	// 	dbg.Fatal("Couldn't create", conode.RoundStamperListenerType, err)
	// }
	// err = peer1.StartAnnouncement(round)
	// if err != nil {
	// 	dbg.Lvl3(err)
	// }

	// stampClient, err := conode.NewStamp("testdata/config.toml")
	// if err != nil {
	// 	t.Fatal("Couldn't open config-file:", err)
	// }

	// for _, port := range []int{7000, 7010} { // constants from config.toml
	// 	stamper := "localhost:" + strconv.Itoa(port)
	// 	dbg.Lvl2("Contacting stamper", stamper)
	// 	wait := make(chan bool)
	// 	go func() {
	// 		tsm, err := stampClient.GetStamp([]byte("test"), stamper)
	// 		dbg.Lvl3("Evaluating results of", stamper)
	// 		wait <- true
	// 		if err != nil {
	// 			t.Fatal("Couldn't get stamp from server:", err)
	// 		}
	// 		if !tsm.Srep.AggPublic.Equal(stampClient.X0) {
	// 			t.Fatal("Not correct aggregate public key")
	// 		}
	// 	}()
	//
	// 	dbg.Lvl2(peer1.Name(), "Stamp server in round", 2, "of", 2)
	// 	round, err = sign.NewRoundFromType(conode.RoundStamperListenerType, peer1.Node)
	// 	if err != nil {
	// 		dbg.Fatal("Couldn't create", conode.RoundStamperListenerType, err)
	// 	}
	// 	err = peer1.StartAnnouncement(round)
	// 	if err != nil {
	// 		dbg.Lvl3(err)
	// 	}
	// 	_ = <-wait
	// }
	// peer1.SendCloseAll()

	// dbg.Lvl2("Closing peer1")
	// peer1.Close()
	// dbg.Lvl2("Closing peer2")
	// peer2.Close()
	// dbg.Lvl3("Done with test")
}

// test JSON decoding and encoding (test (un)marshaling only)
func TestStampSignatureJSON(t *testing.T) {
	suite := edwards.NewAES128SHA256Ed25519(false)
	hid := make([]hashid.HashId, 0)
	hid = append(hid, hashid.HashId([]byte{0x01}))
	ss := &conode.StampSignature{
		SuiteStr:            suite.String(),
		Timestamp:           time.Now().Unix(),         // The timestamp requested for the file
		MerkleRoot:          make([]byte, 0),           // root of the merkle tree
		Prf:                 proof.Proof(hid),          // Merkle proof for the value sent to be stamped
		Response:            suite.Secret().Zero(),     // Aggregate response
		Challenge:           suite.Secret().Zero(),     // Aggregate challenge
		AggCommit:           suite.Point().Base(),      // Aggregate commitment key
		AggPublic:           suite.Point().Base(),      // Aggregate public key (use for easy troubleshooting)
		ExceptionPublicList: make([]abstract.Point, 0), // challenge from root
	}
	b, err := json.Marshal(ss)
	if err != nil {
		dbg.Fatal("Could not marshal StampSignature")
	}
	ss.ExceptionPublicList = append(ss.ExceptionPublicList, suite.Point().Base())
	b, err = json.Marshal(ss)
	if err != nil {
		dbg.Fatal("Could not marshal StampSignature")
	}

	ssUn := conode.StampSignature{SuiteStr: suite.String()}

	if err = json.Unmarshal(b, &ssUn); err != nil {
		dbg.Fatal("Coudl not unmarshal")
	}
}
