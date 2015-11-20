package conode

import (
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	dbg "github.com/dedis/cothority/lib/debug_lvl"

	"github.com/dedis/cothority/lib/sign"
)

type Peer struct {
	*sign.Node
	*StampListener

	RLock     sync.Mutex
	MaxRounds int
	CloseChan chan bool

	Logger   string
	Hostname string
	App      string
}

// NewPeer returns a peer that can be used to set up
// connections.
func NewPeer(node *sign.Node) *Peer {
	s := &Peer{}

	s.Node = node
	s.RLock = sync.Mutex{}

	s.CloseChan = make(chan bool, 5)
	s.StampListener = NewStampListener(s.Node.Name())
	return s
}

// Listen on client connections. If role is root also send annoucement
// for all of the nRounds
func (s *Peer) Run(role string) {
	dbg.Lvl3("Stamp-server", s.Node.Name(), "starting with ", role)
	RegisterRoundCosiStamper(s)
	RegisterRoundStamper(s)

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
	dbg.Lvl4("closing stampserver: %p", s.Node.Name())
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
			err = s.StartAnnouncement(NewRoundCosiStamper(s))
			if err != nil {
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
