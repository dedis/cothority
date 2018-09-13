package personhood

import (
	"sync"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/byzcoin"
	"github.com/dedis/onet/network"
	"github.com/dedis/protobuf"
)

const dbVersion = 1

var storageKey = []byte("storage")

func init() {
	network.RegisterMessage(&storage1{})
}

// saves all data.
func (s *Service) save() error {
	s.storage.Lock()
	defer s.storage.Unlock()
	return s.Save(storageKey, s.storage)
}

// Tries to load the configuration and updates the data in the service
// if it finds a valid config-file.
func (s *Service) tryLoad() error {
	s.storage = &storage1{}
	ver, err := s.LoadVersion()
	if err != nil {
		return err
	}
	if ver < dbVersion {
		// There is no version 0. Save empty storage and update version number.
		if err = s.save(); err != nil {
			return err
		}
		return s.SaveVersion(dbVersion)
	}
	buf, err := s.LoadRaw(storageKey)
	if err != nil {
		return err
	}
	return protobuf.DecodeWithConstructors(buf[16:], s.storage,
		network.DefaultConstructors(cothority.Suite))
}

type storage1 struct {
	Messages       map[string]*Message
	Read           map[string]*readMsg
	Questionnaires map[string]*Questionnaire
	Replies        map[string]*Reply
	Parties        map[string]*Party

	sync.Mutex
}

type readMsg struct {
	Readers []byzcoin.InstanceID
}
