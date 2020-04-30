package byzcoin

import (
	"bytes"
	"encoding/hex"
	"math/big"
	"strings"
	"testing"
	"time"

	"go.dedis.ch/onet/v3"

	"go.dedis.ch/kyber/v3/util/random"

	"github.com/ethereum/go-ethereum/common/math"
	"go.dedis.ch/onet/v3/log"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3/darc"
	"golang.org/x/xerrors"
)

// TestStateTrie is a sanity check for setting and retrieving keys, values and
// index. The main functionalities are tested in the trie package.
func TestStateTrie(t *testing.T) {
	s := newSer(t, 1, testInterval)
	defer s.local.CloseAll()

	st, err := s.service().getStateTrie(s.genesis.SkipChainID())
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
	entries := 10000

	s := newSer(t, 1, 10*time.Second)
	defer s.local.CloseAll()
	s.local.Check = onet.CheckNone

	st, err := s.service().getStateTrie(s.genesis.SkipChainID())
	require.NoError(t, err)
	require.NotNil(t, st)
	require.NotEqual(t, -1, st.GetIndex())

	log.Lvl1("Creating random entries")
	value := random.Bits(1024, true, random.New())
	start := time.Now()
	var scs []StateChange
	for e := 1; e <= entries; e++ {
		scs = append(scs, StateChange{
			StateAction: Create,
			InstanceID:  random.Bits(256, true, random.New()),
			ContractID:  ContractDarcID,
			Value:       value,
		})
		if e%100 == 0 {
			st.StoreAll(scs, 5, CurrentVersion)
			scs = []StateChange{}
			log.Print(e, time.Now().Sub(start))
			start = time.Now()
		}
	}

	log.Lvl1("Creating darcs")
	counter := big.NewInt(0)
	// Store the tree of darcs in a 2D-array with the root in index 0 and
	// the leafs in index darcDepth-1
	darcs := make([][]*darc.Darc, darcDepth)
	for d := darcDepth - 1; d >= 0; d-- {
		width := math.Exp(big.NewInt(int64(darcWidth)),
			big.NewInt(int64(d))).Int64()
		darcs[d] = make([]*darc.Darc, width)
		// The leafs just hold a dummy reference to a darc
		if d == darcDepth-1 {
			for i := range darcs[d] {
				counter.Add(counter, big.NewInt(1))
				id := make([]byte, 32)
				copy(id, counter.Bytes())
				ids := []darc.Identity{darc.NewIdentityDarc(id)}
				rules := darc.InitRules(ids, ids)
				darcs[d][i] = darc.NewDarc(rules, []byte(counter.String()))
			}
		} else {
			for i := range darcs[d] {
				counter.Add(counter, big.NewInt(1))
				ids := make([]darc.Identity, darcWidth)
				for id := range ids {
					ids[id] = darc.NewIdentityDarc(
						darcs[d+1][i*darcWidth+id].GetBaseID())
				}
				rules := darc.InitRules(ids, ids)
				darcs[d][i] = darc.NewDarc(rules, []byte(counter.String()))
			}
		}
	}

	scs = make([]StateChange, 0, counter.Int64())
	for d := range darcs {
		for w := range darcs[d] {
			td := darcs[d][w]
			log.Lvlf3("darc[%d][%d]: %x has %s", d, w,
				td.GetBaseID(), td.Rules)
			buf, err := td.ToProto()
			require.NoError(t, err)
			scs = append(scs, StateChange{
				StateAction: Create,
				InstanceID:  td.GetBaseID(),
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
		d, err := st.LoadDarcFromTrie(id)
		if err != nil {
			return nil
		}
		return d
	}

	root := darcs[0][0]
	rootSign := root.Rules.GetSignExpr()
	start = time.Now()
	for i := range darcs[darcDepth-1] {
		id := darc.ID(make([]byte, 32))
		copy(id, big.NewInt(int64(i+1)).Bytes())
		darcID := darc.NewIdentityDarc(id)
		log.Print("Searching id", darcID.String())
		err = darc.EvalExprDarc(rootSign, getDarcs, true, darcID.String())
		require.NoError(t, err)
	}
	log.Lvl1("time to search:", time.Now().Sub(start))
}
