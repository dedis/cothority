package conode

import (
	"net"
	"strconv"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	dbg "github.com/dedis/cothority/lib/debug_lvl"

	"github.com/dedis/cothority/lib/logutils"
	"github.com/dedis/cothority/proto/sign"
)

var ROUND_TIME time.Duration = sign.ROUND_TIME
var PeerMaxRounds = -1

// struct to ease keeping track of who requires a reply after
// tsm is processed/ aggregated by the TSServer
type MustReplyMessage struct {
	Tsm TimeStampMessage
	To  string // name of reply destination
}

type Peer struct {
	sign.Signer
	name string

	rLock     sync.Mutex
	maxRounds int
	closeChan chan bool

	Logger   string
	Hostname string
	App      string
	Cb       Callbacks
}

func NewPeer(signer sign.Signer, cb Callbacks) *Peer {
	s := &Peer{}

	s.Signer = signer
	s.Cb = cb
	s.Signer.RegisterAnnounceFunc(cb.AnnounceFunc(s))
	s.Signer.RegisterCommitFunc(cb.CommitFunc(s))
	s.Signer.RegisterDoneFunc(cb.Done(s))
	s.rLock = sync.Mutex{}
	s.maxRounds = PeerMaxRounds

	// listen for client requests at one port higher
	// than the signing node
	h, p, err := net.SplitHostPort(s.Signer.Name())
	if err == nil {
		i, err := strconv.Atoi(p)
		if err != nil {
			log.Fatal(err)
		}
		s.name = net.JoinHostPort(h, strconv.Itoa(i+1))
	}
	s.closeChan = make(chan bool, 5)
	return s
}

// Listen on client connections. If role is root also send annoucement
// for all of the nRounds
func (s *Peer) Run(role string) {
	// defer func() {
	// 	log.Infoln(s.Name(), "CLOSE AFTER RUN")
	// 	s.Close()
	// }()

	dbg.Lvl3("Stamp-server", s.name, "starting with", role)
	closed := make(chan bool, 1)

	go func() { err := s.Signer.Listen(); closed <- true; s.Close(); dbg.Lvl2("Listened and error:", err) }()

	var nextRole string // next role when view changes
	for {
		switch role {

		case "root":
			dbg.Lvl4("running as root")
			nextRole = s.runAsRoot(s.maxRounds)
		case "regular":
			dbg.Lvl4("running as regular")
			nextRole = s.runAsRegular()
		default:
			dbg.Fatal("Unable to run as anything")
			return
		}

		// dbg.Lvl4(s.Name(), "nextRole:", nextRole)
		if nextRole == "close" {
			s.Close()
			return
		}
		if nextRole == "" {
			return
		}
		s.LogReRun(nextRole, role)
		role = nextRole
	}

}
func (s *Peer) runAsRoot(nRounds int) string {
	// every 5 seconds start a new round
	ticker := time.Tick(ROUND_TIME)
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

func (s *Peer) runAsRegular() string {
	select {
	case <-s.closeChan:
		dbg.Lvl3("server", s.Name(), "has closed the connection")
		return ""

	case nextRole := <-s.ViewChangeCh():
		return nextRole
	}
}

func (s *Peer) Close() {
	dbg.Lvl4("closing stampserver: %p", s.name)
	s.closeChan <- true
	s.Signer.Close()
}

// listen for clients connections
// this server needs to be running on a different port
// than the Signer that is beneath it
func (s *Peer) Listen() error {
	return s.Cb.Listen(s)
}

func (s *Peer) ConnectToLogger() {
	return
	if s.Logger == "" || s.Hostname == "" || s.App == "" {
		dbg.Lvl4("skipping connect to logger")
		return
	}
	dbg.Lvl4("Connecting to Logger")
	lh, _ := logutils.NewLoggerHook(s.Logger, s.Hostname, s.App)
	dbg.Lvl4("Connected to Logger")
	log.AddHook(lh)
}

func (s *Peer) LogReRun(nextRole string, curRole string) {
	if nextRole == "root" {
		var messg = s.Name() + "became root"
		if curRole == "root" {
			messg = s.Name() + "remained root"
		}

		go s.ConnectToLogger()

		log.WithFields(log.Fields{
			"file": logutils.File(),
			"type": "role_change",
		}).Infoln(messg)
		// dbg.Lvl4("role change: %p", s)

	} else {
		var messg = s.Name() + "remained regular"
		if curRole == "root" {
			messg = s.Name() + "became regular"
		}

		if curRole == "root" {
			log.WithFields(log.Fields{
				"file": logutils.File(),
				"type": "role_change",
			}).Infoln(messg)
			dbg.Lvl4("role change: %p", s)
		}

	}

}
