package personhood

import (
	"sync"

	"go.dedis.ch/cothority/v3/personhood/contracts"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"

	"go.dedis.ch/protobuf"
	"golang.org/x/xerrors"
)

const dbVersion = 2

var storageKey = []byte("storage")

func init() {
	network.RegisterMessage(&storage1{})
	network.RegisterMessage(&storage2{})
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
	s.storage = &storage2{}
	curVersion, err := s.LoadVersion()
	if err != nil {
		return err
	}
	buf, err := s.LoadRaw(storageKey)
	if err != nil {
		return err
	}
	switch curVersion {
	case 0:
		// There is no version 0. Save empty storage and update version number.
		if err = s.save(); err != nil {
			return err
		}
		return s.SaveVersion(dbVersion)
	case 1:
		log.Info("Migrating personhood-database from version 1 to 2")
		var s1 storage1
		err = protobuf.DecodeWithConstructors(buf[16:], &s1,
			network.DefaultConstructors(cothority.Suite))
		s.storage.RoPaSci = s1.RoPaSci
		s.storage.Parties = s1.Parties
		s.storage.Polls = s1.Polls
		return s.SaveVersion(dbVersion)
	case 2:
		return protobuf.DecodeWithConstructors(buf[16:], s.storage,
			network.DefaultConstructors(cothority.Suite))
	default:
		return xerrors.New("unknown version")
	}
}

type storage1 struct {
	RoPaSci   []*contracts.RoPaSci
	Parties   map[string]*Party
	Polls     map[string]*storagePolls
	Challenge map[string]*ChallengeCandidate

	sync.Mutex
}

type storage2 struct {
	RoPaSci      []*contracts.RoPaSci
	Parties      map[string]*Party
	Polls        map[string]*storagePolls
	Challenge    map[string]*ChallengeCandidate
	AdminDarcIDs []darc.ID

	sync.Mutex
}

type storagePolls struct {
	Polls []*PollStruct
}
