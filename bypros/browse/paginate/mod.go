package paginate

import (
	"context"
	"strconv"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/bypros/browse"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
	"golang.org/x/xerrors"
)

// NewService returns a new browsing service based on paginate.
func NewService(pageSize, numPages int) browse.Service {
	return Service{
		pageSize: pageSize,
		numPages: numPages,
	}
}

// Service is a browsing service based on paginate.
//
// - implements browse.Service
type Service struct {
	pageSize int
	numPages int
}

// GetBrowser implements browse.Service.
func (p Service) GetBrowser(handler browse.Handler, id skipchain.SkipBlockID,
	target *network.ServerIdentity) browse.Actor {

	return &Paginate{
		target:  target,
		handler: handler,

		pageSize: p.pageSize,
		numPages: p.numPages,
	}
}

// Paginate defines a browse actor based on paginate
//
// - implements browse.Actor
type Paginate struct {
	target  *network.ServerIdentity
	handler browse.Handler

	pageSize int
	numPages int
}

// Browse implements browse.Actor
func (p *Paginate) Browse(ctx context.Context, fromBlock skipchain.SkipBlockID) error {

	client := onet.NewClientKeep(cothority.Suite, byzcoin.ServiceName)
	defer client.Close()

	wire := &wire{
		client: client,
	}

	err := p.paginate(ctx, p.pageSize, p.numPages, fromBlock, wire)
	if err != nil {
		return xerrors.Errorf("failed to paginate: %v", err)
	}

	return nil
}

// paginate starts the pagination process.
func (p *Paginate) paginate(ctx context.Context, pageSize, numPages int,
	nextBlock skipchain.SkipBlockID, wire *wire) error {

	for {
		paginateRequest := byzcoin.PaginateRequest{
			StartID:  nextBlock,
			PageSize: uint64(pageSize),
			NumPages: uint64(numPages),
			Backward: false,
		}

		conn, err := wire.client.Stream(p.target, &paginateRequest)
		if err != nil {
			return xerrors.Errorf("failed to send stream: %v", err)
		}

		wire.conn = conn

		nextBlock, err = p.handlePages(ctx, numPages, wire, nextBlock)

		if err != nil {
			return xerrors.Errorf("failed to handle page: %v", err)
		}

		if nextBlock == nil {
			return nil
		}
	}
}

// handlePages retrieves the pages from the client and returns the next block ID
// that should be loaded. If nil, that means there are no blocks left to be
// loaded.
func (p *Paginate) handlePages(ctx context.Context, numPages int, wire *wire,
	firstBlock skipchain.SkipBlockID) (skipchain.SkipBlockID, error) {

	var block *skipchain.SkipBlock

	for j := 0; j < numPages; j++ {
		// for each page we check if we still need to continue
		select {
		case <-ctx.Done():
			return nil, nil
		default:
		}

		paginateResponse := byzcoin.PaginateResponse{}
		err := wire.conn.ReadMessage(&paginateResponse)
		if err != nil {
			return nil, xerrors.Errorf("failed to read paginate response: %v", err)
		}

		done, err := p.checkError(ctx, paginateResponse, block, wire, firstBlock)
		if err != nil {
			return nil, xerrors.Errorf("failed to check error: %v", err)
		}

		if done {
			return nil, nil
		}

		for _, block = range paginateResponse.Blocks {
			err := p.handler(block)
			if err != nil {
				return nil, xerrors.Errorf("handler failed: %v", err)
			}
		}
	}

	if len(block.ForwardLink) == 0 {
		log.Lvl1("we're at the end of the chain")
		return nil, nil
	}

	return block.ForwardLink[0].To, nil
}

// checkError checks the paginate response for error. In case we reach the end
// of the chain, it makes the subsequent call the fetch the remaining blocks.
// Return if the pagination is done or not.
func (p *Paginate) checkError(ctx context.Context, resp byzcoin.PaginateResponse,
	block *skipchain.SkipBlock, streaming *wire, firstBlock skipchain.SkipBlockID) (bool, error) {

	// The first block of the page couldn't be fetched. That means we're
	// at then end. It could also mean the very first block we're trying
	// to get doesn't exist.
	if resp.ErrorCode == byzcoin.PaginatePageFailed {
		log.Lvl1("done with the chain")
		return true, nil
	}

	if resp.ErrorCode == byzcoin.PaginateLinkMissing {
		// We couldn't find a next block. That means we're at end of
		// chain. We should load blocks that are left in the page.

		// part of the streaming API, the error text contains the number of
		// blocks left in the page.
		index, err := strconv.Atoi(resp.ErrorText[5])
		if err != nil {
			return false, xerrors.Errorf("failed to read paginate response: %v", err)
		}

		log.Lvlf1("reached the end, loading remaining %d blocks", index)

		if block == nil {
			// this is a special case where the first block of the first
			// page is the last block on the chain.
			err = p.paginate(ctx, index, 1, firstBlock, streaming)
		} else {
			err = p.paginate(ctx, index, 1, block.Hash, streaming)
		}

		if err != nil {
			return false, xerrors.Errorf("failed to load last blocks: %v", err)
		}

		return true, nil
	}

	if resp.ErrorCode == byzcoin.PaginateGetBlockFailed {
		// we couldn't get that block, that's bad
		return false, xerrors.Errorf("failed to read paginate response: %v", resp.ErrorText)
	}

	return false, nil
}

// wire bundles the read/write capabilities of the streaming onet
type wire struct {
	client *onet.Client
	conn   onet.StreamingConn
}
