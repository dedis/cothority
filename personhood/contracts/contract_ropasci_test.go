package contracts

import (
	"crypto/sha256"
	"testing"

	"go.dedis.ch/kyber/v3/util/key"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/calypso"

	"go.dedis.ch/kyber/v3/util/random"
	"golang.org/x/xerrors"

	"go.dedis.ch/protobuf"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3/byzcoin"
)

// This test-file has some more elaborate structures to handle the RoPaSci
// instances. The following structures are used:
//   * sStruct - basic test-structure for the personhood-tests - holds a ByzCoin
//   * testRPS - more specific RoPaSci structure holding two coins and the LTS
//   * rps - holds one RoPaSci game including the necessary variables for easy testing
//   * sCoin - handles a coin, complete with the name, value, and instanceID

// TestContractRoPaSci does a classical Rock-Paper-Scissors game and checks that all
// combinations give the correct payout.
func TestContractRoPaSci(t *testing.T) {
	rost := newRstSimul()
	d, err := rost.addDarc(nil, "pp")
	require.NoError(t, err)

	for move1 := 0; move1 <= 2; move1++ {
		for move2 := 0; move2 <= 2; move2++ {
			cRPS := &ContractRoPaSci{}
			// Creates
			tr, err := newTestRPS(rost, move1, 100)
			require.NoError(t, err)
			coin1expected := tr.initial - tr.stake
			coin2expected := tr.initial - tr.stake

			switch (3 + move1 - move2) % 3 {
			case 0:
			case 1:
				coin1expected += 2 * tr.stake
			case 2:
				coin2expected += 2 * tr.stake
			}

			// First player creates instance
			inst, err := NewInstructionRoPaSciSpawn(d.GetBaseID(), tr.rpsSpawnStruct())
			require.NoError(t, err)
			scs, _, err := cRPS.Spawn(rost, inst, []byzcoin.Coin{tr.stakeCoin1})
			require.NoError(t, err)
			require.Equal(t, 1, len(scs))
			rost.Process(scs)
			rpsID := byzcoin.NewInstanceID(scs[0].InstanceID)
			require.NoError(t, protobuf.Decode(scs[0].Value, cRPS))

			// Second player withdraws stake and plays his move
			inst = NewInstructionRoPaSciInvokeSecond(tr.coin2, move2)
			inst.InstanceID = rpsID
			scs, _, err = cRPS.Invoke(rost, inst, []byzcoin.Coin{tr.stakeCoin2})
			require.NoError(t, err)
			require.Equal(t, 1, len(scs))
			rost.Process(scs)
			require.NoError(t, protobuf.Decode(scs[0].Value, cRPS))

			// First player reveals his choice
			inst = NewInstructionRoPaSciInvokeConfirm(tr.coin1, tr.prehash)
			inst.InstanceID = rpsID
			scs, _, err = cRPS.Invoke(rost, inst, nil)
			require.NoError(t, err)
			rost.Process(scs)

			coin1New, err := rost.getCoin(tr.coin1)
			require.NoError(t, err)
			coin2New, err := rost.getCoin(tr.coin2)
			require.NoError(t, err)
			require.Equal(t, coin1expected, coin1New.Value)
			require.Equal(t, coin2expected, coin2New.Value)
		}
	}
}

