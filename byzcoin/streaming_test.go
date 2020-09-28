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
	b := NewBCTest(t)
	defer b.CloseAll()
	service := b.Service()

	// We should be able to get 1 page with one item, which is the genesis block
	paginateRequest := &PaginateRequest{
		StartID:  b.Genesis.Hash,
		PageSize: 1,
		NumPages: 1,
		Backward: false,
	}
	paginateResponse, closeChan, err := service.PaginateBlocks(paginateRequest)
	require.NoError(t, err)

	select {
	case response := <-paginateResponse:
		if response.ErrorCode != 0 {
			t.Errorf("expected to find error code 0, but found %d, here are "+
				"the messages: %v", response.ErrorCode, response.ErrorText)
		}
		require.Equal(t, 1, len(response.Blocks))
		require.Equal(t, response.Blocks[0].Hash, b.Genesis.Hash)
	case <-time.After(chanTimeout):
		t.Fatal("didn't get a papginateResponse in the channel after timeout")
	}
	select {
	case <-paginateResponse:
		t.Fatal("there shouldn't be additional element in the channel")
	case <-time.After(chanTimeout):
	}

	close(closeChan)

	// Trying to fetch 2 items per page should raise an error since there is
	// only the genesis block
	paginateRequest = &PaginateRequest{
		StartID:  b.Genesis.Hash,
		PageSize: 2,
		NumPages: 1,
		Backward: false,
	}
	paginateResponse, closeChan, err = service.PaginateBlocks(paginateRequest)
	require.NoError(t, err)

	select {
	case response := <-paginateResponse:
		require.Greater(t, response.ErrorCode, uint64(0))
		require.Equal(t, 0, len(response.Blocks))
	case <-time.After(chanTimeout):
		t.Fatal("didn't get a papginateResponse in the channel after timeout")
	}
	select {
	case <-paginateResponse:
		t.Fatal("there shouldn't be additional element in the channel")
	case <-time.After(chanTimeout):
	}

	close(closeChan)

	// Trying to fetch 2 pages should raise an error in the second page since
	// there is only the genesis block
	paginateRequest = &PaginateRequest{
		StartID:  b.Genesis.Hash,
		PageSize: 1,
		NumPages: 2,
		Backward: false,
	}
	paginateResponse, closeChan, err = service.PaginateBlocks(paginateRequest)
	require.NoError(t, err)

	select {
	case response := <-paginateResponse:
		if response.ErrorCode != 0 {
			t.Errorf("expected to find error code 0, but found %d, here are "+
				"the messages: %v", response.ErrorCode, response.ErrorText)
		}
		require.Equal(t, 1, len(response.Blocks))
		require.Equal(t, response.Blocks[0].Hash, b.Genesis.Hash)
	case <-time.After(chanTimeout):
		t.Fatal("didn't get a papginateResponse in the channel after timeout")
	}
	select {
	case response := <-paginateResponse:
		require.Equal(t, response.ErrorCode, uint64(4))
		require.Equal(t, len(response.ErrorText), 3)
		require.Equal(t, 0, len(response.Blocks))
	case <-time.After(chanTimeout):
		t.Fatal("didn't get a papginateResponse in the channel after timeout")
	}
	select {
	case <-paginateResponse:
		t.Fatal("there shouldn't be additional element in the channel")
	case <-time.After(chanTimeout):
	}

	close(closeChan)

	// Adding a new block so we can fetch a page of two blocks, or two pages
	// with one item each.
	tx, err := createOneClientTx(b.GenesisDarc.GetBaseID(), dummyContract, b.Value, b.Signer)
	require.NoError(t, err)
	b.CTx = tx
	resp, err := b.Service().AddTransaction(&AddTxRequest{
		Version:       CurrentVersion,
		SkipchainID:   b.Genesis.SkipChainID(),
		Transaction:   tx,
		InclusionWait: 10,
	})
	transactionOK(t, resp, err)

	// Fetching two items in one page
	paginateRequest = &PaginateRequest{
		StartID:  b.Genesis.Hash,
		PageSize: 2,
		NumPages: 1,
		Backward: false,
	}
	paginateResponse, closeChan, err = service.PaginateBlocks(paginateRequest)
	require.NoError(t, err)
	var secondBlockHash skipchain.SkipBlockID

	select {
	case response := <-paginateResponse:
		if response.ErrorCode != 0 {
			t.Errorf("expected to find error code 0, but found %d, here are "+
				"the messages: %v", response.ErrorCode, response.ErrorText)
		}
		require.Equal(t, 2, len(response.Blocks))
		require.Equal(t, response.Blocks[0].Hash, b.Genesis.Hash)
		secondBlockHash = response.Blocks[1].Hash
	case <-time.After(chanTimeout):
		t.Fatal("didn't get a papginateResponse in the channel after timeout")
	}
	select {
	case <-paginateResponse:
		t.Fatal("there shouldn't be additional element in the channel")
	case <-time.After(chanTimeout):
	}

	close(closeChan)

	// Fecthing two pages with 1 item each
	paginateRequest = &PaginateRequest{
		StartID:  b.Genesis.Hash,
		PageSize: 1,
		NumPages: 2,
		Backward: false,
	}
	paginateResponse, closeChan, err = service.PaginateBlocks(paginateRequest)
	require.NoError(t, err)

	select {
	case response := <-paginateResponse:
		if response.ErrorCode != 0 {
			t.Errorf("expected to find error code 0, but found %d, here are "+
				"the messages: %v", response.ErrorCode, response.ErrorText)
		}
		require.Equal(t, 1, len(response.Blocks))
		require.Equal(t, response.Blocks[0].Hash, b.Genesis.Hash)
	case <-time.After(chanTimeout):
		t.Fatal("didn't get a papginateResponse in the channel after timeout")
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
		t.Fatal("didn't get a papginateResponse in the channel after timeout")
	}
	select {
	case <-paginateResponse:
		t.Fatal("there shouldn't be additional element in the channel")
	case <-time.After(chanTimeout):
	}

	close(closeChan)

	// If we get the page in reverse order from the genesis block we should get
	// an error
	paginateRequest = &PaginateRequest{
		StartID:  b.Genesis.Hash,
		PageSize: 2,
		NumPages: 1,
		Backward: true,
	}
	paginateResponse, closeChan, err = service.PaginateBlocks(paginateRequest)
	require.NoError(t, err)

	select {
	case response := <-paginateResponse:
		// We expect an error code 6 and not 5 because the genesis block has
		// actually a random BackLinkIDs[0] instead of none (the reason is to have
		// uniq chainID)
		require.Equal(t, response.ErrorCode, uint64(6))
		require.Equal(t, len(response.ErrorText), 7)
		require.Equal(t, response.ErrorText[3], "0")
		require.Equal(t, response.ErrorText[5], "1")
		require.Equal(t, 0, len(response.Blocks))
	case <-time.After(chanTimeout):
		t.Fatal("didn't get a papginateResponse in the channel after timeout")
	}
	select {
	case <-paginateResponse:
		t.Fatal("there shouldn't be additional element in the channel")
	case <-time.After(chanTimeout):
	}

	close(closeChan)

	// Trying to fetch 2 pages from the genesis block in reverse order should
	// raise an error in the second page
	paginateRequest = &PaginateRequest{
		StartID:  b.Genesis.Hash,
		PageSize: 1,
		NumPages: 2,
		Backward: true,
	}
	paginateResponse, closeChan, err = service.PaginateBlocks(paginateRequest)
	require.NoError(t, err)

	select {
	case response := <-paginateResponse:
		if response.ErrorCode != 0 {
			t.Errorf("expected to find error code 0, but found %d, here are "+
				"the messages: %v", response.ErrorCode, response.ErrorText)
		}
		require.Equal(t, 1, len(response.Blocks))
		require.Equal(t, response.Blocks[0].Hash, b.Genesis.Hash)
	case <-time.After(chanTimeout):
		t.Fatal("didn't get a papginateResponse in the channel after timeout")
	}
	select {
	case response := <-paginateResponse:
		require.Equal(t, response.ErrorCode, uint64(4))
		require.Equal(t, 0, len(response.Blocks))
	case <-time.After(chanTimeout):
		t.Fatal("didn't get a papginateResponse in the channel after timeout")
	}
	select {
	case <-paginateResponse:
		t.Fatal("there shouldn't be additional element in the channel")
	case <-time.After(chanTimeout):
	}

	close(closeChan)

	// Fetching two items in one page from the second block in reverse order
	// should be allright
	paginateRequest = &PaginateRequest{
		StartID:  secondBlockHash,
		PageSize: 2,
		NumPages: 1,
		Backward: true,
	}
	paginateResponse, closeChan, err = service.PaginateBlocks(paginateRequest)
	require.NoError(t, err)

	select {
	case response := <-paginateResponse:
		if response.ErrorCode != 0 {
			t.Errorf("expected to find error code 0, but found %d, here are "+
				"the messages: %v", response.ErrorCode, response.ErrorText)
		}
		require.Equal(t, 2, len(response.Blocks))
		require.Equal(t, response.Blocks[0].Hash, secondBlockHash)
		require.Equal(t, response.Blocks[1].Hash, b.Genesis.Hash)
	case <-time.After(chanTimeout):
		t.Fatal("didn't get a papginateResponse in the channel after timeout")
	}
	select {
	case <-paginateResponse:
		t.Fatal("there shouldn't be additional element in the channel")
	case <-time.After(chanTimeout):
	}

	close(closeChan)

	// Fecthing two pages with 1 item each per page from the second block should
	// also be allright
	paginateRequest = &PaginateRequest{
		StartID:  secondBlockHash,
		PageSize: 1,
		NumPages: 2,
		Backward: true,
	}
	paginateResponse, closeChan, err = service.PaginateBlocks(paginateRequest)
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
		t.Fatal("didn't get a papginateResponse in the channel after timeout")
	}
	select {
	case response := <-paginateResponse:
		if response.ErrorCode != 0 {
			t.Errorf("expected to find error code 0, but found %d, here are "+
				"the messages: %v", response.ErrorCode, response.ErrorText)
		}
		require.Equal(t, 1, len(response.Blocks))
		require.Equal(t, response.Blocks[0].Hash, b.Genesis.Hash)
	case <-time.After(chanTimeout):
		t.Fatal("didn't get a papginateResponse in the channel after timeout")
	}
	select {
	case <-paginateResponse:
		t.Fatal("there shouldn't be additional element in the channel")
	case <-time.After(chanTimeout):
	}

	close(closeChan)

	// Using a wrong page size should return an error 2
	paginateRequest = &PaginateRequest{
		StartID:  b.Genesis.Hash,
		PageSize: 0,
		NumPages: 1,
		Backward: false,
	}
	paginateResponse, closeChan, err = service.PaginateBlocks(paginateRequest)
	require.NoError(t, err)

	select {
	case response := <-paginateResponse:
		if response.ErrorCode != 2 {
			t.Errorf("expected to find error code 2, but found %d, here are "+
				"the messages: %v", response.ErrorCode, response.ErrorText)
		}
		require.Equal(t, 0, len(response.Blocks))
	case <-time.After(chanTimeout):
		t.Fatal("didn't get a papginateResponse in the channel after timeout")
	}

	select {
	case <-paginateResponse:
		t.Fatal("there shouldn't be additional element in the channel")
	case <-time.After(chanTimeout):
	}

	close(closeChan)

	// Now using a wrong num page, it should also return an error 2
	paginateRequest = &PaginateRequest{
		StartID:  secondBlockHash,
		PageSize: 1,
		NumPages: 0,
		Backward: false,
	}

	paginateResponse, closeChan, err = service.PaginateBlocks(paginateRequest)
	require.NoError(t, err)

	select {
	case response := <-paginateResponse:
		if response.ErrorCode != 2 {
			t.Errorf("expected to find error code 2, but found %d, here are "+
				"the messages: %v", response.ErrorCode, response.ErrorText)
		}
		require.Equal(t, 0, len(response.Blocks))
	case <-time.After(chanTimeout):
		t.Fatal("didn't get a papginateResponse in the channel after timeout")
	}

	select {
	case <-paginateResponse:
		t.Fatal("there shouldn't be additional element in the channel")
	case <-time.After(chanTimeout):
	}

	close(closeChan)
}
