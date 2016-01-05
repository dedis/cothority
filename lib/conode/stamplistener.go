package conode

import (
	"github.com/dedis/cothority/lib/cliutils"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/crypto/abstract"
	"golang.org/x/net/context"
	"net"
	"os"
	"strconv"
	"sync"
)

const (
	READING = iota
	PROCESSING
)

/*
The counterpart to stamp.go - it listens for incoming requests
and passes those to the roundstamper.
*/

func init() {
	SLList = make(map[string]*StampListener)
}

var SLList map[string]*StampListener

type StampListener struct {
	network.Host
	// for aggregating messages from clients
	Mux   sync.Mutex
	Queue [][]MustReplyMessage
	// All clients connected to that listener
	Clients map[string]network.Conn
	// The name of the listener
	NameL string
	// The channel for closing the connection
	waitClose chan string
	// The port we're listening on
	Port net.Listener
}

// Creates a new stamp listener one port above the
// address given in nameP
func NewStampListener(nameP string, suite abstract.Suite) *StampListener {
	// listen for client requests at one port higher
	// than the signing node
	var nameL string
	h, p, err := net.SplitHostPort(nameP)
	if err == nil {
		i, err := strconv.Atoi(p)
		if err != nil {
			dbg.Fatal(err)
		}
		nameL = net.JoinHostPort(h, strconv.Itoa(i+1))
	} else {
		dbg.Fatal("Couldn't split host into name and port:", err)
	}
	sl, ok := SLList[nameL]
	if !ok {
		sl = &StampListener{}
		dbg.Lvl3("Creating new StampListener for", nameL)
		sl.Queue = make([][]MustReplyMessage, 2)
		sl.Queue[READING] = make([]MustReplyMessage, 0)
		sl.Queue[PROCESSING] = make([]MustReplyMessage, 0)
		sl.Clients = make(map[string]network.Conn)
		sl.waitClose = make(chan string)
		sl.NameL = nameL
		sl.Host = network.NewTcpHost(nameL, network.DefaultConstructors(suite))
		SLList[sl.NameL] = sl
		sl.ListenRequests()
	} else {
		dbg.Lvl3("Taking cached StampListener")
	}
	return sl
}

// listen for clients connections
func (s *StampListener) ListenRequests() error {
	dbg.Lvl3("Setup StampListener on", s.NameL)
	global, _ := cliutils.GlobalBind(s.NameL)
	go s.Listen(global, func(c network.Conn) {
		dbg.Lvlf2("StampLister new connection from %s", c.Remote())
		if _, ok := s.Clients[c.Remote()]; !ok {
			s.Clients[c.Remote()] = c
			for {
				ctx := context.TODO()
				am, err := c.Receive(ctx)
				if err != nil {
					dbg.Lvl2(s.Name(), " error receiving client message:", err)
					if err == network.ErrClosed || err == network.ErrUnknown || err == network.ErrEOF {
						dbg.Lvl2("Stamplistener", s.Name(), "Abort client connection")
						return
					}
					continue
				}
				switch am.MsgType {
				case StampRequestType:
					s.Mux.Lock()
					stampRequest := am.Msg.(StampRequest)
					s.Queue[READING] = append(s.Queue[READING],
						MustReplyMessage{Tsm: stampRequest, To: c.Remote()})
					s.Mux.Unlock()
				case StampCloseType:
					dbg.Lvl2("Closing connection")
					c.Close()
				case StampExitType:
					dbg.Lvl2("Exiting server upon request")
					os.Exit(-1)
				default:
					c.Close()
					dbg.Lvl2("Received unexpected packet from", c.Remote(), ". Abort")
					return

				}
			}
		}
	})
	return nil
}

// Close shuts down the connection
func (s *StampListener) Close() {
	s.Host.Close()
	delete(SLList, s.NameL)
	dbg.Lvl3(s.NameL, "Closing stamplistener done - SLList is", SLList)
}

// StampListenersClose closes all open stamplisteners
func StampListenersClose() {
	for _, s := range SLList {
		s.Close()
	}
}
