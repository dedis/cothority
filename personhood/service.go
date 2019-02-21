package personhood

/*
The service.go defines what to do for each API-call. This part of the service
runs on the node.
*/

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"sort"
	"time"

	"go.dedis.ch/cothority/v3/byzcoin/contracts"

	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/kyber/v3/sign/anon"

	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
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

	// meetups is a list of last users calling.
	meetups []UserLocation

	storage *storage1
}

// Capabilities returns the version of endpoints this conode offers:
// The versioning is a 24 bit value, that can be interpreted in hexadecimal
// as the following:
//   Version = [3]byte{xx, yy, zz}
//   - xx - major version - incompatible
//   - yy - minor version - downwards compatible. A client with a lower number will be able
//     to interact with this server
//   - zz - patch version - whatever suits you - higher is better, but no incompatibilities
func (s *Service) Capabilities(rq *Capabilities) (*CapabilitiesResponse, error) {
	return &CapabilitiesResponse{
		Capabilities: []Capability{
			{
				Endpoint: "byzcoin",
				Version:  [3]byte{2, 2, 0},
			},
			{
				Endpoint: "poll",
				Version:  [3]byte{0, 0, 1},
			},
			{
				Endpoint: "ropascilist",
				Version:  [3]byte{0, 1, 1},
			},
			{
				Endpoint: "partylist",
				Version:  [3]byte{0, 1, 0},
			},
			{
				Endpoint: "teststore",
				Version:  [3]byte{0, 1, 0},
			},
		},
	}, nil
}

// Meetup simulates an anonymous user detection. It should work without a service,
// just locally, perhaps via bluetooth or sound.
func (s *Service) Meetup(rq *Meetup) (*MeetupResponse, error) {
	if rq.Wipe != nil && *rq.Wipe {
		log.Print("Wiping Meetups")
		s.meetups = []UserLocation{}
		return &MeetupResponse{}, nil
	}
	if rq.UserLocation != nil {
		rq.UserLocation.Time = time.Now().Unix()
		// Prune old entries, supposing they're in chronological order
		for i := len(s.meetups) - 1; i >= 0; i-- {
			if s.meetups[i].PublicKey.Equal(rq.UserLocation.PublicKey) {
				log.Print("found same meetup")
				s.meetups = append(s.meetups[:i], s.meetups[i+1:]...)
				continue
			}
			if time.Now().Unix()-(s.meetups[i].Time) > 60 {
				log.Print("deleting", i)
				s.meetups = append(s.meetups[0:i], s.meetups[i+1:]...)
			}
		}
		s.meetups = append(s.meetups, *rq.UserLocation)
		// Prune if list is too long
		if len(s.meetups) > 20 {
			s.meetups = append(s.meetups[1:])
		}
	}
	reply := &MeetupResponse{}
	for _, m := range s.meetups {
		log.Printf("adding %+v", m)
		reply.Users = append(reply.Users, m)
	}
	return reply, nil
}

// Poll handles anonymous, troll-resistant polling.
func (s *Service) Poll(rq *Poll) (*PollResponse, error) {
	sps := s.storage.Polls[string(rq.ByzCoinID)]
	if sps == nil {
		s.storage.Polls[string(rq.ByzCoinID)] = &storagePolls{}
		return s.Poll(rq)
	}
	switch {
	case rq.NewPoll != nil:
		np := PollStruct{
			Title:       rq.NewPoll.Title,
			Description: rq.NewPoll.Description,
			Choices:     rq.NewPoll.Choices,
			Personhood:  rq.NewPoll.Personhood,
			PollID:      rq.NewPoll.PollID,
		}
		_, err := s.getPopContract(rq.ByzCoinID, np.Personhood.Slice())
		if err != nil {
			return nil, err
		}
		//np.PollID = random.Bits(256, true, random.New())
		sps.Polls = append(sps.Polls, &np)
		return &PollResponse{Polls: []PollStruct{np}}, s.save()
	case rq.List != nil:
		pr := &PollResponse{Polls: []PollStruct{}}
		for _, p := range sps.Polls {
			member := false
			for _, id := range rq.List.PartyIDs {
				if id.Equal(p.Personhood) {
					member = true
					break
				}
			}
			if member {
				pr.Polls = append(pr.Polls, *p)
			}
		}
		return pr, s.save()
	case rq.Answer != nil:
		var poll *PollStruct
		for _, p := range sps.Polls {
			if bytes.Compare(p.PollID, rq.Answer.PollID) == 0 {
				poll = p
				break
			}
		}
		if poll == nil {
			return nil, errors.New("didn't find that poll")
		}
		if rq.Answer.Choice < 0 ||
			rq.Answer.Choice >= len(poll.Choices) {
			return nil, errors.New("this choice doesn't exist")
		}

		msg := append([]byte("Choice"), byte(rq.Answer.Choice))
		scope := append([]byte("Poll"), append(rq.ByzCoinID, poll.PollID...)...)
		scopeHash := sha256.Sum256(scope)
		ph, err := s.getPopContract(rq.ByzCoinID, poll.Personhood.Slice())
		if err != nil {
			return nil, err
		}
		tag, err := anon.Verify(&suiteBlake2s{}, msg, ph.Attendees.Keys, scopeHash[:], rq.Answer.LRS)
		if err != nil {
			return nil, err
		}
		var update bool
		for i, c := range poll.Chosen {
			if bytes.Compare(c.LRSTag, tag) == 0 {
				log.Lvl2("Updating choice", i)
				poll.Chosen[i].Choice = rq.Answer.Choice
				update = true
				break
			}
		}
		if !update {
			poll.Chosen = append(poll.Chosen, PollChoice{Choice: rq.Answer.Choice, LRSTag: tag})
		}
		return &PollResponse{Polls: []PollStruct{*poll}}, s.save()
	default:
		s.storage.Polls[string(rq.ByzCoinID)] = &storagePolls{Polls: []*PollStruct{}}
		return &PollResponse{Polls: []PollStruct{}}, s.save()
	}
}

