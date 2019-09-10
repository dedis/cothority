package personhood

import (
	"crypto/sha256"
	"encoding/binary"
	"testing"

	"go.dedis.ch/cothority/v3"

	"go.dedis.ch/cothority/v3/byzcoin/contracts"

	"go.dedis.ch/protobuf"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/calypso"
	"go.dedis.ch/cothority/v3/darc"
)

// This test-file has some more elaborate structures to handle the RoPaSci
// instances. The following structures are used:
//   * sStruct - basic test-structure for the personhood-tests - holds a ByzCoin
//   * testRPS - more specific RoPaSci structure holding two coins and the LTS
//   * rps - holds one RoPaSci game including the necessary variables for easy testing
//   * sCoin - handles a coin, complete with the name, value, and instanceID

// TestContractRoPaSci does a classical Rock-Paper-Scissors game and checks that all
// combinations give the correct payout.
func TestContractPoPaSci(t *testing.T) {
	s := newS(t)
	defer s.Close()

	sr := s.newTestRPS()

	for move1 := 0; move1 <= 2; move1++ {
		for move2 := 0; move2 <= 2; move2++ {
			switch (3 + move1 - move2) % 3 {
			case 0:
				sr.coin1.coin.Value -= 100
				sr.coin2.coin.Value -= 100
			case 1:
				sr.coin1.coin.Value += 100
				sr.coin2.coin.Value -= 100
			case 2:
				sr.coin1.coin.Value -= 100
				sr.coin2.coin.Value += 100
			}
			rps := sr.newRPS(move1, sr.coin1.id, 100)
			rps.second(move2, sr.coin2.id)
			rps.confirm(rps.preHash)
			coin1New := s.coinGet(t, sr.coin1.id)
			coin2New := s.coinGet(t, sr.coin2.id)
			require.Equal(t, sr.coin1.coin.Value, coin1New.Value)
			require.Equal(t, sr.coin2.coin.Value, coin2New.Value)
		}
	}
}

// TestContractRoPaSciCalypso uses the calypso-Rock-Paper-Scissors that stores the
// prehash of player 1 in a CalypsoWrite, so that player 2 doesn't have to wait for
// player 1 to reveal.
func TestContractPoPaSciCalypso(t *testing.T) {
	s := newS(t)
	defer s.Close()

	sr := s.newTestRPS()

	for move1 := 0; move1 <= 2; move1++ {
		for move2 := 0; move2 <= 2; move2++ {
			switch (3 + move1 - move2) % 3 {
			case 0:
				sr.coin1.coin.Value -= 100
				sr.coin2.coin.Value -= 100
			case 1:
				sr.coin1.coin.Value += 100
				sr.coin2.coin.Value -= 100
			case 2:
				sr.coin1.coin.Value -= 100
				sr.coin2.coin.Value += 100
			}
			rps := sr.newCalypsoRPS(move1, sr.coin1.id, 100)
			ret := rps.calypsoSecond(move2, sr.coin2.id)
			wrProof, err := s.cl.GetProof(ret.CalypsoWrite.Slice())
			require.NoError(t, err)
			rdProof, err := s.cl.GetProof(ret.CalypsoRead.Slice())
			require.NoError(t, err)
			dkr, err := sr.ca.DecryptKey(&calypso.DecryptKey{
				Read:  rdProof.Proof,
				Write: wrProof.Proof,
			})
			require.NoError(t, err)
			preHash, err := dkr.RecoverKey(rps.keypair.Ed25519.Secret)
			require.NoError(t, err)
			rps.confirm(append(preHash, []byte{0, 0, 0, 0}...))
			coin1New := s.coinGet(t, sr.coin1.id)
			coin2New := s.coinGet(t, sr.coin2.id)
			require.Equal(t, sr.coin1.coin.Value, coin1New.Value)
			require.Equal(t, sr.coin2.coin.Value, coin2New.Value)
		}
	}
}

//
// ** Helpers for the tests
//

// newTestRPS creates a new test-structure for RoPaSci games, including an LTS.
func (s *sStruct) newTestRPS() (sr testRPS) {
	sr.sStruct = s
	sr.coin1 = s.newCoin("RoPaSci", 1e6)
	sr.coin2 = s.newCoin("RoPaSci", 1e6)
	sr.ca = calypso.NewClient(s.cl)
	// Authorize bc for inclusion in nodes
	for _, si := range s.roster.List {
		require.NoError(s.t, sr.ca.Authorize(si, s.cl.ID))
	}

	s.counter++
	var err error
	sr.lts, err = sr.ca.CreateLTS(s.roster, s.genesisDarc.GetBaseID(), []darc.Signer{s.signer},
		[]uint64{s.counter})
	require.NoError(s.t, err)
	return
}

type testRPS struct {
	*sStruct
	coin1 sCoin
	coin2 sCoin
	lts   *calypso.CreateLTSReply
	ca    *calypso.Client
}

// newRPS creates a rock-paper-scissors game.
func (sr *testRPS) newRPS(move int, coinID byzcoin.InstanceID, stake uint64) (r rps) {
	// Create RoPaSci
	r.preHash = make([]byte, 32)
	r.preHash[0] = byte(move)
	moveHash := sha256.Sum256(r.preHash)
	ropasci := RoPaSciStruct{
		Description:     "test",
		FirstPlayerHash: moveHash[:],
	}
	ropasciBuf, err := protobuf.Encode(&ropasci)
	require.NoError(sr.t, err)

	// Create RoPaSci contract
	coinsBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(coinsBuf, 100)
	tx, _ := sr.addTx(
		byzcoin.Instruction{
			InstanceID: coinID,
			Invoke: &byzcoin.Invoke{
				ContractID: contracts.ContractCoinID,
				Command:    "fetch",
				Args:       byzcoin.Arguments{{Name: "coins", Value: coinsBuf}},
			},
		},
		byzcoin.Instruction{
			InstanceID: byzcoin.NewInstanceID(sr.genesisDarc.GetBaseID()),
			Spawn: &byzcoin.Spawn{
				ContractID: ContractRoPaSciID,
				Args:       byzcoin.Arguments{{Name: "struct", Value: ropasciBuf}},
			},
		},
	)
	r.id = tx.Instructions[1].DeriveID("")
	r.stake = stake
	r.testRPS = sr
	r.account1 = coinID
	return
}

