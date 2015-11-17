package conode

import (
	"net"
	"strconv"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	dbg "github.com/dedis/cothority/lib/debug_lvl"

	"github.com/dedis/cothority/lib/logutils"
	"github.com/dedis/cothority/lib/sign"
)

type Peer struct {
	*sign.Node
	NameP string

	RLock     sync.Mutex
	MaxRounds int
	CloseChan chan bool

	Logger   string
	Hostname string
	App      string
	Cb       sign.Round
}

// NewPeer returns a peer that can be used to set up
// connections. It takes a signer and a callbacks-struct
// that need to be initialised already.
func NewPeer(node *sign.Node, cb sign.Round) *Peer {
	s := &Peer{}

	s.Node = node
	s.Cb = cb
	s.Node.SetCallbacks(cb)
	s.RLock = sync.Mutex{}

	// listen for client requests at one port higher
	// than the signing node
	h, p, err := net.SplitHostPort(s.Node.Name())
	if err == nil {
		i, err := strconv.Atoi(p)
		if err != nil {
			log.Fatal(err)
		}
		s.NameP = net.JoinHostPort(h, strconv.Itoa(i+1))
	}
	s.CloseChan = make(chan bool, 5)
	return s
}

// listen for clients connections
func (s *Peer) Setup() error {
	return s.Cb.Setup(s.NameP)
}

// Listen on client connections. If role is root also send annoucement
// for all of the nRounds
func (s *Peer) Run(role string) {
	dbg.Lvl3("Stamp-server", s.NameP, "starting with ", role)
	closed := make(chan bool, 1)

	go func() { err := s.Node.Listen(); closed <- true; s.Close(); log.Error(err) }()
	s.RLock.Lock()

	// TODO: remove this hack
	s.MaxRounds = -1
	s.RLock.Unlock()

	var nextRole string // next role when view changes
	for {
		switch role {

		case "root":
			dbg.Lvl4("running as root")
			nextRole = s.runAsRoot(s.MaxRounds)
		case "regular":
			dbg.Lvl4("running as regular")
			nextRole = s.runAsRegular()
		default:
			dbg.Fatal("Unable to run as anything")
			return
		}

		dbg.Lvl2(s.Name(), "Role now:", role, "nextRole:", nextRole)
		if nextRole == "close" {
			s.Close()
			return
		}
		role = nextRole
	}

}

// Closes the channel
func (s *Peer) Close() {
	dbg.Lvl4("closing stampserver: %p", s.NameP)
	s.CloseChan <- true
	s.Node.Close()
}

// This node is the root-node - still possible to change
// the role
func (s *Peer) runAsRoot(nRounds int) string {
	// every 5 seconds start a new round
	ticker := time.Tick(sign.ROUND_TIME)
	if s.LastRound()+1 > nRounds && nRounds >= 0 {
		dbg.Lvl1(s.Name(), "runAsRoot called with too large round number")
		return "close"
	}

	dbg.Lvl3(s.Name(), "running as root", s.LastRound(), int64(nRounds))
	for {
		select {
		case nextRole := <-s.ViewChangeCh():
			dbg.Lvl4(s.Name(), "assuming next role")
			return nextRole
		// s.reRunWith(nextRole, nRounds, true)
		case <-ticker:

			dbg.Lvl4(s.Name(), "Stamp server in round", s.LastRound()+1, "of", nRounds)

			var err error
			if s.App == "vote" {
				vote := &sign.Vote{
					Type: sign.AddVT,
					Av: &sign.AddVote{
						Parent: s.Name(),
						Name:   "test-add-node"}}
				err = s.StartVotingRound(vote)
			} else {
				err = s.StartSigningRound()
			}
			if err == sign.ChangingViewError {
				// report change in view, and continue with the select
				log.WithFields(log.Fields{
					"file": logutils.File(),
					"type": "view_change",
				}).Info("Tried to stary signing round on " + s.Name() + " but it reports view change in progress")
				// skip # of failed round
				time.Sleep(1 * time.Second)
				break
			} else if err != nil {
				dbg.Lvl3(err)
				time.Sleep(1 * time.Second)
				break
			}

			if s.LastRound()+1 >= nRounds && nRounds >= 0 {
				dbg.Lvl2(s.Name(), "reports exceeded the max round: terminating", s.LastRound()+1, ">=", nRounds)
				return "close"
			}
		}
	}
}

// This node is a child of the root-node
func (s *Peer) runAsRegular() string {
	select {
	case <-s.CloseChan:
		dbg.Lvl3("server", s.Name(), "has closed the connection")
		return ""

	case nextRole := <-s.ViewChangeCh():
		return nextRole
	}
}
