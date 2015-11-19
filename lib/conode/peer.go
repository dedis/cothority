package conode

import (
	"net"
	"strconv"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	dbg "github.com/dedis/cothority/lib/debug_lvl"

	"github.com/dedis/cothority/lib/cliutils"
	"github.com/dedis/cothority/lib/coconet"
	"github.com/dedis/cothority/lib/sign"
	"os"
)

const (
	READING = iota
	PROCESSING
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

	Clients map[string]coconet.Conn

	// for aggregating messages from clients
	Mux   sync.Mutex
	Queue [][]MustReplyMessage
}

// NewPeer returns a peer that can be used to set up
// connections.
func NewPeer(node *sign.Node) *Peer {
	s := &Peer{}

	s.Node = node
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
	s.Queue = make([][]MustReplyMessage, 2)
	s.Queue[READING] = make([]MustReplyMessage, 0)
	s.Queue[PROCESSING] = make([]MustReplyMessage, 0)
	s.Clients = make(map[string]coconet.Conn)
	s.Node = node
	return s
}

// listen for clients connections
func (s *Peer) Setup() error {
	dbg.Lvl3("Setup Peer")
	global, _ := cliutils.GlobalBind(s.NameP)
	dbg.Lvl3("Listening in server at", global)
	ln, err := net.Listen("tcp4", global)
	if err != nil {
		panic(err)
	}

	go func() {
		for {
			dbg.Lvl2("Listening to sign-requests: %p", s)
			conn, err := ln.Accept()
			if err != nil {
				// handle error
				dbg.Lvl3("failed to accept connection")
				continue
			}

			c := coconet.NewTCPConnFromNet(conn)
			dbg.Lvl2("Established connection with client:", c)

			if _, ok := s.Clients[c.Name()]; !ok {
				s.Clients[c.Name()] = c

				go func(co coconet.Conn) {
					for {
						tsm := TimeStampMessage{}
						err := co.GetData(&tsm)
						dbg.Lvl2("Got data to sign %+v - %+v", tsm, tsm.Sreq)
						if err != nil {
							dbg.Lvlf1("%p Failed to get from child: %s", s.NameP, err)
							co.Close()
							return
						}
						switch tsm.Type {
						default:
							dbg.Lvlf1("Message of unknown type: %v\n", tsm.Type)
						case StampRequestType:
							s.Mux.Lock()
							s.Queue[READING] = append(s.Queue[READING],
								MustReplyMessage{Tsm: tsm, To: co.Name()})
							s.Mux.Unlock()
						case StampClose:
							dbg.Lvl2("Closing connection")
							co.Close()
							return
						case StampExit:
							dbg.Lvl2("Exiting server upon request")
							os.Exit(-1)
						}
					}
				}(c)
			}
		}
	}()

	return nil
}

// Listen on client connections. If role is root also send annoucement
// for all of the nRounds
func (s *Peer) Run(role string) {
	dbg.Lvl3("Stamp-server", s.NameP, "starting with ", role)
	fn := func(*sign.Node) sign.Round {
		return NewRoundStamper(s)
	}
	sign.RegisterRoundFactory(RoundStamperType, fn)
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
			err = s.StartAnnouncement(NewRoundStamper(s))
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
