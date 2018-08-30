package service

/*
The service.go defines what to do for each API-call. This part of the service
runs on the node.
*/

import (
	"encoding/binary"
	"errors"
	"sort"
	"sync"

	ol "github.com/dedis/cothority/omniledger/service"
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
	Messages       map[string]*Message
	Read           map[string]*readMsg
	Questionnaires map[string]*Questionnaire
	Replies        map[string]*Reply
	Parties        map[string]*Party

	sync.Mutex
}

type readMsg struct {
	Readers []ol.InstanceID
}

// LinkPoP stores a link to a pop-party to accept this configuration. It will
// try to create an account to receive payments from clients.
func (s *Service) LinkPoP(lp *LinkPoP) (*StringReply, error) {
	s.storage.Parties[string(lp.PopInstance.Slice())] = &lp.Party
	s.save()
	return nil, nil
}

func (s *Service) GetAccount(ga *GetAccount) (*GetAccountReply, error) {
	party, ok := s.storage.Parties[string(ga.PopInstance.Slice())]
	if !ok {
		return nil, errors.New("this party doesn't exist")
	}
	return &GetAccountReply{party.Account}, nil
}

// RegisterQuestionnaire creates a questionnaire with a number of questions to
// chose from and how much each replier gets rewarded.
func (s *Service) RegisterQuestionnaire(rq *RegisterQuestionnaire) (*StringReply, error) {
	idStr := string(rq.Questionnaire.ID)
	s.storage.Questionnaires[idStr] = &rq.Questionnaire
	s.storage.Replies[idStr] = &Reply{}
	return nil, nil
}

func (s *Service) ListQuestionnaires(lq *ListQuestionnaires) (*ListQuestionnairesReply, error) {
	var qreply []Questionnaire
	for _, q := range s.storage.Questionnaires {
		qreply = append(qreply, *q)
	}
	sort.Slice(qreply, func(i, j int) bool {
		return qreply[i].Balance > qreply[j].Balance
	})
	if len(qreply) < lq.Start {
		return &ListQuestionnairesReply{}, nil
	}
	qreply = qreply[lq.Start:]
	if len(qreply) > lq.Number {
		qreply = qreply[:lq.Number]
	}
	for i, q := range qreply {
		if q.Balance == 0 {
			qreply = qreply[:i]
			break
		}
	}
	return &ListQuestionnairesReply{qreply}, nil
}

func (s *Service) AnswerQuestionnaire(aq *AnswerQuestionnaire) (*StringReply, error) {
	q := s.storage.Questionnaires[string(aq.ID)]
	if q == nil {
		return nil, errors.New("didn't find questionnaire")
	}
	if len(aq.Replies) > q.Replies {
		return nil, errors.New("too many replies")
	}
	for _, r := range aq.Replies {
		if r >= len(q.Questions) || r < 0 {
			return nil, errors.New("reply out of bound")
		}
	}
	if q.Balance < q.Reward {
		return nil, errors.New("no reward left")
	}
	r := s.storage.Replies[string(q.ID)]
	if r == nil {
		r = &Reply{}
		s.storage.Replies[string(q.ID)] = r
	} else {
		for _, u := range r.Users {
			if u.Equal(aq.Account) {
				return nil, errors.New("cannot answer more than once")
			}
		}
	}
	q.Balance -= q.Reward
	r.Users = append(r.Users, aq.Account)
	// TODO: send reard to account

	return nil, nil
}

func (s *Service) TopupQuestionnaire(tq *TopupQuestionnaire) (*StringReply, error) {
	return nil, errors.New("not implemented yet")
}

func (s *Service) SendMessage(sm *SendMessage) (*StringReply, error) {
	idStr := string(sm.Message.ID)
	if msg := s.storage.Messages[idStr]; msg != nil {
		return nil, errors.New("this message-ID already exists")
	}
	s.storage.Messages[idStr] = &sm.Message
	s.storage.Read[idStr] = &readMsg{}
	return nil, nil
}

func (s *Service) ListMessages(lm *ListMessages) (*ListMessagesReply, error) {
	var mreply []Message
	for _, q := range s.storage.Messages {
		mreply = append(mreply, *q)
	}
	sort.Slice(mreply, func(i, j int) bool {
		return mreply[i].Balance > mreply[j].Balance
	})
	if len(mreply) < lm.Start {
		return &ListMessagesReply{}, nil
	}
	mreply = mreply[lm.Start:]
	if len(mreply) > lm.Number {
		mreply = mreply[:lm.Number]
	}
	for i, q := range mreply {
		if q.Balance == 0 {
			mreply = mreply[:i]
			break
		}
	}
	lmr := &ListMessagesReply{}
	for _, msg := range mreply {
		lmr.IDs = append(lmr.IDs, msg.ID)
		lmr.Subjects = append(lmr.Subjects, msg.Subject)
	}
	return lmr, nil
}

func (s *Service) ReadMessage(rm *ReadMessage) (*ReadMessageReply, error) {
	msg := s.storage.Messages[string(rm.ID)]
	if msg == nil {
		return nil, errors.New("no such subject")
	}
	party := s.storage.Parties[string(rm.Party)]
	if party == nil {
		return nil, errors.New("no such party")
	}
	if msg.Balance < msg.Reward {
		return &ReadMessageReply{*msg}, nil
	}
	read := s.storage.Read[string(msg.ID)]
	for _, reader := range read.Readers {
		if reader.Equal(rm.Reader) {
			return &ReadMessageReply{*msg}, nil
		}
	}
	msg.Balance -= msg.Reward
	read.Readers = append(read.Readers, rm.Reader)

	cBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(cBuf, msg.Reward)
	ctx := ol.ClientTransaction{
		Instructions: []ol.Instruction{{
			InstanceID: party.Account,
			Index:      0,
			Length:     1,
			Invoke: &ol.Invoke{
				Command: "transfer",
				Args: []ol.Argument{{
					Name:  "coins",
					Value: cBuf,
				},
					{
						Name:  "destination",
						Value: rm.Reader.Slice(),
					}},
			},
		}},
	}
	err := ctx.Instructions[0].SignBy(party.Darc.GetBaseID(), party.Signer)
	if err != nil {
		return nil, errors.New("couldn't sign: " + err.Error())
	}
	cl := ol.NewClient()
	cl.Roster = *party.FinalStatement.Desc.Roster
	cl.ID = party.OmniLedgerID
	_, err = cl.AddTransactionAndWait(ctx, 10)
	if err != nil {
		return nil, errors.New("couldn't send reward: " + err.Error())
	}

	return &ReadMessageReply{*msg}, nil
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
	if err := s.RegisterHandlers(s.AnswerQuestionnaire, s.LinkPoP, s.ListMessages,
		s.ListQuestionnaires, s.ReadMessage, s.RegisterQuestionnaire, s.SendMessage,
		s.TopupQuestionnaire, s.TopupMessage); err != nil {
		return nil, errors.New("Couldn't register messages")
	}
	if err := s.tryLoad(); err != nil {
		log.Error(err)
		return nil, err
	}
	if len(s.storage.Messages) == 0 {
		s.storage.Messages = make(map[string]*Message)
	}
	if len(s.storage.Questionnaires) == 0 {
		s.storage.Questionnaires = make(map[string]*Questionnaire)
	}
	if len(s.storage.Parties) == 0 {
		s.storage.Parties = make(map[string]*Party)
	}
	if len(s.storage.Replies) == 0 {
		s.storage.Replies = make(map[string]*Reply)
	}
	if len(s.storage.Read) == 0 {
		s.storage.Read = make(map[string]*readMsg)
	}
	return s, nil
}
