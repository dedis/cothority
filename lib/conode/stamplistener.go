package conode
import (
	"github.com/dedis/cothority/lib/cliutils"
	"net"
	"github.com/dedis/cothority/lib/coconet"
	"os"
	"github.com/dedis/cothority/lib/dbg"
	"sync"
	"strconv"
	"log"
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
	// for aggregating messages from clients
	Mux       sync.Mutex
	Queue     [][]MustReplyMessage
	// All clients connected to that listener
	Clients   map[string]coconet.Conn
	// The name of the listener
	NameP     string
	// The channel for closing the connection
	waitClose chan string
	// The port we're listening on
	Port      net.Listener
}

func NewStampListener(name string) *StampListener {
	sl, ok := SLList[name]
	if !ok {
		dbg.LLvl3("Creating new StampListener for", name)
		sl = &StampListener{}
		sl.Queue = make([][]MustReplyMessage, 2)
		sl.Queue[READING] = make([]MustReplyMessage, 0)
		sl.Queue[PROCESSING] = make([]MustReplyMessage, 0)
		sl.Clients = make(map[string]coconet.Conn)
		sl.waitClose = make(chan string)

		// listen for client requests at one port higher
		// than the signing node
		h, p, err := net.SplitHostPort(name)
		if err == nil {
			i, err := strconv.Atoi(p)
			if err != nil {
				log.Fatal(err)
			}
			sl.NameP = net.JoinHostPort(h, strconv.Itoa(i + 1))
		} else {
			log.Fatal("Couldn't split host into name and port:", err)
		}
		SLList[name] = sl
		sl.ListenRequests()
	}
	return sl
}

// listen for clients connections
func (s *StampListener) ListenRequests() error {
	dbg.Lvl3("Setup Peer")
	global, _ := cliutils.GlobalBind(s.NameP)
	dbg.LLvl3("Listening in server at", global)
	var err error
	s.Port, err = net.Listen("tcp4", global)
	if err != nil {
		panic(err)
	}

	go func() {
		for {
			dbg.LLvl2("Listening to sign-requests: %p", s)
			conn, err := s.Port.Accept()
			if err != nil {
				// handle error
				dbg.LLvl3("failed to accept connection")
				select {
				case w := <-s.waitClose:
					dbg.LLvl3("Closing stamplistener:", w)
					return
				default:
					continue
				}
			}

			dbg.LLvl3("Waiting for connection")
			c := coconet.NewTCPConnFromNet(conn)
			dbg.LLvl2("Established connection with client:", c)

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

// Close shuts down the connection
func (s *StampListener) Close() {
	dbg.LLvl3(s.NameP, "Closing wait channel")
	close(s.waitClose)
	dbg.LLvl3(s.NameP, "Closing stamplistener")
	s.Port.Close()
	dbg.LLvl3(s.NameP, "Closing stamplistener done")
}