// newCalypsoRPS creates a rock-paper-scissors game with the pre-hash stored in a
// calypsoWrite.
func (sr *testRPS) newCalypsoRPS(move int, coinID byzcoin.InstanceID, stake uint64) (r rps) {
	// Create RoPaSci
	r.preHash = make([]byte, 32)
	r.preHash[0] = byte(move)
	moveHash := sha256.Sum256(r.preHash)
	writeCommit := sha256.Sum256(moveHash[:])
	ropasci := RoPaSciStruct{
		Description:        "test",
		FirstPlayerHash:    moveHash[:],
		FirstPlayerAccount: sr.coin1.id,
	}
	ropasciBuf, err := protobuf.Encode(&ropasci)
	require.NoError(sr.t, err)

	wr := calypso.NewWrite(cothority.Suite, sr.lts.InstanceID, darc.ID(writeCommit[:]), sr.lts.X,
		r.preHash[0:28])
	wrBuf, err := protobuf.Encode(wr)
	require.NoError(sr.t, err)

	// Create RoPaSci contract
	coinsBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(coinsBuf, 100)
	tx, _ := sr.addTx(
		byzcoin.Instruction{
			InstanceID: coinID,
			Invoke: &byzcoin.Invoke{
				ContractID: contracts.ContractCoinID,
				Command:    "fetch",
				Args:       byzcoin.Arguments{{Name: "coins", Value: coinsBuf}},
			},
		},
		byzcoin.Instruction{
			InstanceID: byzcoin.NewInstanceID(sr.genesisDarc.GetBaseID()),
			Spawn: &byzcoin.Spawn{
				ContractID: ContractRoPaSciID,
				Args: byzcoin.Arguments{
					{Name: "struct", Value: ropasciBuf},
					{Name: "secret", Value: wrBuf},
				},
			},
		},
	)
	r.id = tx.Instructions[1].DeriveID("")
	r.stake = stake
	r.testRPS = sr
	r.account1 = coinID
	return
}

type rps struct {
	*testRPS
	id       byzcoin.InstanceID
	account1 byzcoin.InstanceID
	preHash  []byte
	stake    uint64
	keypair  darc.Signer
}

// second is the move of the second player. This is for non-calypso RoPaScis.
func (r *rps) second(move int, coinID byzcoin.InstanceID) {
	coinsBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(coinsBuf, r.stake)
	r.addTx(
		byzcoin.Instruction{
			InstanceID: coinID,
			Invoke: &byzcoin.Invoke{ContractID: contracts.ContractCoinID,
				Command: "fetch",
				Args: byzcoin.Arguments{
					{Name: "coins", Value: coinsBuf},
				},
			},
		},
		byzcoin.Instruction{
			InstanceID: r.id,
			Invoke: &byzcoin.Invoke{ContractID: ContractRoPaSciID,
				Command: "second",
				Args: byzcoin.Arguments{
					{Name: "account", Value: coinID.Slice()},
					{Name: "choice", Value: []byte{byte(move)}},
				},
			},
		},
	)
}

// confirm is either the 1st or the 2nd player, depending on whether it is a
// calypso-enabled game, and the 2nd player can confirm, too.
func (r *rps) confirm(preHash []byte) {
	r.invoke(r.id, ContractRoPaSciID, "confirm", byzcoin.Arguments{
		{Name: "prehash", Value: preHash},
		{Name: "account", Value: r.account1.Slice()},
	})
}

// calypsoSecond is for a calypso RoPaSci, where the 2nd player gets a CalypsoRead instance
// with which he can reveal/confirm the game.
func (r *rps) calypsoSecond(move int, coinID byzcoin.InstanceID) (rpsRet RoPaSciStruct) {
	coinsBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(coinsBuf, r.stake)
	r.keypair = darc.NewSignerEd25519(nil, nil)
	pubBuf, err := r.keypair.Ed25519.Point.MarshalBinary()
	require.NoError(r.t, err)
	r.addTx(
		byzcoin.Instruction{
			InstanceID: coinID,
			Invoke: &byzcoin.Invoke{ContractID: contracts.ContractCoinID,
				Command: "fetch",
				Args: byzcoin.Arguments{
					{Name: "coins", Value: coinsBuf},
				},
			},
		},
		byzcoin.Instruction{
			InstanceID: r.id,
			Invoke: &byzcoin.Invoke{ContractID: ContractRoPaSciID,
				Command: "second",
				Args: byzcoin.Arguments{
					{Name: "account", Value: coinID.Slice()},
					{Name: "choice", Value: []byte{byte(move)}},
					{Name: "public", Value: pubBuf},
				},
			},
		},
	)
	pr, err := r.cl.GetProof(r.id.Slice())
	require.NoError(r.t, err)
	_, val, cid, _, err := pr.Proof.KeyValue()
	require.NoError(r.t, err)
	require.Equal(r.t, ContractRoPaSciID, cid)
	require.NoError(r.t, protobuf.Decode(val, &rpsRet))
	return
}
