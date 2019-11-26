package personhood

/*
The service.go defines what to do for each API-call. This part of the service
runs on the node.
*/

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"time"

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

	err = byzcoin.RegisterGlobalContract(ContractPopPartyID, ContractPopPartyFromBytes)
	if err != nil {
		log.ErrFatal(err)
	}
	err = byzcoin.RegisterGlobalContract(ContractSpawnerID, ContractSpawnerFromBytes)
	if err != nil {
		log.ErrFatal(err)
	}
	err = byzcoin.RegisterGlobalContract(ContractCredentialID, ContractCredentialFromBytes)
	if err != nil {
		log.ErrFatal(err)
	}
	err = byzcoin.RegisterGlobalContract(ContractRoPaSciID, ContractRoPaSciFromBytes)
	if err != nil {
		log.ErrFatal(err)
	}
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
		},
	}, nil
}

// Meetup simulates an anonymous user detection. It should work without a service,
// just locally, perhaps via bluetooth or sound.
func (s *Service) Meetup(rq *Meetup) (*MeetupResponse, error) {
	if rq.Wipe != nil && *rq.Wipe {
		s.meetups = []UserLocation{}
		return &MeetupResponse{}, nil
	}
	if rq.UserLocation != nil {
		rq.UserLocation.Time = time.Now().Unix()
		// Prune old entries, supposing they're in chronological order
		for i := len(s.meetups) - 1; i >= 0; i-- {
			if s.meetups[i].PublicKey.Equal(rq.UserLocation.PublicKey) {
				s.meetups = append(s.meetups[:i], s.meetups[i+1:]...)
				continue
			}
			if time.Now().Unix()-(s.meetups[i].Time) > 60 {
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
	cpop, err := s.byzcoinService().GetContractInstance(ContractPopPartyID, val)
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
			cbc, err := s.byzcoinService().GetContractInstance(ContractRoPaSciID, buf)
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
		// Remove finalized parties from the returned result
		if err == nil && party.State < FinalizedState {
			parties = append(parties, *p)
		}
	}
	err := s.save()
	if err != nil {
		return nil, err
	}
	return &PartyListResponse{Parties: parties}, nil
}

func (s *Service) byzcoinService() *byzcoin.Service {
	return s.Service(byzcoin.ServiceName).(*byzcoin.Service)
}

func getParty(p *Party) (cpp *ContractPopParty, err error) {
	cl := byzcoin.NewClient(p.ByzCoinID, p.Roster)
	pr, err := cl.GetProofFromLatest(p.InstanceID.Slice())
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

func newService(c *onet.Context) (onet.Service, error) {
	s := &Service{
		ServiceProcessor: onet.NewServiceProcessor(c),
	}
	if err := s.RegisterHandlers(s.Capabilities, s.Meetup, s.Poll, s.RoPaSciList, s.PartyList); err != nil {
		return nil, errors.New("couldn't register messages")
	}

	if err := s.tryLoad(); err != nil {
		log.Error(err)
		return nil, err
	}
	s.storage.RoPaSci = []*RoPaSci{}
	if len(s.storage.Parties) == 0 {
		s.storage.Parties = make(map[string]*Party)
	}
	if len(s.storage.Polls) == 0 {
		s.storage.Polls = make(map[string]*storagePolls)
	}
	return s, nil
}
