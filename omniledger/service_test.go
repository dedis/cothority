package omniledger

import (
	"encoding/binary"
	//"errors"
	"fmt"
	"github.com/dedis/cothority/omniledger/lib"
	"github.com/dedis/kyber/suites"
	"github.com/dedis/onet/network"
	"github.com/dedis/protobuf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"

	bc "github.com/dedis/cothority/byzcoin"
	"github.com/dedis/cothority/darc"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/onet"
)

var tSuite = suites.MustFind("Ed25519")
var testInterval = 500 * time.Millisecond
var shardCount = 2
var serverCount = 10

type ser struct {
	local    *onet.LocalTest
	hosts    []*onet.Server
	roster   *onet.Roster
	services []*Service
	sb       *skipchain.SkipBlock
	value    []byte
	darc     *darc.Darc
	signer   darc.Signer
	tx       bc.ClientTransaction
	interval time.Duration
}

func TestService_CreateOmniLedger(t *testing.T) {
	s := newSer(t, 0, testInterval, serverCount)
	defer s.local.CloseAll()

	// Check roster
	rep, err := s.service().CreateOmniLedger(&CreateOmniLedger{
		Version: bc.CurrentVersion,
	})
	require.NotNil(t, err)

	// Check #shard
	rep, err = s.service().CreateOmniLedger(&CreateOmniLedger{
		Version: bc.CurrentVersion,
		Roster:  *s.roster,
	})
	require.NotNil(t, err)

	// Check epoch size
	rep, err = s.service().CreateOmniLedger(&CreateOmniLedger{
		Version:    bc.CurrentVersion,
		Roster:     *s.roster,
		ShardCount: 1,
	})
	require.NotNil(t, err)

	// Check there is enough validator
	rep, err = s.service().CreateOmniLedger(&CreateOmniLedger{
		Version:    bc.CurrentVersion,
		Roster:     *s.roster,
		ShardCount: 5 * serverCount,
		EpochSize:  1,
	})
	require.NotNil(t, err)

	// Passing argument check
	rep, err = s.service().CreateOmniLedger(getCorrectRequest(s))
	if err != nil {
		fmt.Println("Error: ", err)
	}

	assert.NotNil(t, rep)

	// Verify number of created shard is correct
	//fmt.Println(len(rep.ShardRoster))
	assert.True(t, len(rep.ShardRoster) == shardCount)

	// Verify each shard has enough validators, i.e. >= 4
	for _, shard := range rep.ShardRoster {
		valPerShard := len(shard.List)
		assert.True(t, valPerShard == 4 || valPerShard > 4)
	}

	// Verify no two shards have same node
	m := make(map[network.ServerIdentity]int)
	for _, shard := range rep.ShardRoster {
		for _, si := range shard.List {
			m[*si]++
		}
	}
	for k := range m {
		assert.True(t, m[k] < 2)
	}
}

func newSer(t *testing.T, step int, interval time.Duration, serverCount int) *ser {
	return newSerN(t, step, interval, serverCount, false)
}

func newSerN(t *testing.T, step int, interval time.Duration, n int, viewchange bool) *ser {
	s := &ser{
		local:  onet.NewLocalTestT(tSuite, t),
		value:  []byte("anyvalue"),
		signer: darc.NewSignerEd25519(nil, nil),
	}
	s.hosts, s.roster, _ = s.local.GenTree(n, true)
	for _, sv := range s.local.GetServices(s.hosts, OmniLedgerID) {
		service := sv.(*Service)
		s.services = append(s.services, service)
	}

	return s
}

func (s *ser) service() *Service {
	return s.services[0]
}

func getCorrectRequest(s *ser) *CreateOmniLedger {
	owner := s.signer
	version := bc.CurrentVersion
	roster := s.roster
	//shardCount := 2
	epochSize := 500 * time.Millisecond

	//c := bc.NewClient(nil, *roster)

	ibMsg, err := bc.DefaultGenesisMsg(version, roster, []string{"spawn:darc", "spawn:omniledgerepoch"}, owner.Identity())
	if err != nil {
		fmt.Println("ERR: ", err)
	}

	d := ibMsg.GenesisDarc

	darcBuf, err := protobuf.Encode(&d)
	if err != nil {
		fmt.Println("ERR: ", err)
	}

	scBuff := make([]byte, 8)
	binary.PutVarint(scBuff, int64(shardCount))

	esBuff := lib.EncodeDuration(epochSize)

	rosterBuf, err := protobuf.Encode(roster)
	if err != nil {
		fmt.Println("ERR: ", err)
	}

	ts := time.Now()
	tsBuf := make([]byte, 8)
	binary.BigEndian.PutUint64(tsBuf, uint64(ts.Unix()))

	/*signerCtrs, err := c.GetSignerCounters(owner.Identity().String())
	if err != nil {
		fmt.Println("ERR: ", err)
	}
	if len(signerCtrs.Counters) != 1 {
		err = errors.New("incorrect signer counter length")
		fmt.Println("ERR: ", err)
	}*/

	instr := bc.Instruction{
		InstanceID: bc.NewInstanceID(d.GetBaseID()),
		Spawn: &bc.Spawn{
			ContractID: ContractOmniledgerEpochID,
			Args: []bc.Argument{
				bc.Argument{Name: "darc", Value: darcBuf},
				bc.Argument{Name: "roster", Value: rosterBuf},
				bc.Argument{Name: "shardCount", Value: scBuff},
				bc.Argument{Name: "epochSize", Value: esBuff},
				bc.Argument{Name: "timestamp", Value: tsBuf},
			},
		},
		//SignerCounter: []uint64{signerCtrs.Counters[0] + 1},
		SignerCounter: []uint64{1},
	}
	//err = instr.SignBy(d.GetBaseID(), owner)
	spawnTx := &bc.ClientTransaction{
		Instructions: bc.Instructions{instr},
	}
	spawnTx.SignWith(owner)
	spawnTx.InstructionsHash = spawnTx.Instructions.Hash()

	return &CreateOmniLedger{
		Version:      version,
		Roster:       *roster,
		ShardCount:   shardCount,
		EpochSize:    epochSize,
		IBGenesisMsg: ibMsg,
		OwnerID:      owner.Identity(),
		SpawnTx:      spawnTx,
		Timestamp:    ts,
	}
}
