package ots

import (
	"go.dedis.ch/onet/v3/log"
	"golang.org/x/xerrors"
	"sync"
)

var storageKey = []byte("storage")

type storage struct {
	sync.Mutex
}

// saves all data.
func (s *Service) save() error {
	s.storage.Lock()
	defer s.storage.Unlock()
	err := s.Save(storageKey, s.storage)
	if err != nil {
		log.Error("Couldn't save data:", err)
		return xerrors.Errorf("saving data: %v", err)
	}
	return nil
}

// Tries to load the configuration and updates the data in the service
// if it finds a valid config-file.
func (s *Service) tryLoad() error {
	s.storage = &storage{}

	// Make sure we don't have any unallocated maps.
	//defer func() {
	//	if len(s.storage.Shares) == 0 {
	//		s.storage.Shares = make(map[string]*share.PriShare)
	//	}
	//}()

	msg, err := s.Load(storageKey)
	if err != nil {
		return xerrors.Errorf("loading storage: %v", err)
	}
	if msg == nil {
		return nil
	}
	var ok bool
	s.storage, ok = msg.(*storage)
	if !ok {
		return xerrors.New("data of wrong type")
	}
	return nil
}
