package byzcoin

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3/skipchain"
)

var chanTimeout = time.Millisecond * 100

func TestStreamingService_PaginateBlocks(t *testing.T) {
	// Creating a service with only the genesis block
	s := newSerN(t, 1, testInterval, 4, disableViewChange)
	defer s.local.CloseAll()
	service := s.service()

	// We should be able to get 1 page with one item, which is the genesis block
	paginateRequest := &PaginateRequest{
		StartID:  s.genesis.Hash,
		PageSize: 1,
		NumPages: 1,
		Backward: false,
		StreamID: nil,
	}
	paginateResponse, closeChan, err := service.PaginateBlocks(paginateRequest)
	defer close(closeChan)
	require.NoError(t, err)

	select {
	case response := <-paginateResponse:
		if response.ErrorCode != 0 {
			t.Errorf("expected to find error code 0, but found %d, here are "+
				"the messages: %v", response.ErrorCode, response.ErrorText)
		}
		require.Equal(t, 1, len(response.Blocks))
		require.Equal(t, response.Blocks[0].Hash, s.genesis.Hash)
	case <-time.After(chanTimeout):
		t.Fatal("didn't get a papginateResponse in the channel after 1 second")
	}
	select {
	case <-paginateResponse:
		t.Fatal("there shouldn't be additional element in the channel")
	case <-time.After(chanTimeout):
	}

	// Trying to fetch 2 items per page should raise an error since there is
	// only the genesis block
	paginateRequest = &PaginateRequest{
		StartID:  s.genesis.Hash,
		PageSize: 2,
		NumPages: 1,
		Backward: false,
		StreamID: nil,
	}
	paginateResponse, closeChan, err = service.PaginateBlocks(paginateRequest)
	defer close(closeChan)
	require.NoError(t, err)

	select {
	case response := <-paginateResponse:
		require.Greater(t, response.ErrorCode, uint64(0))
		require.Equal(t, 0, len(response.Blocks))
	case <-time.After(chanTimeout):
		t.Fatal("didn't get a papginateResponse in the channel after 1 second")
	}
	select {
	case <-paginateResponse:
		t.Fatal("there shouldn't be additional element in the channel")
	case <-time.After(chanTimeout):
	}

	// Trying to fetch 2 pages should raise an error in the second page since
	// there is only the genesis block
	paginateRequest = &PaginateRequest{
		StartID:  s.genesis.Hash,
		PageSize: 1,
		NumPages: 2,
		Backward: false,
		StreamID: nil,
	}
	paginateResponse, closeChan, err = service.PaginateBlocks(paginateRequest)
	defer close(closeChan)
	require.NoError(t, err)

	select {
	case response := <-paginateResponse:
		if response.ErrorCode != 0 {
			t.Errorf("expected to find error code 0, but found %d, here are "+
				"the messages: %v", response.ErrorCode, response.ErrorText)
		}
		require.Equal(t, 1, len(response.Blocks))
		require.Equal(t, response.Blocks[0].Hash, s.genesis.Hash)
	case <-time.After(chanTimeout):
		t.Fatal("didn't get a papginateResponse in the channel after 1 second")
	}
	select {
	case response := <-paginateResponse:
		require.Greater(t, response.ErrorCode, uint64(0))
		require.Equal(t, 0, len(response.Blocks))
	case <-time.After(chanTimeout):
		t.Fatal("didn't get a papginateResponse in the channel after 1 second")
	}
	select {
	case <-paginateResponse:
		t.Fatal("there shouldn't be additional element in the channel")
	case <-time.After(chanTimeout):
	}

	// Adding a new block so we can fetch a page of two blocks, or two pages
	// with one item each.
	tx, err := createOneClientTx(s.darc.GetBaseID(), dummyContract, s.value, s.signer)
	require.NoError(t, err)
	s.tx = tx
	resp, err := s.service().AddTransaction(&AddTxRequest{
		Version:       CurrentVersion,
		SkipchainID:   s.genesis.SkipChainID(),
		Transaction:   tx,
		InclusionWait: 10,
	})
	transactionOK(t, resp, err)

	// Fetching two items in one page
	paginateRequest = &PaginateRequest{
		StartID:  s.genesis.Hash,
		PageSize: 2,
		NumPages: 1,
		Backward: false,
		StreamID: nil,
	}
	paginateResponse, closeChan, err = service.PaginateBlocks(paginateRequest)
	defer close(closeChan)
	require.NoError(t, err)
	var secondBlockHash skipchain.SkipBlockID

	select {
	case response := <-paginateResponse:
		if response.ErrorCode != 0 {
			t.Errorf("expected to find error code 0, but found %d, here are "+
				"the messages: %v", response.ErrorCode, response.ErrorText)
		}
		require.Equal(t, 2, len(response.Blocks))
		require.Equal(t, response.Blocks[0].Hash, s.genesis.Hash)
		secondBlockHash = response.Blocks[1].Hash
	case <-time.After(chanTimeout):
		t.Fatal("didn't get a papginateResponse in the channel after 1 second")
	}
	select {
	case <-paginateResponse:
		t.Fatal("there shouldn't be additional element in the channel")
	case <-time.After(chanTimeout):
	}

	// Fecthing two pages with 1 item each
	paginateRequest = &PaginateRequest{
		StartID:  s.genesis.Hash,
		PageSize: 1,
		NumPages: 2,
		Backward: false,
		StreamID: nil,
	}
	paginateResponse, closeChan, err = service.PaginateBlocks(paginateRequest)
	defer close(closeChan)
	require.NoError(t, err)

	select {
	case response := <-paginateResponse:
		if response.ErrorCode != 0 {
			t.Errorf("expected to find error code 0, but found %d, here are "+
				"the messages: %v", response.ErrorCode, response.ErrorText)
		}
		require.Equal(t, 1, len(response.Blocks))
		require.Equal(t, response.Blocks[0].Hash, s.genesis.Hash)
	case <-time.After(chanTimeout):
		t.Fatal("didn't get a papginateResponse in the channel after 1 second")
	}
	select {
	case response := <-paginateResponse:
		if response.ErrorCode != 0 {
			t.Errorf("expected to find error code 0, but found %d, here are "+
				"the messages: %v", response.ErrorCode, response.ErrorText)
		}
		require.Equal(t, 1, len(response.Blocks))
		require.Equal(t, response.Blocks[0].Hash, secondBlockHash)
	case <-time.After(chanTimeout):
		t.Fatal("didn't get a papginateResponse in the channel after 1 second")
	}
	select {
	case <-paginateResponse:
		t.Fatal("there shouldn't be additional element in the channel")
	case <-time.After(chanTimeout):
	}

	// If we get the page in reverse order from the genesis block we should get
	// an error
	paginateRequest = &PaginateRequest{
		StartID:  s.genesis.Hash,
		PageSize: 2,
		NumPages: 1,
		Backward: true,
		StreamID: nil,
	}
	paginateResponse, closeChan, err = service.PaginateBlocks(paginateRequest)
	defer close(closeChan)
	require.NoError(t, err)

	select {
	case response := <-paginateResponse:
		require.Greater(t, response.ErrorCode, uint64(0))
		require.Equal(t, 0, len(response.Blocks))
	case <-time.After(chanTimeout):
		t.Fatal("didn't get a papginateResponse in the channel after 1 second")
	}
	select {
	case <-paginateResponse:
		t.Fatal("there shouldn't be additional element in the channel")
	case <-time.After(chanTimeout):
	}

	// Trying to fetch 2 pages from the genesis block in reverse order should
	// raise an error in the second page
	paginateRequest = &PaginateRequest{
		StartID:  s.genesis.Hash,
		PageSize: 1,
		NumPages: 2,
		Backward: true,
		StreamID: nil,
	}
	paginateResponse, closeChan, err = service.PaginateBlocks(paginateRequest)
	defer close(closeChan)
	require.NoError(t, err)

	select {
	case response := <-paginateResponse:
		if response.ErrorCode != 0 {
			t.Errorf("expected to find error code 0, but found %d, here are "+
				"the messages: %v", response.ErrorCode, response.ErrorText)
		}
		require.Equal(t, 1, len(response.Blocks))
		require.Equal(t, response.Blocks[0].Hash, s.genesis.Hash)
	case <-time.After(chanTimeout):
		t.Fatal("didn't get a papginateResponse in the channel after 1 second")
	}
	select {
	case response := <-paginateResponse:
		require.Greater(t, response.ErrorCode, uint64(0))
		require.Equal(t, 0, len(response.Blocks))
	case <-time.After(chanTimeout):
		t.Fatal("didn't get a papginateResponse in the channel after 1 second")
	}
	select {
	case <-paginateResponse:
		t.Fatal("there shouldn't be additional element in the channel")
	case <-time.After(chanTimeout):
	}

	// Fetching two items in one page from the second block in reverse order
	// should be allright
	paginateRequest = &PaginateRequest{
		StartID:  secondBlockHash,
		PageSize: 2,
		NumPages: 1,
		Backward: true,
		StreamID: nil,
	}
	paginateResponse, closeChan, err = service.PaginateBlocks(paginateRequest)
	defer close(closeChan)
	require.NoError(t, err)

	select {
	case response := <-paginateResponse:
		if response.ErrorCode != 0 {
			t.Errorf("expected to find error code 0, but found %d, here are "+
				"the messages: %v", response.ErrorCode, response.ErrorText)
		}
		require.Equal(t, 2, len(response.Blocks))
		require.Equal(t, response.Blocks[0].Hash, secondBlockHash)
		require.Equal(t, response.Blocks[1].Hash, s.genesis.Hash)
	case <-time.After(chanTimeout):
		t.Fatal("didn't get a papginateResponse in the channel after 1 second")
	}
	select {
	case <-paginateResponse:
		t.Fatal("there shouldn't be additional element in the channel")
	case <-time.After(chanTimeout):
	}

	// Fecthing two pages with 1 item each per page from the second block should
	// also be allright
	paginateRequest = &PaginateRequest{
		StartID:  secondBlockHash,
		PageSize: 1,
		NumPages: 2,
		Backward: true,
		StreamID: nil,
	}
	paginateResponse, closeChan, err = service.PaginateBlocks(paginateRequest)
	defer close(closeChan)
	require.NoError(t, err)

	select {
	case response := <-paginateResponse:
		if response.ErrorCode != 0 {
			t.Errorf("expected to find error code 0, but found %d, here are "+
				"the messages: %v", response.ErrorCode, response.ErrorText)
		}
		require.Equal(t, 1, len(response.Blocks))
		require.Equal(t, response.Blocks[0].Hash, secondBlockHash)
	case <-time.After(chanTimeout):
		t.Fatal("didn't get a papginateResponse in the channel after 1 second")
	}
	select {
	case response := <-paginateResponse:
		if response.ErrorCode != 0 {
			t.Errorf("expected to find error code 0, but found %d, here are "+
				"the messages: %v", response.ErrorCode, response.ErrorText)
		}
		require.Equal(t, 1, len(response.Blocks))
		require.Equal(t, response.Blocks[0].Hash, s.genesis.Hash)
	case <-time.After(chanTimeout):
		t.Fatal("didn't get a papginateResponse in the channel after 1 second")
	}
	select {
	case <-paginateResponse:
		t.Fatal("there shouldn't be additional element in the channel")
	case <-time.After(chanTimeout):
	}
}
