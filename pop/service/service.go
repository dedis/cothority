package service

/*
The service.go defines what to do for each API-call. This part of the service
runs on the node.
*/

import (
	"errors"
	"fmt"
	"math/rand"

	"bytes"

	"time"

	"github.com/dedis/crypto/abstract"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
)

// Name is the name to refer to the Template service from another
// package.
const Name = "PoPServer"
const cfgName = "pop.bin"

var checkConfigID network.MessageTypeID
var checkConfigReplyID network.MessageTypeID

func init() {
	onet.RegisterNewService(Name, newService)
	network.RegisterMessage(CheckConfig{})
	network.RegisterMessage(saveData{})
	checkConfigID = network.RegisterMessage(CheckConfig{})
	checkConfigReplyID = network.RegisterMessage(CheckConfigReply{})
}

// Service is our template-service
type Service struct {
	// We need to embed the ServiceProcessor, so that incoming messages
	// are correctly handled.
	*onet.ServiceProcessor
	path string
	data *saveData
	// channel to return the configreply
	ccChannel chan *CheckConfigReply
}

type saveData struct {
	// Pin holds the randomly chosen pin
	Pin string
	// Public key of linked pop
	Public abstract.Point
	// The final statement
	Final *FinalStatement
}

// PinRequest prints out a pin if none is given, else it verifies it has the
// correct pin, and if so, it stores the public key as reference.
func (s *Service) PinRequest(req *PinRequest) (network.Message, onet.ClientError) {
	if req.Pin == "" {
		rand.Seed(int64(time.Now().Nanosecond()))
		s.data.Pin = fmt.Sprintf("%06d", rand.Intn(1000000))
		log.Info("PIN:", s.data.Pin)
		return nil, onet.NewClientErrorCode(ErrorWrongPIN, "Read PIN in server-log")
	}
	if req.Pin != s.data.Pin {
		return nil, onet.NewClientErrorCode(ErrorWrongPIN, "Wrong PIN")
	}
	s.data.Public = req.Public
	s.save()
	log.Lvl1("Successfully registered PIN/Public", s.data.Pin, req.Public)
	return nil, nil
}

// StoreConfig saves the pop-config locally
func (s *Service) StoreConfig(req *StoreConfig) (network.Message, onet.ClientError) {
	log.Lvlf3("%s %v %x", s.Context.ServerIdentity(), req.Desc, req.Desc.Hash())
	if req.Desc.Roster == nil {
		return nil, onet.NewClientErrorCode(ErrorInternal, "no roster set")
	}
	if s.data.Public == nil {
		return nil, onet.NewClientErrorCode(ErrorInternal, "Not linked yet")
	}
	s.data.Final = &FinalStatement{Desc: req.Desc, Signature: []byte{}}
	s.save()
	return &StoreConfigReply{req.Desc.Hash()}, nil
}

// FinalizeRequest returns the FinalStatement if all conodes already received
// a PopDesc and signed off. The FinalStatement holds the updated PopDesc, the
// pruned attendees-public-key-list and the collective signature.
func (s *Service) FinalizeRequest(req *FinalizeRequest) (network.Message, onet.ClientError) {
	if s.data.Final == nil || s.data.Final.Desc == nil {
		return nil, onet.NewClientErrorCode(ErrorInternal, "Not linked or no config yet")
	}
	// Contact all other nodes and ask them if they already have a config.
	s.data.Final.Attendees = make([]abstract.Point, len(req.Attendees))
	copy(s.data.Final.Attendees, req.Attendees)
	cc := &CheckConfig{s.data.Final.Desc.Hash(), req.Attendees}
	for _, c := range s.data.Final.Desc.Roster.List {
		if !c.ID.Equal(s.ServerIdentity().ID) {
			log.Lvl3("Contacting", c, cc.Attendees)
			err := s.SendRaw(c, cc)
			if err != nil {
				return nil, onet.NewClientErrorCode(ErrorInternal, err.Error())
			}
			rep := <-s.ccChannel
			if rep == nil {
				return nil, onet.NewClientErrorCode(ErrorOtherFinals,
					"Not all other conodes finalized yet")
			}
		}
	}
	s.data.Final.Signature = []byte("signed")
	s.save()
	return &FinalizeResponse{s.data.Final}, nil
}

