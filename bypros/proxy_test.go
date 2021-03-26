package bypros

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3/bypros/storage"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/byzcoin/contracts"
	"go.dedis.ch/cothority/v3/skipchain"
)

const valueRule = "spawn:value"

func TestMain(m *testing.M) {
	storageFac = newFakeStorage
	os.Exit(m.Run())
}

func TestProxyFollow_No_Token(t *testing.T) {
	s := Service{}

	req := &Follow{}

	_, err := s.Follow(req)
	require.EqualError(t, err, "already following")
}

func TestProxyFollow_One_Block(t *testing.T) {
	bct := byzcoin.NewBCTestDefault(t)
	defer bct.CloseAll()

	bct.AddGenesisRules(valueRule)
	bct.CreateByzCoin()

	storage := &fakeStorage{}

	s := Service{
		follow:  make(chan struct{}, 1),
		storage: storage,
	}

	defer s.notifyStop()

	s.follow <- struct{}{}

	req := &Follow{
		ScID:   bct.Genesis.Hash,
		Target: bct.Roster.Get(0),
	}
	_, err := s.Follow(req)
	require.NoError(t, err)

	require.True(t, s.following)

	require.Len(t, storage.getBlocks(), 0)

	bct.SendInst(&byzcoin.TxArgsDefault, byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(bct.GenesisDarc.GetBaseID()),
		Spawn: &byzcoin.Spawn{
			ContractID: contracts.ContractValueID,
		},
	})

	// we should have received the added block
	require.Len(t, storage.getBlocks(), 1)
}

func TestProxyFollow_Many_Blocks(t *testing.T) {

	bct := byzcoin.NewBCTestDefault(t)
	defer bct.CloseAll()

	bct.AddGenesisRules(valueRule)
	bct.CreateByzCoin()

	bct.SendInst(&byzcoin.TxArgsDefault, byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(bct.GenesisDarc.GetBaseID()),
		Spawn: &byzcoin.Spawn{
			ContractID: contracts.ContractValueID,
		},
	})

	storage := &fakeStorage{}

	s := Service{
		follow:  make(chan struct{}, 1),
		storage: storage,
	}

	defer s.notifyStop()

	s.follow <- struct{}{}

	req := &Follow{
		ScID:   bct.Genesis.Hash,
		Target: bct.Roster.Get(0),
	}
	_, err := s.Follow(req)
	require.NoError(t, err)

	require.True(t, s.following)

	require.Len(t, storage.getBlocks(), 0)

	bct.SendInst(&byzcoin.TxArgsDefault, byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(bct.GenesisDarc.GetBaseID()),
		Spawn: &byzcoin.Spawn{
			ContractID: contracts.ContractValueID,
		},
	})

	bct.SendInst(&byzcoin.TxArgsDefault, byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(bct.GenesisDarc.GetBaseID()),
		Spawn: &byzcoin.Spawn{
			ContractID: contracts.ContractValueID,
		},
	})

	// we should have received the added block
	require.Len(t, storage.getBlocks(), 2)
}

func TestProxyFollow_Wrong_Skipchain(t *testing.T) {
	// once the services uses a specific skipchain, it prevents users from
	// providing another one.

	bct := byzcoin.NewBCTestDefault(t)
	defer bct.CloseAll()

	bct.AddGenesisRules(valueRule)
	bct.CreateByzCoin()

	storage := &fakeStorage{}

	s := Service{
		follow:  make(chan struct{}, 2),
		storage: storage,
	}

	defer s.notifyStop()

	s.follow <- struct{}{}
	s.follow <- struct{}{}

	req := &Follow{
		ScID:   bct.Genesis.Hash,
		Target: bct.Roster.Get(0),
	}
	_, err := s.Follow(req)
	require.NoError(t, err)

	sbID2 := skipchain.SkipBlockID{0xaa}
	req.ScID = sbID2

	_, err = s.Follow(req)
	require.EqualError(t, err, fmt.Sprintf("wrong skipchain ID: expected '%x', got '%x'", bct.Genesis.Hash, sbID2))

	time.Sleep(time.Second)
}