// TestContractRoPaSciCalypso uses the calypso-Rock-Paper-Scissors that stores the
// prehash of player 1 in a CalypsoWrite, so that player 2 doesn't have to wait for
// player 1 to reveal.
func TestContractRoPaSciCalypso(t *testing.T) {
	rost := newRstSimul()
	d, err := rost.addDarc(nil, "pp")
	require.NoError(t, err)

	for move1 := 0; move1 <= 2; move1++ {
		for move2 := 0; move2 <= 2; move2++ {
			cRPS := &ContractRoPaSci{}
			// Creates
			tr, err := newTestRPS(rost, move1, 100)
			require.NoError(t, err)
			coin1expected := tr.initial - tr.stake
			coin2expected := tr.initial - tr.stake

			switch (3 + move1 - move2) % 3 {
			case 0:
			case 1:
				coin1expected += 2 * tr.stake
			case 2:
				coin2expected += 2 * tr.stake
			}

			// First player creates instance
			ltsID := byzcoin.NewInstanceID(random.Bits(256, true, random.New()))
			ltsKP := key.NewKeyPair(cothority.Suite)
			fpHashHash := sha256.Sum256(tr.fpHash)
			wr := calypso.NewWrite(cothority.Suite, ltsID,
				fpHashHash[:], ltsKP.Public, tr.prehash[:28])
			inst, err := NewInstructionRoPaSciSpawnSecret(d.GetBaseID(),
				tr.rpsSpawnStruct(), *wr)
			require.NoError(t, err)
			scs, _, err := cRPS.Spawn(rost, inst, []byzcoin.Coin{tr.stakeCoin1})
			require.NoError(t, err)
			require.Equal(t, 2, len(scs))
			require.Equal(t, calypso.ContractWriteID, scs[0].ContractID)
			rost.Process(scs)
			rpsID := byzcoin.NewInstanceID(scs[1].InstanceID)
			require.NoError(t, protobuf.Decode(scs[1].Value, cRPS))

			// Second player withdraws stake and plays his move
			instP, err := NewInstructionRoPaSciInvokeSecondSecret(tr.coin2,
				move2, cothority.Suite.Point().Base())
			require.NoError(t, err)
			instP.InstanceID = rpsID
			scs, _, err = cRPS.Invoke(rost, *instP,
				[]byzcoin.Coin{tr.stakeCoin2})
			require.NoError(t, err)
			require.Equal(t, 2, len(scs))
			require.Equal(t, calypso.ContractReadID, scs[0].ContractID)
			rost.Process(scs)
			require.NoError(t, protobuf.Decode(scs[1].Value, cRPS))

			// TODO: test if returned calypsoRead allows to recover the prehash

			// First player reveals his choice
			inst = NewInstructionRoPaSciInvokeConfirm(tr.coin1, tr.prehash)
			inst.InstanceID = rpsID
			scs, _, err = cRPS.Invoke(rost, inst, nil)
			require.NoError(t, err)
			rost.Process(scs)

			coin1New, err := rost.getCoin(tr.coin1)
			require.NoError(t, err)
			coin2New, err := rost.getCoin(tr.coin2)
			require.NoError(t, err)
			require.Equal(t, coin1expected, coin1New.Value)
			require.Equal(t, coin2expected, coin2New.Value)
		}
	}
}

type testRPS struct {
	coin1      byzcoin.InstanceID
	coin2      byzcoin.InstanceID
	firstMove  int
	initial    uint64
	stake      uint64
	stakeCoin1 byzcoin.Coin
	stakeCoin2 byzcoin.Coin
	prehash    []byte
	fpHash     []byte
}

// newTestRPS creates a new test-structure for RoPaSci games, including an LTS.
func newTestRPS(s *rstSimul, firstMove int, stake uint64) (tr testRPS,
	err error) {
	tr.initial = 1e6
	tr.coin1, err = s.createCoin("RoPaSci", tr.initial)
	if err != nil {
		err = xerrors.Errorf("couldn't create 1st coin: %+v", err)
	}
	tr.coin2, err = s.createCoin("RoPaSci", tr.initial)
	if err != nil {
		err = xerrors.Errorf("couldn't create 2nd coin: %+v", err)
	}
	tr.firstMove = firstMove
	tr.stake = stake
	_, tr.stakeCoin1, err = s.withdrawCoin(tr.coin1, stake)
	if err != nil {
		err = xerrors.Errorf("couldn't withdraw stake 1: %+v", err)
		return
	}
	_, tr.stakeCoin2, err = s.withdrawCoin(tr.coin2, stake)
	if err != nil {
		err = xerrors.Errorf("couldn't withdraw stake 2: %+v", err)
		return
	}
	// Only take 28 random bytes so that it also fits into the calypsoWrite
	tr.prehash = append([]byte{byte(firstMove)},
		random.Bits(27*8, true, random.New())...)
	tr.prehash = append(tr.prehash, []byte{0, 0, 0, 0}...)
	// Calculate the first player hash with zeroes at the end of the prehash,
	// so that the calypso write,
	// which only has 28 bytes of secret capacity, will be correctly used.
	fpHash := sha256.Sum256(tr.prehash)
	tr.fpHash = fpHash[:]
	return
}

func (tr *testRPS) rpsSpawnStruct() RoPaSciStruct {
	nilInst := byzcoin.NewInstanceID(nil)
	return RoPaSciStruct{
		Description:        "test-RPS",
		Stake:              byzcoin.Coin{},
		FirstPlayerHash:    tr.fpHash,
		FirstPlayer:        -1,
		SecondPlayer:       -1,
		FirstPlayerAccount: &tr.coin1,
		CalypsoWrite:       &nilInst,
		CalypsoRead:        &nilInst,
	}
}