// CheckConfig receives a hash for a config and a list of attendees. It returns
// a CheckConfigReply filled according to this structure's description. If
// the config has been found, it strips its own attendees from the one missing
// in the other configuration.
func (s *Service) CheckConfig(req *network.Envelope) {
	cc, ok := req.Msg.(CheckConfig)
	if !ok {
		log.Errorf("Didn't get a CheckConfig: %#v", req.Msg)
		return
	}

	ccr := &CheckConfigReply{0, cc.PopHash, nil}
	if s.data.Final != nil {
		if !bytes.Equal(s.data.Final.Desc.Hash(), cc.PopHash) {
			ccr.PopStatus = 1
		} else {
			s.intersectAttendees(cc.Attendees)
			if len(s.data.Final.Attendees) == 0 {
				ccr.PopStatus = 2
			} else {
				ccr.PopStatus = 3
				ccr.Attendees = s.data.Final.Attendees
			}
		}
	}
	log.Lvl3(s.Context.ServerIdentity(), ccr.PopStatus, ccr.Attendees)
	err := s.SendRaw(req.ServerIdentity, ccr)
	if err != nil {
		log.Error("Couldn't send reply:", err)
	}
}

// CheckConfigReply strips the attendees missing in the reply, if the
// PopStatus == 3.
func (s *Service) CheckConfigReply(req *network.Envelope) {
	ccrVal, ok := req.Msg.(CheckConfigReply)
	var ccr *CheckConfigReply
	ccr = func() *CheckConfigReply {
		if !ok {
			log.Errorf("Didn't get a CheckConfigReply: %v", req.Msg)
			return nil
		}
		if !bytes.Equal(ccrVal.PopHash, s.data.Final.Desc.Hash()) {
			log.Error("Not correct hash")
			return nil
		}
		if ccrVal.PopStatus < 3 {
			log.Warn("Wrong pop-status:", ccrVal.PopStatus)
			return nil
		}
		s.intersectAttendees(ccrVal.Attendees)
		return &ccrVal
	}()
	if len(s.ccChannel) == 0 {
		s.ccChannel <- ccr
	}
}

// Get intersection of attendees
func (s *Service) intersectAttendees(atts []abstract.Point) {
	na := []abstract.Point{}
	for i, p := range s.data.Final.Attendees {
		for _, d := range atts {
			if p.Equal(d) {
				na = append(na, p)
				continue
			}
		}
		s.data.Final.Attendees[i] = nil
	}
	s.data.Final.Attendees = na
}

// saves the actual identity
func (s *Service) save() {
	log.LLvl3("Saving service")
	err := s.Save("storage", s.data)
	if err != nil {
		log.Error("Couldn't save data:", err)
	}
	log.Print("saved succesfully")
}

// Tries to load the configuration and updates if a configuration
// is found, else it returns an error.
func (s *Service) tryLoad() error {
	if !s.DataAvailable("storage") {
		log.Print("No data available")
		return nil
	}
	msg, err := s.Load("storage")
	if err != nil {
		return err
	}
	var ok bool
	s.data, ok = msg.(*saveData)
	if !ok {
		return errors.New("Data of wrong type")
	}
	log.Print("loaded succesfully")
	return nil
}

// newService registers the request-methods.
func newService(c *onet.Context) onet.Service {
	s := &Service{
		ServiceProcessor: onet.NewServiceProcessor(c),
		data:             &saveData{},
		ccChannel:        make(chan *CheckConfigReply, 1),
	}
	if err := s.RegisterHandlers(s.PinRequest, s.StoreConfig, s.FinalizeRequest); err != nil {
		log.ErrFatal(err, "Couldn't register messages")
	}
	if err := s.tryLoad(); err != nil {
		log.Error(err)
	}
	s.RegisterProcessorFunc(checkConfigID, s.CheckConfig)
	s.RegisterProcessorFunc(checkConfigReplyID, s.CheckConfigReply)
	return s
}
