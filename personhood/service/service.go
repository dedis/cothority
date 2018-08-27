package service

/*
The service.go defines what to do for each API-call. This part of the service
runs on the node.
*/

import (
	"errors"
	"sync"

	template "github.com/dedis/cothority_template"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
)

// Used for tests
var templateID onet.ServiceID

func init() {
	var err error
	templateID, err = onet.RegisterNewService(template.ServiceName, newService)
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
	Messages       []Message
	Questionnaires []Questionnaire
	sync.Mutex
}

// LinkPop stores a link to a pop-party to accept this configuration. It will
// try to create an account to receive payments from clients.
func (s *Service) LinkPop(lp *LinkPop) (*StringReply, error) {
	return nil, errors.New("not implemented yet")
}

func (s *Service) GetAccount(ga *GetAccount) (*GetAccountReply, error) {
	return nil, errors.New("not implemented yet")
}

// RegisterQuestionnaire creates a questionnaire with a number of questions to
// chose from and how much each replier gets rewarded.
func (s *Service) RegisterQuestionnaire(rq *RegisterQuestionnaire) (*StringReply, error) {
	return nil, errors.New("not implemented yet")
}

func (s *Service) ListQuestionnaires(lq *ListQuestionnaires) (*ListQuestionnairesReply, error) {
	return nil, errors.New("not implemented yet")
}

func (s *Service) AnswerQuestionnaire(aq *AnswerQuestionnaire) (*StringReply, error) {
	return nil, errors.New("not implemented yet")
}

func (s *Service) TopupQuestionnaire(tq *TopupQuestionnaire) (*StringReply, error) {
	return nil, errors.New("not implemented yet")
}

func (s *Service) SendMessage(sm *SendMessage) (*StringReply, error) {
	return nil, errors.New("not implemented yet")
}

func (s *Service) ListMessages(lm *ListMessages) (*ListMessagesReply, error) {
	return nil, errors.New("not implemented yet")
}
func (s *Service) ReadMessage(rm *ReadMessage) (*ReadMessageReply, error) {
	return nil, errors.New("not implemented yet")
}

func (s *Service) TopupMessage(tm *TopupMessage) (*StringReply, error) {
	return nil, errors.New("not implemented yet")
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

func newService(c *onet.Context) (onet.Service, error) {
	s := &Service{
		ServiceProcessor: onet.NewServiceProcessor(c),
	}
	if err := s.RegisterHandlers(s.AnswerQuestionnaire, s.LinkPop, s.ListMessages,
		s.ListQuestionnaires, s.ReadMessage, s.RegisterQuestionnaire, s.SendMessage,
		s.TopupQuestionnaire, s.TopupMessage); err != nil {
		return nil, errors.New("Couldn't register messages")
	}
	if err := s.tryLoad(); err != nil {
		log.Error(err)
		return nil, err
	}
	return s, nil
}
