package byzcoin

import (
	"fmt"
	"sync"

	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/onet/v3/network"
)

const (
	// PaginateWrongInput is used when an invalid parameter is given in the
	// paginate request
	PaginateWrongInput = 2
	// PaginateWrongStart is used when the startID is invalid
	PaginateWrongStart = 3
	// PaginatePageFailed is used when the first block of the page failed to be
	// retreived
	PaginatePageFailed = 4
	// PaginateLinkMissing is used when the next (or previous if backward is
	// true) block does not exist
	PaginateLinkMissing = 5
	// PaginateGetBlockFailed is used when it coulnd't get a next or previous block
	PaginateGetBlockFailed = 6
)

func init() {
	network.RegisterMessages(&StreamingRequest{}, &StreamingResponse{},
		&PaginateRequest{}, &PaginateResponse{})
}

type streamingManager struct {
	sync.Mutex
	// key: skipchain ID, value: slice of listeners
	listeners map[string][]chan *StreamingResponse
}

func (s *streamingManager) notify(scID string, block *skipchain.SkipBlock) {
	s.Lock()
	defer s.Unlock()

	ls, ok := s.listeners[scID]
	if !ok {
		return
	}

	for _, c := range ls {
		c <- &StreamingResponse{
			Block: block,
		}
	}
}

func (s *streamingManager) newListener(scID string) chan *StreamingResponse {
	s.Lock()
	defer s.Unlock()

	if s.listeners == nil {
		s.listeners = make(map[string][]chan *StreamingResponse)
	}

	ls := s.listeners[scID]
	outChan := make(chan *StreamingResponse)
	ls = append(ls, outChan)
	s.listeners[scID] = ls
	return outChan
}

func (s *streamingManager) stopListener(scID string, outChan chan *StreamingResponse) {
	s.Lock()
	defer s.Unlock()

	ls := s.listeners[scID]
	if ls == nil {
		return
	}

	for i, listener := range ls {
		if listener == outChan {
			close(listener)
			s.listeners[scID] = append(ls[:i], ls[i+1:]...)
			return
		}
	}
}

func (s *streamingManager) stopAll() {
	s.Lock()
	defer s.Unlock()

	for key, l := range s.listeners {
		for _, c := range l {
			// Force the streaming connection in Onet to close.
			close(c)
		}

		delete(s.listeners, key)
	}
}

// StreamTransactions will stream all transactions IDs to the client until the
// client closes the connection.
func (s *Service) StreamTransactions(msg *StreamingRequest) (chan *StreamingResponse, chan bool, error) {
	stopChan := make(chan bool)
	key := string(msg.ID)
	outChan := s.streamingMan.newListener(key)

	go func() {
		s.closedMutex.Lock()
		if s.closed {
			s.closedMutex.Unlock()
			return
		}
		s.working.Add(1)
		defer s.working.Done()
		s.closedMutex.Unlock()

		// Either the service is closing and we force the connection to stop or
		// the streaming connection is closed upfront.
		<-stopChan
		// In both cases we clean the listener.
		s.streamingMan.stopListener(key, outChan)
	}()
	return outChan, stopChan, nil
}

// PaginateBlocks return blocks with pagination, ie. N asynchounous requests
// that contain each K consecutive block. The caller is responsible for closing
// the close chan when the caller wants to close the connection.
func (s *Service) PaginateBlocks(msg *PaginateRequest) (chan *PaginateResponse, chan bool, error) {

	outChan := make(chan *PaginateResponse)
	stopChan := make(chan bool)

	go func() {

		if msg.PageSize < 1 {
			outChan <- &PaginateResponse{
				ErrorCode: PaginateWrongInput,
				ErrorText: []string{fmt.Sprintf("PageSize should be >= 1, "+
					"but we found %d", msg.PageSize)},
			}
			return
		}

		if msg.NumPages < 1 {
			outChan <- &PaginateResponse{
				ErrorCode: PaginateWrongInput,
				ErrorText: []string{fmt.Sprintf("NumPages should be >= 1, "+
					"but we found %d", msg.NumPages)},
			}
			return
		}

		if msg.StartID == nil {
			outChan <- &PaginateResponse{
				ErrorCode: PaginateWrongStart,
				ErrorText: []string{"StartID is nil"},
			}
			return
		}

		nextID := msg.StartID

		for pageNum := uint64(0); pageNum < msg.NumPages; pageNum++ {
			_, skipBlock, err := s.getBlockTx(nextID)

			blocks := make([]*skipchain.SkipBlock, msg.PageSize)
			if err != nil {
				outChan <- &PaginateResponse{
					ErrorCode: PaginatePageFailed,
					ErrorText: []string{"failed to get the first block with ID",
						fmt.Sprintf("%x", msg.StartID), fmt.Sprintf("%v", err)},
				}
				return
			}
			blocks[0] = skipBlock

			if msg.Backward {
				if len(skipBlock.BackLinkIDs) != 0 {
					nextID = skipBlock.BackLinkIDs[0]
				} else {
					nextID = nil
				}
			} else {
				if len(skipBlock.ForwardLink) != 0 {
					nextID = skipBlock.ForwardLink[0].To
				} else {
					nextID = nil
				}
			}

			for i := uint64(1); i < msg.PageSize; i++ {

				if nextID == nil {
					outChan <- &PaginateResponse{
						ErrorCode: PaginateLinkMissing,
						ErrorText: []string{"couldn't find a nextID for block",
							fmt.Sprintf("%x", skipBlock.Hash), "page number",
							fmt.Sprintf("%d", pageNum), "index", fmt.Sprintf("%d", i)},
					}
					return
				}

				_, skipBlock, err = s.getBlockTx(nextID)
				if err != nil {
					outChan <- &PaginateResponse{
						ErrorCode: PaginateGetBlockFailed,
						ErrorText: []string{"failed to get block with ID",
							fmt.Sprintf("%x", nextID), "page number",
							fmt.Sprintf("%d", pageNum), "index",
							fmt.Sprintf("%d", i), fmt.Sprintf("%v", err)},
					}
					return
				}
				blocks[i] = skipBlock

				if msg.Backward {
					if len(skipBlock.BackLinkIDs) != 0 {
						nextID = skipBlock.BackLinkIDs[0]
					} else {
						nextID = nil
					}
				} else {
					if len(skipBlock.ForwardLink) != 0 {
						nextID = skipBlock.ForwardLink[0].To
					} else {
						nextID = nil
					}
				}
			}
			response := &PaginateResponse{
				Blocks:     blocks,
				PageNumber: pageNum,
				Backward:   msg.Backward,
			}
			// Allows the service to exit prematurely if the connection stops
			select {
			case <-stopChan:
				return
			default:
				outChan <- response
			}
		}
		// Waiting for the streaming connection to stop. This signal comes
		// from onet, which sets it when the client closes the connection.
		<-stopChan
	}()

	return outChan, stopChan, nil
}
