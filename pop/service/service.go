package service

/*
The service.go defines what to do for each API-call. This part of the service
runs on the node.
*/

import (
	"fmt"
	"math/rand"

	"bytes"

	"io/ioutil"
	"os"

	"path"

	"github.com/dedis/crypto/abstract"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
)

// Name is the name to refer to the Template service from another
// package.
const Name = "PoPServer"
const cfgName = "pop.bin"

var checkConfigID network.PacketTypeID
var checkConfigReplyID network.PacketTypeID

func init() {
	onet.RegisterNewService(Name, newService)
	network.RegisterPacketType(CheckConfig{})
	checkConfigID = network.RegisterPacketType(CheckConfig{})
	checkConfigReplyID = network.RegisterPacketType(CheckConfigReply{})
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
	pin string
	// Public key of linked pop
	public abstract.Point
	// The final statement
	final *FinalStatement
}

// PinRequest prints out a pin if none is given, else it verifies it has the
// correct pin, and if so, it stores the public key as reference.
func (s *Service) PinRequest(req *PinRequest) (network.Body, onet.ClientError) {
	if req.Pin == "" {
		s.data.pin = fmt.Sprintf("%06d", rand.Intn(100000))
		log.Info("PIN:", s.data.pin)
		return nil, onet.NewClientErrorCode(ErrorWrongPIN, "Read PIN in server-log")
	}
	if req.Pin != s.data.pin {
		return nil, onet.NewClientErrorCode(ErrorWrongPIN, "Wrong PIN")
	}
	s.data.public = req.Public
	return nil, nil
}

// StoreConfig saves the pop-config locally
func (s *Service) StoreConfig(req *StoreConfig) (network.Body, onet.ClientError) {
	log.Lvlf3("%s %v %x", s.Context.ServerIdentity(), req.Desc, req.Desc.Hash())
	if req.Desc.Roster == nil {
		return nil, onet.NewClientErrorCode(ErrorInternal, "no roster set")
	}
	s.data.final = &FinalStatement{Desc: req.Desc}
	return &StoreConfigReply{req.Desc.Hash()}, nil
}

// FinalizeRequest returns the FinalStatement if all conodes already received
// a PopDesc and signed off. The FinalStatement holds the updated PopDesc, the
// pruned attendees-public-key-list and the collective signature.
func (s *Service) FinalizeRequest(req *FinalizeRequest) (network.Body, onet.ClientError) {
	if s.data.final == nil || s.data.final.Desc == nil {
		return nil, onet.NewClientErrorCode(ErrorInternal, "no config yet")
	}
	// Contact all other nodes and ask them if they already have a config.
	s.data.final.Attendees = make([]abstract.Point, len(req.Attendees))
	copy(s.data.final.Attendees, req.Attendees)
	cc := &CheckConfig{s.data.final.Desc.Hash(), req.Attendees}
	for _, c := range s.data.final.Desc.Roster.List {
		if !c.ID.Equal(s.ServerIdentity().ID) {
			log.LLvl3("Contacting", c, cc.Attendees)
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
	s.data.final.Signature = []byte("signed")
	return &FinalizeResponse{s.data.final}, nil
}

// CheckConfig receives a hash for a config and a list of attendees. It returns
// a CheckConfigReply filled according to this structure's description. If
// the config has been found, it strips its own attendees from the one missing
// in the other configuration.
func (s *Service) CheckConfig(req *network.Packet) {
	cc, ok := req.Msg.(CheckConfig)
	if !ok {
		log.Errorf("Didn't get a CheckConfig: %#v", req.Msg)
		return
	}

	ccr := &CheckConfigReply{0, cc.PopHash, nil}
	if s.data.final != nil {
		if !bytes.Equal(s.data.final.Desc.Hash(), cc.PopHash) {
			ccr.PopStatus = 1
		} else {
			s.intersectAttendees(cc.Attendees)
			if len(s.data.final.Attendees) == 0 {
				ccr.PopStatus = 2
			} else {
				ccr.PopStatus = 3
			}
		}
	}
	ccr.Attendees = s.data.final.Attendees
	log.Lvl3(s.Context.ServerIdentity(), ccr.PopStatus, ccr.Attendees)
	err := s.SendRaw(req.ServerIdentity, ccr)
	if err != nil {
		log.Error("Couldn't send reply:", err)
	}
}

// CheckConfigReply strips the attendees missing in the reply, if the
// PopStatus == 3.
func (s *Service) CheckConfigReply(req *network.Packet) {
	ccrVal, ok := req.Msg.(CheckConfigReply)
	var ccr *CheckConfigReply
	ccr = func() *CheckConfigReply {
		if !ok {
			log.Errorf("Didn't get a CheckConfigReply: %v", req.Msg)
			return nil
		}
		if !bytes.Equal(ccrVal.PopHash, s.data.final.Desc.Hash()) {
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
	for i, p := range s.data.final.Attendees {
		for _, d := range atts {
			if p.Equal(d) {
				na = append(na, p)
				continue
			}
		}
		s.data.final.Attendees[i] = nil
	}
	s.data.final.Attendees = na
}

// saves the actual identity
func (s *Service) save() {
	log.Lvl3("Saving service")
	b, err := network.MarshalRegisteredType(s.data)
	if err != nil {
		log.Error("Couldn't marshal service:", err)
	} else {
		err = ioutil.WriteFile(path.Join(s.path, cfgName), b, 0660)
		if err != nil {
			log.Error("Couldn't save file:", err)
		}
	}
}

// Tries to load the configuration and updates if a configuration
// is found, else it returns an error.
func (s *Service) tryLoad() error {
	configFile := path.Join(s.path, cfgName)
	b, err := ioutil.ReadFile(configFile)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("Error while reading %s: %s", configFile, err)
	}
	if len(b) > 0 {
		_, msg, err := network.UnmarshalRegistered(b)
		if err != nil {
			return fmt.Errorf("Couldn't unmarshal: %s", err)
		}
		log.Lvl3("Successfully loaded")
		s.data = msg.(*saveData)
	}
	return nil
}

// newService registers the request-methods.
func newService(c *onet.Context, path string) onet.Service {
	s := &Service{
		ServiceProcessor: onet.NewServiceProcessor(c),
		path:             path,
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