func (s *Service) getPopContract(bcID skipchain.SkipBlockID, phIID []byte) (*ContractPopParty, error) {
	gpr, err := s.Service(byzcoin.ServiceName).(*byzcoin.Service).GetProof(&byzcoin.GetProof{
		Version: byzcoin.CurrentVersion,
		Key:     phIID,
		ID:      bcID,
	})
	if err != nil {
		return nil, err
	}
	val, cid, _, err := gpr.Proof.Get(phIID)
	if err != nil {
		return nil, err
	}
	if cid != ContractPopPartyID {
		return nil, errors.New("this is not a personhood contract")
	}
	cpop, err := ContractPopPartyFromBytes(val)
	return cpop.(*ContractPopParty), err
}

// RoPaSciList can either store a new rock-paper-scissors in the list, or just return the list of
// available RoPaScis. It removes finalized RoPaScis, as they should not be picked up
// by new clients.
func (s *Service) RoPaSciList(rq *RoPaSciList) (*RoPaSciListResponse, error) {
	log.Lvl1(s.ServerIdentity(), "RoPaSciList:", rq, s.storage.RoPaSci)
	if rq.Wipe != nil && *rq.Wipe {
		log.Lvl2(s.ServerIdentity(), "Wiping all known rock-paper-scissor games")
		s.storage.RoPaSci = []*RoPaSci{}
		return &RoPaSciListResponse{}, nil
	}
	if rq.NewRoPaSci != nil {
		s.storage.RoPaSci = append(s.storage.RoPaSci, rq.NewRoPaSci)
	}
	var roPaScis []RoPaSci
	for i := 0; i < len(s.storage.RoPaSci); i++ {
		rps := s.storage.RoPaSci[i]
		err := func() error {
			reply, err := s.Service(byzcoin.ServiceName).(*byzcoin.Service).GetProof(&byzcoin.GetProof{
				Version: byzcoin.CurrentVersion,
				Key:     rps.RoPaSciID.Slice(),
				ID:      rps.ByzcoinID,
			})
			if err != nil {
				return err
			}
			buf, _, _, err := reply.Proof.Get(rps.RoPaSciID.Slice())
			if err != nil {
				return err
			}
			cbc, err := ContractRoPaSciFromBytes(buf)
			if err != nil {
				return err
			}
			if cbc.(*ContractRoPaSci).SecondPlayer >= 0 {
				return errors.New("finished game")
			}
			return nil
		}()
		if err != nil {
			log.Error(s.ServerIdentity(), "Removing RockPaperScissors instance from list:", err)
			s.storage.RoPaSci = append(s.storage.RoPaSci[0:i], s.storage.RoPaSci[i+1:]...)
			i--
			continue
		}
		roPaScis = append(roPaScis, *rps)
	}
	err := s.save()
	if err != nil {
		return nil, err
	}
	return &RoPaSciListResponse{RoPaScis: roPaScis}, nil
}

