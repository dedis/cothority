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
