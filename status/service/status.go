// Package status is a service for reporting the all the services running on a
// server.
//
// This file contains all the code to run a Stat service. The Stat receives
// takes a request for the Status reports of the server, and sends back the
// status reports for each service in the server.
package status

import (
	"errors"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/messaging"
	"go.dedis.ch/kyber/v3/sign/schnorr"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
	"math"
	"time"
)

// ServiceName is the name to refer to the Status service.
const ServiceName = "Status"

// How old a checkConnectivity request can be before it is refused
const maxRequestAge = 120 * time.Second

func init() {
	_, err := onet.RegisterNewService(ServiceName, newStatService)
	log.ErrFatal(err)
}

// Stat is the service that returns the status reports of all services running
// on a server.
type Stat struct {
	*onet.ServiceProcessor
}

// Version will be set by the main() function before starting the server.
var Version = "unknown"

// Request treats external request to this service.
func (st *Stat) Request(req *Request) (network.Message, error) {
	statuses := st.Context.ReportStatus()

	// Add this in here, because onet no longer knows the version, it is just a
	// support library and should never really have known it.
	statuses["Conode"] = &onet.Status{Field: make(map[string]string)}
	statuses["Conode"].Field["version"] = Version

	log.Lvl4("Returning", statuses)
	return &Response{
		Status:         statuses,
		ServerIdentity: st.ServerIdentity(),
	}, nil
}

var errTimeout = errors.New("timeout while waiting for replies")

// CheckConnectivity does an all-by-all connectivity test
func (st *Stat) CheckConnectivity(req *CheckConnectivity) (*CheckConnectivityReply, error) {
	// Check signature
	hash, err := req.hash()
	if err != nil {
		return nil, errors.New("couldn't hash message: " + err.Error())
	}
	err = schnorr.Verify(cothority.Suite, st.ServerIdentity().Public, hash, req.Signature)
	if err != nil {
		return nil, errors.New("signature verification failed: " + err.Error())
	}
	if math.Abs(time.Now().Sub(time.Unix(req.Time,
		0)).Seconds()) > maxRequestAge.Seconds() {
		return nil, errors.New("too old request")
	}

	list := req.List
	id, _ := onet.NewRoster(list).Search(st.ServerIdentity().ID)
	if id < 0 {
		return nil, errors.New("cannot check nodes without being in the roster")
	}
	if id > 0 {
		log.Lvl1("Re-arranging list")
		list[0], list[id] = list[id], list[0]
	}

	// Test the whole list
	log.Lvl1("Checking whole roster")
	to := time.Duration(req.Timeout)
	err = st.testNodes(list, to)
	if err == nil {
		return &CheckConnectivityReply{Nodes: list}, nil
	}

	if !req.FindFaulty {
		return nil, errors.New("one or more of the nodes did not reply. " +
			"Run with FindFaulty=true")
	}

	// Add one node after the other and only keep nodes that have a full
	// connectivity
	newList := []*network.ServerIdentity{list[0]}
	for _, si := range list[1:] {
		tmpList := append(newList, si)
		log.Lvl1("Checking list", tmpList)
		err = st.testNodes(tmpList, to)
		if err != nil {
			log.Warn("Couldn't contact everybody using node", si)
		} else {
			newList = tmpList
		}
	}

	return &CheckConnectivityReply{newList}, nil
}

func (st *Stat) testNodes(nodes []*network.ServerIdentity, to time.Duration) error {
	r := onet.NewRoster(nodes)
	tree := r.GenerateBinaryTree()
	p, err := st.CreateProtocol(messaging.BroadcastName, tree)
	if err != nil {
		return errors.New("protocol creation failed: " + err.Error())
	}
	bc := p.(*messaging.Broadcast)
	done := make(chan bool)
	bc.RegisterOnDone(func() {
		done <- true
	})
	if err = p.Start(); err != nil {
		return errors.New("couldn't start protocol: " + err.Error())
	}
	select {
	case <-done:
		return nil
	case <-time.After(to):
		return errTimeout
	}
}

// newStatService creates a new service that is built for Status
func newStatService(c *onet.Context) (onet.Service, error) {
	s := &Stat{
		ServiceProcessor: onet.NewServiceProcessor(c),
	}
	err := s.RegisterHandlers(s.Request, s.CheckConnectivity)
	if err != nil {
		return nil, errors.New("couldn't register handlers: " + err.Error())
	}

	return s, nil
}
