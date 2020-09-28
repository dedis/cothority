package byzcoin

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"math/big"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/xerrors"

	"github.com/stretchr/testify/require"

	"go.dedis.ch/onet/v3/log"

	"go.dedis.ch/kyber/v3/util/random"

	"go.dedis.ch/cothority/v3/darc"
)

// TestStateTrie is a sanity check for setting and retrieving keys, values and
// index. The main functionalities are tested in the trie package.
func TestStateTrie(t *testing.T) {
	b := NewBCTest(t)
	defer b.CloseAll()

	st, err := b.Service().getStateTrie(b.Genesis.SkipChainID())
	require.NoError(t, err)
	require.NotNil(t, st)
	require.NotEqual(t, -1, st.GetIndex())

	key := []byte("testInstance")
	contractID := "testContract"
	value := []byte("testValue")
	version := uint64(123)
	darcID := darc.ID([]byte("123"))
	sc := StateChange{
		StateAction: Create,
		InstanceID:  key,
		ContractID:  contractID,
		Value:       value,
		Version:     version,
		DarcID:      darcID,
	}
	// store with bad expected root hash should fail, value should not be inside
	require.Error(t, st.VerifiedStoreAll([]StateChange{sc}, 5, CurrentVersion, []byte("badhash")))
	_, _, _, _, err = st.GetValues(key)
	require.True(t, xerrors.Is(err, errKeyNotSet))

	// store the state changes normally using StoreAll and it should work
	require.NoError(t, st.StoreAll([]StateChange{sc}, 5, CurrentVersion))
	require.Equal(t, st.GetIndex(), 5)

	require.NoError(t, st.StoreAll([]StateChange{sc}, 6, CurrentVersion))
	require.Equal(t, st.GetIndex(), 6)

	_, _, _, _, err = st.GetValues(append(key, byte(0)))
	require.True(t, xerrors.Is(err, errKeyNotSet))

	val, ver, cid, did, err := st.GetValues(key)
	require.NoError(t, err)
	require.Equal(t, value, val)
	require.Equal(t, version, ver)
	require.Equal(t, cid, string(contractID))
	require.True(t, did.Equal(darcID))

	// test the staging state trie, most of the tests are done in the trie package
	key2 := []byte("key2")
	val2 := []byte("val2")
	sst := st.MakeStagingStateTrie()
	oldRoot := sst.GetRoot()
	require.NoError(t, sst.Set(key2, val2))
	newRoot := sst.GetRoot()
	require.False(t, bytes.Equal(oldRoot, newRoot))
	candidateVal2, err := sst.Get(key2)
	require.NoError(t, err)
	require.True(t, bytes.Equal(val2, candidateVal2))

	// test the commit of staging state trie, root should be computed differently now, but it should be the same
	require.NoError(t, sst.Commit())
	require.True(t, bytes.Equal(sst.GetRoot(), newRoot))
}

// TestDarcRetrieval is used as a benchmark here. It stores a lot of darcs
// that are all linked in a tree-structure, and then uses EvalExprDarc to
// check whether an identity is valid or not.
// Use with
//   go test -v -cpuprofile cpu.prof -run TestDarcRetrieval
//   go tool pprof -pdf cpu.prof
func TestDarcRetrieval(t *testing.T) {
	darcDepth := 3
	darcWidth := 10
	entries := 1000

	bArgs := NewBCTestArgs()
	bArgs.PropagationInterval = 10 * time.Second
	b := NewBCTestWithArgs(t, bArgs)
	defer b.CloseAll()
	// When running with `-cpuprofile`, additional go-routines are added,
	// which should be ignored.
	log.AddUserUninterestingGoroutine("/runtime/cpuprof.go")

	st, err := b.Service().getStateTrie(b.Genesis.SkipChainID())
	require.NoError(t, err)
	require.NotEqual(t, -1, st.GetIndex())

	log.Lvl1("Creating random entries")
	value := make([]byte, 128)
	scs := make([]StateChange, entries)
	for e := 0; e < entries; e++ {
		scs[e] = StateChange{
			StateAction: Create,
			InstanceID:  random.Bits(256, true, random.New()),
			ContractID:  ContractDarcID,
			Value:       value,
		}
	}
	require.NoError(t, st.StoreAll(scs, 5, CurrentVersion))

	log.Lvl1("Creating darcs")
	counter := 0
	// Store the tree of darcs in a 2D-array with the root in index 0 and
	// the leafs in index darcDepth-1
	darcs := make([][]*darc.Darc, darcDepth)
	for d := darcDepth - 1; d >= 0; d-- {
		width := big.NewInt(0).Exp(big.NewInt(int64(darcWidth)),
			big.NewInt(int64(d)), nil).Int64()
		darcs[d] = make([]*darc.Darc, width)
		// The leafs just hold a dummy reference to a darc
		if d == darcDepth-1 {
			for i := range darcs[d] {
				counter++
				id := make([]byte, 32)
				binary.BigEndian.PutUint64(id, uint64(counter))
				ids := []darc.Identity{darc.NewIdentityDarc(id)}
				rules := darc.InitRules(ids, ids)
				darcs[d][i] = darc.NewDarc(rules, []byte(strconv.Itoa(counter)))
			}
		} else {
			for i := range darcs[d] {
				counter++
				ids := make([]darc.Identity, darcWidth)
				for id := range ids {
					ids[id] = darc.NewIdentityDarc(
						darcs[d+1][i*darcWidth+id].GetBaseID())
				}
				rules := darc.InitRules(ids, ids)
				darcs[d][i] = darc.NewDarc(rules, []byte(strconv.Itoa(counter)))
			}
		}
	}

	scs = make([]StateChange, 0, counter)
	for d, dd := range darcs {
		for w, dw := range dd {
			log.Lvlf3("darc[%d][%d]: %x has %s", d, w,
				dw.GetBaseID(), dw.Rules)
			buf, err := dw.ToProto()
			require.NoError(t, err)
			scs = append(scs, StateChange{
				StateAction: Create,
				InstanceID:  dw.GetBaseID(),
				ContractID:  ContractDarcID,
				Value:       buf,
				Version:     0,
			})
		}
	}
	require.NoError(t, st.StoreAll(scs, 5, CurrentVersion))

	getDarcs := func(s string, latest bool) *darc.Darc {
		if !latest {
			log.Error("cannot handle intermediate darcs")
			return nil
		}
		id, err := hex.DecodeString(strings.Replace(s, "darc:", "", 1))
		if err != nil || len(id) != 32 {
			log.Error("invalid darc id", s, len(id), err)
			return nil
		}
		d, err := st.LoadDarc(id)
		if err != nil {
			return nil
		}
		return d
	}

	log.Lvl1("Starting to search")
	root := darcs[0][0]
	rootSign := root.Rules.GetSignExpr()
	start := time.Now()
	for i := range darcs[darcDepth-1] {
		id := darc.ID(make([]byte, 32))
		binary.BigEndian.PutUint64(id, uint64(i+1))
		darcID := darc.NewIdentityDarc(id)
		err = darc.EvalExprDarc(rootSign, getDarcs, true, darcID.String())
		require.NoError(t, err)
	}
	log.Lvl1("time to search:", time.Since(start))
}
