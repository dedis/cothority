package connectivity

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
	"golang.org/x/xerrors"
)

var serviceID onet.ServiceID

func init() {
	var err error
	serviceID, err = onet.RegisterNewService(Name, newService)
	log.ErrFatal(err)
	network.RegisterMessage(&storage{})
}

// Service is our template-service
type Service struct {
	// We need to embed the ServiceProcessor, so that incoming messages
	// are correctly handled.
	*onet.ServiceProcessor

	storage *storage
}

// storageID reflects the data we're storing - we could store more
// than one structure.
var storageID = []byte("main")

// storage is used to save our data.
type storage struct {
	sync.Mutex
	ConnectivityMatrix
	// CacheExpiryDuration stores the cache expiry interval
	CacheExpiryDuration time.Duration
	// CacheExpiresAt stores the expiry time of the cache
	cacheExpiresAt time.Time
}

func (s *Service) NewProtocol(tn *onet.TreeNodeInstance, conf *onet.GenericConfig) (onet.ProtocolInstance, error) {
	log.Lvl3("Not templated yet")
	return nil, nil
}

// saves all data.
func (s *Service) save() {
	s.storage.Lock()
	defer s.storage.Unlock()
	err := s.Save(storageID, s.storage)
	if err != nil {
		log.Error("Couldn't save data:", err)
	}
}

// Tries to load the configuration and updates the data in the service
// if it finds a valid config-file.
func (s *Service) tryLoad() error {
	s.storage = &storage{}
	msg, err := s.Load(storageID)
	if err != nil {
		return err
	}
	if msg == nil {
		return nil
	}
	var ok bool
	s.storage, ok = msg.(*storage)
	if !ok {
		return errors.New("Data of wrong type")
	}
	return nil
}

// newService receives the context that holds information about the node it's
// running on. Saving and loading can be done using the context. The data will
// be stored in memory for tests and simulations, and on disk for real deployments.
func newService(c *onet.Context) (onet.Service, error) {
	connectivityTTL := os.Getenv("CONNECTIVITY_TTL")
	var cacheExpiryDuration time.Duration
	if connectivityTTL == "" {
		cacheExpiryDuration = 5 * time.Minute
	} else {
		ttl, err := strconv.Atoi(connectivityTTL)
		if err != nil {
			return nil, xerrors.Errorf("error parsing $CONNECTIVITY_TTL: %v", err)
		}
		cacheExpiryDuration = time.Duration(ttl) * time.Second
	}

	s := &Service{
		ServiceProcessor: onet.NewServiceProcessor(c),
	}
	if err := s.RegisterHandlers(s.Check); err != nil {
		return nil, errors.New("Couldn't register messages")
	}
	if err := s.tryLoad(); err != nil {
		log.Error(err)
		return nil, err
	}
	s.storage.cacheExpiresAt = time.Now()
	s.storage.CacheExpiryDuration = cacheExpiryDuration
	return s, nil
}

func (s *Service) Check(req *CheckRequest) (*CheckReply, error) {
	s.storage.Lock()
	defer s.storage.Unlock()

	if time.Now().Before(s.storage.cacheExpiresAt) {
		return &CheckReply{
			ConnectivityMatrix: s.storage.ConnectivityMatrix,
		}, nil
	}

	tree := req.Roster.GenerateNaryTreeWithRoot(len(req.Roster.List)-1, s.ServerIdentity())
	if tree == nil {
		return nil, xerrors.Errorf("couldn't generate tree")
	}

	pi, err := s.CreateProtocol(Name, tree)
	if err != nil {
		return nil, xerrors.Errorf("couldn't create protocol: %v", err)
	}

	probeTime := time.Now()
	if s.storage.ConnectivityMatrix.Status == nil {
		s.storage.ConnectivityMatrix.Status = make(map[string]*state)
	}
	matrix := s.storage.ConnectivityMatrix.Status
	for _, node := range req.Roster.List {
		if _, ok := matrix[node.String()]; !ok {
			matrix[node.String()] = &state{}
		}
	}
	s.storage.ConnectivityMatrix.LastCheckedAt = probeTime

	pi.Start()

	for err := range pi.(*ConnectivityProtocol).failure {
		// TODO: depends on the error returned from TreeNodeInstance.SendToAllChildren
		name := strings.Split(err.Error(), ": ")[0]
		matrix[name].Down = true
		matrix[name].LastErrorAt = probeTime.Unix()
	}

	s.storage.cacheExpiresAt = time.Now().Add(s.storage.CacheExpiryDuration)

	// TODO: Can we prevent a copy here?
	return &CheckReply{
		ConnectivityMatrix: s.storage.ConnectivityMatrix,
	}, nil
}
