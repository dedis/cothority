package personhood

/*
The service.go defines what to do for each API-call. This part of the service
runs on the node.
*/

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"sort"

	"github.com/dedis/cothority/byzcoin"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
)

// Used for tests
var templateID onet.ServiceID

// ServiceName of the personhood service
var ServiceName = "Personhood"

func init() {
	var err error
	templateID, err = onet.RegisterNewService(ServiceName, newService)
	log.ErrFatal(err)
}

// Service is our template-service
type Service struct {
	// We need to embed the ServiceProcessor, so that incoming messages
	// are correctly handled.
	*onet.ServiceProcessor

	storage *storage1
}

// LinkPoP stores a link to a pop-party to accept this configuration. It will
// try to create an account to receive payments from clients.
func (s *Service) LinkPoP(lp *LinkPoP) (*StringReply, error) {
	log.Lvlf2("%s: Linking pop: %+v", s.ServerIdentity(), lp)
	s.storage.Parties[string(lp.Party.InstanceID.Slice())] = &lp.Party
	s.save()
	log.Lvlf2("%s: parties: %+v", s.ServerIdentity(), s.storage.Parties)
	return &StringReply{}, nil
}

// RegisterQuestionnaire creates a questionnaire with a number of questions to
// chose from and how much each replier gets rewarded.
func (s *Service) RegisterQuestionnaire(rq *RegisterQuestionnaire) (*StringReply, error) {
	idStr := string(rq.Questionnaire.ID)
	s.storage.Questionnaires[idStr] = &rq.Questionnaire
	s.storage.Replies[idStr] = &Reply{}
	return &StringReply{}, s.save()
}

// ListQuestionnaires requests all questionnaires from Start, but not more than
// Number.
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

// AnswerQuestionnaire sends the answer from one client.
func (s *Service) AnswerQuestionnaire(aq *AnswerQuestionnaire) (*StringReply, error) {
	q := s.storage.Questionnaires[string(aq.QuestID)]
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
	// TODO: send reward to account

	return &StringReply{}, s.save()
}

// TopupQuestionnaire can be used to add new balance to a questionnaire.
func (s *Service) TopupQuestionnaire(tq *TopupQuestionnaire) (*StringReply, error) {
	quest := s.storage.Questionnaires[string(tq.QuestID)]
	if quest == nil {
		return nil, errors.New("this questionnaire doesn't exist")
	}
	quest.Balance += tq.Topup
	return &StringReply{}, nil
}

// SendMessage stores the message in the system.
func (s *Service) SendMessage(sm *SendMessage) (*StringReply, error) {
	log.Lvl2(s.ServerIdentity(), sm.Message)
	idStr := string(sm.Message.ID)
	if msg := s.storage.Messages[idStr]; msg != nil {
		return nil, errors.New("this message-ID already exists")
	}
	s.storage.Messages[idStr] = &sm.Message
	s.storage.Read[idStr] = &readMsg{[]byzcoin.InstanceID{sm.Message.Author}}

	return &StringReply{}, s.save()
}

// ListMessages sorts all messages by balance and sends back the messages from
// Start, but not more than Number.
func (s *Service) ListMessages(lm *ListMessages) (*ListMessagesReply, error) {
	log.Lvl2(s.ServerIdentity(), lm)
	var mreply []Message
	for _, q := range s.storage.Messages {
		for _, r := range s.storage.Read[string(q.ID)].Readers {
			if r.Equal(lm.ReaderID) {
				continue
			}
		}
		if q.Balance >= q.Reward {
			mreply = append(mreply, *q)
		}
	}
	sort.Slice(mreply, func(i, j int) bool {
		return mreply[i].score() > mreply[j].score()
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
		lmr.MsgIDs = append(lmr.MsgIDs, msg.ID)
		lmr.Subjects = append(lmr.Subjects, msg.Subject)
		lmr.Balances = append(lmr.Balances, msg.Balance)
		lmr.Rewards = append(lmr.Rewards, msg.Reward)
		lmr.PartyIIDs = append(lmr.PartyIIDs, msg.PartyIID)
	}
	return lmr, nil
}

// ReadMessage requests the full message and the reward for that message.
func (s *Service) ReadMessage(rm *ReadMessage) (*ReadMessageReply, error) {
	msg := s.storage.Messages[string(rm.MsgID)]
	if msg == nil {
		return nil, errors.New("no such messageID")
	}
	party := s.storage.Parties[string(rm.PartyIID)]
	if party == nil {
		return nil, errors.New("no such partyIID")
	}
	if msg.Balance < msg.Reward ||
		msg.Author.Equal(rm.Reader) {
		return &ReadMessageReply{*msg, false}, nil
	}
	read := s.storage.Read[string(msg.ID)]
	for _, reader := range read.Readers {
		if reader.Equal(rm.Reader) {
			return &ReadMessageReply{*msg, false}, nil
		}
	}
	msg.Balance -= msg.Reward
	read.Readers = append(read.Readers, rm.Reader)

	cl := byzcoin.NewClient(party.ByzCoinID, *party.FinalStatement.Desc.Roster)
	signerCtrs, err := cl.GetSignerCounters(party.Signer.Identity().String())
	if err != nil {
		return nil, err
	}
	if len(signerCtrs.Counters) != 1 {
		return nil, errors.New("incorrect version in signer counter")
	}

	cBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(cBuf, msg.Reward)
	partyCoin := sha256.New()
	partyCoin.Write(rm.PartyIID)
	pubBuf, err := party.Signer.Ed25519.Point.MarshalBinary()
	if err != nil {
		return nil, errors.New("couldn't marshal party public key: " + err.Error())
	}
	partyCoin.Write(pubBuf)
	ctx := byzcoin.ClientTransaction{
		Instructions: []byzcoin.Instruction{{
			InstanceID: byzcoin.NewInstanceID(partyCoin.Sum(nil)),
			Invoke: &byzcoin.Invoke{
				Command: "transfer",
				Args: []byzcoin.Argument{{
					Name:  "coins",
					Value: cBuf,
				},
					{
						Name:  "destination",
						Value: rm.Reader.Slice(),
					}},
			},
			SignerCounter: []uint64{signerCtrs.Counters[0] + 1},
		}},
	}

	err = ctx.SignWith(party.Signer)
	if err != nil {
		return nil, errors.New("couldn't sign: " + err.Error())
	}
	_, err = cl.AddTransactionAndWait(ctx, 10)
	if err != nil {
		return nil, errors.New("couldn't send reward: " + err.Error())
	}

	return &ReadMessageReply{*msg, true}, s.save()
}

// TopupMessage to fill up the balance of a message
func (s *Service) TopupMessage(tm *TopupMessage) (*StringReply, error) {
	msg := s.storage.Messages[string(tm.MsgID)]
	if msg == nil {
		return nil, errors.New("this message doesn't exist")
	}
	msg.Balance += tm.Amount
	return &StringReply{}, nil
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