func TestProxyFollow_UnFollow(t *testing.T) {

	bct := byzcoin.NewBCTestDefault(t)
	defer bct.CloseAll()

	bct.AddGenesisRules(valueRule)
	bct.CreateByzCoin()

	storage := &fakeStorage{}

	s := Service{
		follow:  make(chan struct{}, 1),
		storage: storage,
	}

	defer s.notifyStop()

	s.follow <- struct{}{}

	req := &Follow{
		ScID:   bct.Genesis.Hash,
		Target: bct.Roster.Get(0),
	}
	_, err := s.Follow(req)
	require.NoError(t, err)

	require.True(t, s.following)

	require.Len(t, storage.getBlocks(), 0)

	bct.SendInst(&byzcoin.TxArgsDefault, byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(bct.GenesisDarc.GetBaseID()),
		Spawn: &byzcoin.Spawn{
			ContractID: contracts.ContractValueID,
		},
	})

	// we should have received the added block
	require.Len(t, storage.getBlocks(), 1)

	_, err = s.Unfollow(&Unfollow{})
	require.NoError(t, err)

	bct.SendInst(&byzcoin.TxArgsDefault, byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(bct.GenesisDarc.GetBaseID()),
		Spawn: &byzcoin.Spawn{
			ContractID: contracts.ContractValueID,
		},
	})

	// we should still have the same number of blocks
	require.Len(t, storage.getBlocks(), 1)

	_, err = s.Follow(req)
	require.NoError(t, err)

	bct.SendInst(&byzcoin.TxArgsDefault, byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(bct.GenesisDarc.GetBaseID()),
		Spawn: &byzcoin.Spawn{
			ContractID: contracts.ContractValueID,
		},
	})

	// we should have a new block
	require.Len(t, storage.getBlocks(), 2)
}

func TestProxyCatchUp_Genesis(t *testing.T) {

	bct := byzcoin.NewBCTestDefault(t)
	defer bct.CloseAll()

	bct.AddGenesisRules(valueRule)
	bct.CreateByzCoin()

	storage := &fakeStorage{}

	s := Service{
		follow:  make(chan struct{}, 1),
		storage: storage,
	}

	resp, stop, err := s.CatchUP(&CatchUpMsg{
		ScID:        bct.Genesis.Hash,
		Target:      bct.Roster.Get(0),
		FromBlock:   bct.Genesis.Hash,
		UpdateEvery: 1,
	})

	require.NoError(t, err)
	defer close(stop)

	expected := []*CatchUpResponse{
		{
			Status: CatchUpStatus{
				Message:    "parsed block 0",
				BlockIndex: 0,
				BlockHash:  bct.Genesis.Hash,
			},
		},
		{
			Done: true,
		},
	}

	for i := 0; i < len(expected); i++ {
		select {
		case r := <-resp:
			require.Equal(t, expected[i], r)
		case <-time.After(time.Second * 5):
			t.Error("timeout on catch up response")
		}
	}

	select {
	case r, more := <-resp:
		require.False(t, more, "unexpected resp:", r)
	default:
	}

	// we should have the genesis block
	require.Len(t, storage.getBlocks(), 1)
}

func TestProxyCatchUp_Wrong_Skipchain(t *testing.T) {

	bct := byzcoin.NewBCTestDefault(t)
	defer bct.CloseAll()

	bct.AddGenesisRules(valueRule)
	bct.CreateByzCoin()

	storage := &fakeStorage{}

	s := Service{
		storage: storage,
	}

	_, stop, err := s.CatchUP(&CatchUpMsg{
		ScID:        bct.Genesis.Hash,
		Target:      bct.Roster.Get(0),
		FromBlock:   bct.Genesis.Hash,
		UpdateEvery: 9,
	})

	close(stop)
	require.NoError(t, err)

	sbID2 := skipchain.SkipBlockID{0xaa}
	_, stop2, err := s.CatchUP(&CatchUpMsg{
		ScID:        sbID2,
		Target:      bct.Roster.Get(0),
		FromBlock:   bct.Genesis.Hash,
		UpdateEvery: 9,
	})

	require.EqualError(t, err, fmt.Sprintf("wrong skipchain ID: expected '%x', got '%x'", bct.Genesis.Hash, sbID2))
	require.Nil(t, stop2)
}

