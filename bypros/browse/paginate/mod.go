package paginate

import (
	"context"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"go.dedis.ch/cothority/v3/bypros/browse"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/protobuf"
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
	wsAddr string) browse.Actor {

	return &Paginate{
		wsAddr:  wsAddr,
		handler: handler,

		pageSize: p.pageSize,
		numPages: p.numPages,
	}
}

// Paginate defines a browse actor based on paginate
//
// - implements browse.Actor
type Paginate struct {
	wsAddr  string
	handler browse.Handler

	pageSize int
	numPages int
}

// Browse implements browse.Actor
func (p *Paginate) Browse(ctx context.Context, fromBlock skipchain.SkipBlockID) error {

	ws, _, err := websocket.DefaultDialer.Dial(p.wsAddr+"/ByzCoin/PaginateRequest", nil)
	if err != nil {
		return xerrors.Errorf("failed to open ws connection: %v", err)
	}

	defer func() {
		err = ws.WriteControl(websocket.CloseMessage, nil, time.Now().Add(time.Second*5))
		if err != nil {
			log.Warnf("failed to send close: %v", err)
		}

		ws.Close()
	}()

	err = p.paginate(ctx, p.pageSize, p.numPages, fromBlock, ws)
	if err != nil {
		return xerrors.Errorf("failed to paginate: %v", err)
	}

	return nil
}

// paginate starts the pagination process.
func (p *Paginate) paginate(ctx context.Context, pageSize, numPages int,
	nextBlock skipchain.SkipBlockID, ws *websocket.Conn) error {

	for {
		paginateRequest := byzcoin.PaginateRequest{
			StartID:  nextBlock,
			PageSize: uint64(pageSize),
			NumPages: uint64(numPages),
			Backward: false,
		}

		buf, err := protobuf.Encode(&paginateRequest)
		if err != nil {
			return xerrors.Errorf("failed to encode request: %v", err)
		}

		err = ws.WriteMessage(websocket.BinaryMessage, buf)
		if err != nil {
			return xerrors.Errorf("failed to send request: %v", err)
		}

		nextBlock, err = p.handlePages(ctx, numPages, ws, nextBlock)

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
func (p *Paginate) handlePages(ctx context.Context, numPages int, ws *websocket.Conn,
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

		_, buf, err := ws.ReadMessage()
		if err != nil {
			return nil, xerrors.Errorf("failed to read response: %v", err)
		}

		err = protobuf.Decode(buf, &paginateResponse)
		if err != nil {
			return nil, xerrors.Errorf("failed to decode response: %v", err)
		}

		// The first block of the page couldn't be fetched. That means we're
		// at then end. It could also mean the very first block we're trying
		// to get doesn't exist.
		if paginateResponse.ErrorCode == byzcoin.PaginatePageFailed {
			log.LLvl1("done with the chain")
			return nil, nil
		}

		if paginateResponse.ErrorCode == byzcoin.PaginateLinkMissing {
			// We couldn't find a next block. That means we're at end of
			// chain. We should load blocks that are left in the page.

			// part of the streaming API, the error text contains the number of
			// blocks left in the page.
			index, err := strconv.Atoi(paginateResponse.ErrorText[5])
			if err != nil {
				return nil, xerrors.Errorf("failed to read paginate response: %v", err)
			}

			log.LLvlf1("reached the end, loading remaining %d blocks", index)

			if block == nil {
				// this is a special case where the first block of the first
				// page is the last block on the chain.
				err = p.paginate(ctx, index, 1, firstBlock, ws)
			} else {
				err = p.paginate(ctx, index, 1, block.Hash, ws)
			}
			if err != nil {
				return nil, xerrors.Errorf("failed to load last blocks: %v", err)
			}
			return nil, nil
		}

		if paginateResponse.ErrorCode == byzcoin.PaginateGetBlockFailed {
			// we couldn't get that block, that's bad
			return nil, xerrors.Errorf("failed to read paginate response: %v", err)
		}

		for _, block = range paginateResponse.Blocks {
			err := p.handler(block)
			if err != nil {
				return nil, xerrors.Errorf("handler failed: %v", err)
			}
		}
	}

	if len(block.ForwardLink) == 0 {
		log.LLvl1("we're at the end of the chain")
		return nil, nil
	}

	return block.ForwardLink[0].To, nil
}