// PartyList can either store a new party in the list, or just return the list of
// available parties. It doesn't return finalized parties, so as not to confuse the
// clients, but keeps them in the list for other methods like ReadMessage.
func (s *Service) PartyList(rq *PartyList) (*PartyListResponse, error) {
	log.Lvlf2("PartyList: %+v", rq)
	if rq.WipeParties != nil && *rq.WipeParties {
		log.Lvl2(s.ServerIdentity(), "Wiping party cache")
		s.storage.Parties = map[string]*Party{}
	}
	if rq.NewParty != nil {
		s.storage.Parties[string(rq.NewParty.InstanceID.Slice())] = rq.NewParty
	}
	var parties []Party
	for _, p := range s.storage.Parties {
		party, err := getParty(p)
		// Remove finalized parties
		if err == nil && party.State < 3 {
			parties = append(parties, *p)
		}
	}
	err := s.save()
	if err != nil {
		return nil, err
	}
	return &PartyListResponse{Parties: parties}, nil
}

func getParty(p *Party) (cpp *ContractPopParty, err error) {
	cl := byzcoin.NewClient(p.ByzCoinID, p.Roster)
	pr, err := cl.GetProof(p.InstanceID.Slice())
	if err != nil {
		return
	}
	buf, cid, _, err := pr.Proof.Get(p.InstanceID.Slice())
	if err != nil {
		return
	}
	if cid != ContractPopPartyID {
		err = errors.New("didn't get a party instance")
		return
	}
	cbc, err := ContractPopPartyFromBytes(buf)
	return cbc.(*ContractPopParty), err
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

	cl := byzcoin.NewClient(party.ByzCoinID, party.Roster)
	signerCtrs, err := cl.GetSignerCounters(party.Signer.Identity().String())
	if err != nil {
		return nil, err
	}
	if len(signerCtrs.Counters) != 1 {
		return nil, errors.New("incorrect version in signer counter")
	}

	cBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(cBuf, msg.Reward)
	ctx := byzcoin.ClientTransaction{
		Instructions: []byzcoin.Instruction{{
			InstanceID: party.CoinIID,
			Invoke: &byzcoin.Invoke{
				Command:    "transfer",
				ContractID: contracts.ContractCoinID,
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

	err = ctx.FillSignersAndSignWith(party.Signer)
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

// TestStore allows easier testing of the mobile apps by giving an endpoint
// where current testing data can be stored.
func (s *Service) TestStore(ts *TestStore) (*TestStore, error) {
	if ts.ByzCoinID != nil && len(ts.ByzCoinID) == 32 {
		log.Lvlf1("Storing TestStore %x / %x", ts.ByzCoinID, ts.SpawnerIID.Slice())
		s.storage.Ts.ByzCoinID = ts.ByzCoinID
		s.storage.Ts.SpawnerIID = ts.SpawnerIID
	} else {
		log.Lvlf1("Retrieving TestStore %x / %x", s.storage.Ts.ByzCoinID[:], s.storage.Ts.SpawnerIID[:])
	}
	return &s.storage.Ts, nil
}

func newService(c *onet.Context) (onet.Service, error) {
	s := &Service{
		ServiceProcessor: onet.NewServiceProcessor(c),
	}
	if err := s.RegisterHandlers(s.Capabilities, s.Meetup, s.Poll, s.RoPaSciList, s.PartyList,
		s.TestStore); err != nil {
		return nil, errors.New("couldn't register messages")
	}
	byzcoin.RegisterContract(c, ContractPopPartyID, ContractPopPartyFromBytes)
	byzcoin.RegisterContract(c, ContractSpawnerID, ContractSpawnerFromBytes)
	byzcoin.RegisterContract(c, ContractCredentialID, ContractCredentialFromBytes)
	byzcoin.RegisterContract(c, ContractRoPaSciID, ContractRoPaSciFromBytes)

	if err := s.tryLoad(); err != nil {
		log.Error(err)
		return nil, err
	}
	//bid, _ := hex.DecodeString("9a812404dd8306bcae1cf419a643c21041731a8972b1ddbe3295614706c9183c")
	//sid, _ := hex.DecodeString("8898f2dd77ec045cd1ec67302f029e513ced173ab8ccf2c8ee4c9a306bd39091")
	//s.storage.Ts = TestStore{
	//	ByzCoinID:  bid,
	//	SpawnerIID: byzcoin.NewInstanceID(sid),
	//}
	s.storage.RoPaSci = []*RoPaSci{}
	if len(s.storage.Messages) == 0 {
		s.storage.Messages = make(map[string]*Message)
	}
	if len(s.storage.Questionnaires) == 0 {
		s.storage.Questionnaires = make(map[string]*Questionnaire)
	}
	if len(s.storage.Parties) == 0 || true {
		s.storage.Parties = make(map[string]*Party)
	}
	if len(s.storage.Replies) == 0 {
		s.storage.Replies = make(map[string]*Reply)
	}
	if len(s.storage.Read) == 0 {
		s.storage.Read = make(map[string]*readMsg)
	}
	if len(s.storage.Polls) == 0 {
		s.storage.Polls = make(map[string]*storagePolls)
	}
	return s, nil
}
