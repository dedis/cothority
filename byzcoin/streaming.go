package byzcoin

import (
	"sync"

	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/onet/v3/network"
)

func init() {
	network.RegisterMessages(&StreamingRequest{}, &StreamingResponse{})
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

func (s *streamingManager) newListener(scID string) (chan *StreamingResponse, int) {
	s.Lock()
	defer s.Unlock()

	if s.listeners == nil {
		s.listeners = make(map[string][]chan *StreamingResponse)
	}

	ls := s.listeners[scID]
	id := len(s.listeners)
	outChan := make(chan *StreamingResponse)
	ls = append(ls, outChan)
	s.listeners[scID] = ls
	return outChan, id
}

func (s *streamingManager) stopListener(scID string, i int) {
	s.Lock()
	defer s.Unlock()

	ls, ok := s.listeners[scID]
	if !ok || i >= len(ls) {
		panic("listener does not exist")
	}

	close(ls[i])

	ls = append(ls[:i], ls[i+1:]...)
	s.listeners[scID] = ls
}

// StreamTransactions will stream all transactions IDs to the client until the
// client closes the connection.
func (s *Service) StreamTransactions(msg *StreamingRequest) (chan *StreamingResponse, chan bool, error) {
	stopChan := make(chan bool)
	key := string(msg.ID)
	outChan, idx := s.streamingMan.newListener(key)
	go func() {
		<-stopChan
		s.streamingMan.stopListener(key, idx)
	}()
	return outChan, stopChan, nil
}