func TestProxyCatchUp_Multiple_Blocks(t *testing.T) {

	bct := byzcoin.NewBCTestDefault(t)
	defer bct.CloseAll()

	bct.AddGenesisRules(valueRule)
	bct.CreateByzCoin()

	storage := &fakeStorage{}

	s := Service{
		follow:  make(chan struct{}, 1),
		storage: storage,
	}

	// Adding 3 blocks
	for i := 0; i < 3; i++ {
		bct.SendInst(&byzcoin.TxArgsDefault, byzcoin.Instruction{
			InstanceID: byzcoin.NewInstanceID(bct.GenesisDarc.GetBaseID()),
			Spawn: &byzcoin.Spawn{
				ContractID: contracts.ContractValueID,
			},
		})
	}

	resp, stop, err := s.CatchUP(&CatchUpMsg{
		ScID:        bct.Genesis.Hash,
		Target:      bct.Roster.Get(0),
		FromBlock:   bct.Genesis.Hash,
		UpdateEvery: 2, // will be notified only for block with index 1 and 3
	})

	require.NoError(t, err)
	defer close(stop)

	expected := []*CatchUpResponse{
		{
			Status: CatchUpStatus{
				Message:    "parsed block 1",
				BlockIndex: 1,
			},
		},
		{
			Status: CatchUpStatus{
				Message:    "parsed block 3",
				BlockIndex: 3,
			},
		},
		{
			Done: true,
		},
	}

	for i := 0; i < len(expected); i++ {
		select {
		case r := <-resp:
			require.Equal(t, expected[i].Status.Message, r.Status.Message)
			require.Equal(t, expected[i].Status.BlockIndex, r.Status.BlockIndex)
			require.Equal(t, expected[i].Err, r.Err)
			require.Equal(t, expected[i].Done, r.Done)
		case <-time.After(time.Second):
			t.Error("timeout on catch up response")
		}
	}

	select {
	case r := <-resp:
		t.Error("unexpected resp:", r)
	default:
	}

	// we should have 4 blocks
	require.Len(t, storage.getBlocks(), 4)
}

func TestProxyCatchUp_Query(t *testing.T) {

	bct := byzcoin.NewBCTestDefault(t)
	defer bct.CloseAll()

	bct.AddGenesisRules(valueRule)
	bct.CreateByzCoin()

	storage := &fakeStorage{}

	s := Service{
		follow:  make(chan struct{}, 1),
		storage: storage,
	}

	req := Query{
		Query: "fake query",
	}

	reply, err := s.Query(&req)
	require.NoError(t, err)

	// the service is only responsible for calling the query on the storage
	require.Equal(t, []byte("fake query"), reply.Result)
}

func TestProxyUnFollow_Error(t *testing.T) {

	bct := byzcoin.NewBCTestDefault(t)
	defer bct.CloseAll()

	s := Service{
		following: false,
	}

	req := &Unfollow{}
	_, err := s.Unfollow(req)
	require.EqualError(t, err, "not following")
}

// -----------------------------------------------------------------------------
// Utility functions

func newFakeStorage() (storage.Storage, error) {
	return &fakeStorage{}, nil
}

type fakeStorage struct {
	sync.Mutex

	storage.Storage

	storeBlocks []*skipchain.SkipBlock
}

// GetBlock should return the block id from the storage, or -1 if not found.
func (s *fakeStorage) GetBlock(blockHash []byte) (int, error) {
	return -1, nil
}

// StoreBlock should store the block.
func (s *fakeStorage) StoreBlock(block *skipchain.SkipBlock) (int, error) {
	s.Lock()
	defer s.Unlock()

	s.storeBlocks = append(s.storeBlocks, block)

	return -1, nil
}

// Query executes the query and returns the result.
func (s *fakeStorage) Query(query string) ([]byte, error) {
	return []byte(query), nil
}

func (s *fakeStorage) getBlocks() []*skipchain.SkipBlock {
	s.Lock()
	defer s.Unlock()

	return append([]*skipchain.SkipBlock{}, s.storeBlocks...)
}
