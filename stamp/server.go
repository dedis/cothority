package stamp

import (
	"net"
	"strconv"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/ineiti/cothorities/coconet"
	"github.com/ineiti/cothorities/hashid"
	"github.com/ineiti/cothorities/proof"
	"github.com/ineiti/cothorities/sign"
	"github.com/ineiti/cothorities/helpers/logutils"
)

type Server struct {
	sign.Signer
	name    string
	Clients map[string]coconet.Conn

	// for aggregating messages from clients
	mux        sync.Mutex
	Queue      [][]MustReplyMessage
	READING    int
	PROCESSING int

	// Leaves, Root and Proof for a round
	Leaves []hashid.HashId // can be removed after we verify protocol
	Root   hashid.HashId
	Proofs []proof.Proof

	rLock     sync.Mutex
	maxRounds int
	closeChan chan bool

	Logger   string
	Hostname string
	App      string
}

func NewServer(signer sign.Signer) *Server {
	s := &Server{}

	s.Clients = make(map[string]coconet.Conn)
	s.Queue = make([][]MustReplyMessage, 2)
	s.READING = 0
	s.PROCESSING = 1

	s.Signer = signer
	s.Signer.RegisterAnnounceFunc(s.OnAnnounce())
	s.Signer.RegisterDoneFunc(s.OnDone())
	s.rLock = sync.Mutex{}

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
	s.Queue[s.READING] = make([]MustReplyMessage, 0)
	s.Queue[s.PROCESSING] = make([]MustReplyMessage, 0)
	s.closeChan = make(chan bool, 5)
	return s
}

var clientNumber int = 0

func (s *Server) Close() {
	log.Printf("closing stampserver: %p", s)
	s.closeChan <- true
	s.Signer.Close()
}

// listen for clients connections
// this server needs to be running on a different port
// than the Signer that is beneath it
func (s *Server) Listen() error {
	// log.Println("Listening @ ", s.name)
	ln, err := net.Listen("tcp4", s.name)
	if err != nil {
		panic(err)
	}

	go func() {
		for {
			// log.Printf("LISTENING TO CLIENTS: %p", s)
			conn, err := ln.Accept()
			if err != nil {
				// handle error
				log.Errorln("failed to accept connection")
				continue
			}

			c := coconet.NewTCPConnFromNet(conn)
			// log.Println("CLIENT TCP CONNECTION SUCCESSFULLY ESTABLISHED:", c)

			if _, ok := s.Clients[c.Name()]; !ok {
				s.Clients[c.Name()] = c

				go func(c coconet.Conn) {
					for {
						tsm := TimeStampMessage{}
						err := c.Get(&tsm)
						if err != nil {
							log.Errorf("%p Failed to get from child:", s, err)
							s.Close()
							return
						}
						switch tsm.Type {
						default:
							log.Errorf("Message of unknown type: %v\n", tsm.Type)
						case StampRequestType:
							// log.Println("RECEIVED STAMP REQUEST")
							s.mux.Lock()
							READING := s.READING
							s.Queue[READING] = append(s.Queue[READING],
								MustReplyMessage{Tsm: tsm, To: c.Name()})
							s.mux.Unlock()
						}
					}
				}(c)
			}

		}
	}()

	return nil
}

// Used for goconns
// should only be used if clients are created in batch
func (s *Server) ListenToClients() {
	// log.Printf("LISTENING TO CLIENTS: %p", s, s.Clients)
	for _, c := range s.Clients {
		go func(c coconet.Conn) {
			for {
				tsm := TimeStampMessage{}
				err := c.Get(&tsm)
				if err == coconet.ErrClosed {
					log.Errorf("%p Failed to get from client:", s, err)
					s.Close()
					return
				}
				if err != nil {
					log.WithFields(log.Fields{
						"file": logutils.File(),
					}).Errorf("%p failed To get message:", s, err)
				}
				switch tsm.Type {
				default:
					log.Errorln("Message of unknown type")
				case StampRequestType:
					// log.Println("STAMP REQUEST")
					s.mux.Lock()
					READING := s.READING
					s.Queue[READING] = append(s.Queue[READING],
						MustReplyMessage{Tsm: tsm, To: c.Name()})
					s.mux.Unlock()
				}
			}
		}(c)
	}
}

func (s *Server) ConnectToLogger() {
	return
	if s.Logger == "" || s.Hostname == "" || s.App == "" {
		log.Println("skipping connect to logger")
		return
	}
	log.Println("Connecting to Logger")
	lh, _ := logutils.NewLoggerHook(s.Logger, s.Hostname, s.App)
	log.Println("Connected to Logger")
	log.AddHook(lh)
}

func (s *Server) LogReRun(nextRole string, curRole string) {
	if nextRole == "root" {
		var messg = s.Name() + " became root"
		if curRole == "root" {
			messg = s.Name() + " remained root"
		}

		go s.ConnectToLogger()

		log.WithFields(log.Fields{
			"file": logutils.File(),
			"type": "role_change",
		}).Infoln(messg)
		// log.Printf("role change: %p", s)

	} else {
		var messg = s.Name() + " remained regular"
		if curRole == "root" {
			messg = s.Name() + " became regular"
		}

		if curRole == "root" {
			log.WithFields(log.Fields{
				"file": logutils.File(),
				"type": "role_change",
			}).Infoln(messg)
			log.Printf("role change: %p", s)
		}

	}

}

func (s *Server) runAsRoot(nRounds int) string {
	// every 5 seconds start a new round
	ticker := time.Tick(ROUND_TIME)
	if s.LastRound()+1 > nRounds {
		log.Errorln(s.Name(), "runAsRoot called with too large round number")
		return "close"
	}

	log.Infoln(s.Name(), "running as root", s.LastRound(), int64(nRounds))
	for {
		select {
		case nextRole := <-s.ViewChangeCh():
			log.Println(s.Name(), "assuming next role")
			return nextRole
			// s.reRunWith(nextRole, nRounds, true)
		case <-ticker:

			start := time.Now()
			log.Println(s.Name(), "is STAMP SERVER STARTING SIGNING ROUND FOR:", s.LastRound()+1, "of", nRounds)

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
				log.Errorln(err)
				time.Sleep(1 * time.Second)
				break
			}

			if s.LastRound()+1 >= nRounds {
				log.Errorln(s.Name(), "reports exceeded the max round: terminating", s.LastRound()+1, ">=", nRounds)
				return "close"
			}

			elapsed := time.Since(start)
			log.WithFields(log.Fields{
				"file":  logutils.File(),
				"type":  "root_round",
				"round": s.LastRound(),
				"time":  elapsed,
			}).Info("root round")

		}
	}
}

func (s *Server) runAsRegular() string {
	select {
	case <-s.closeChan:
		log.WithFields(log.Fields{
			"file": logutils.File(),
			"type": "close",
		}).Infoln("server" + s.Name() + "has closed")
		return ""

	case nextRole := <-s.ViewChangeCh():
		return nextRole
	}
}

// Listen on client connections. If role is root also send annoucement
// for all of the nRounds
func (s *Server) Run(role string, nRounds int) {
	// defer func() {
	// 	log.Infoln(s.Name(), "CLOSE AFTER RUN")
	// 	s.Close()
	// }()

	log.Println("Stamp-server starting with ", role, "and rounds", nRounds)
	closed := make(chan bool, 1)

	go func() { err := s.Signer.Listen(); closed <- true; s.Close(); log.Error(err) }()
	if role == "test_connect" {
		role = "regular"
		go func() {
			//time.Sleep(30 * time.Second)
			hostlist := s.Hostlist()
			ticker := time.Tick(15 * time.Second)
			i := 0
			for _ = range ticker {
				select {
				case <-closed:
					log.Println("server.Run: received closed")
					return
				default:
				}
				if i%2 == 0 {
					log.Println("removing self")
					s.Signer.RemoveSelf()
				} else {
					log.Println("adding self: ", hostlist[(i/2)%len(hostlist)])
					s.Signer.AddSelf(hostlist[(i/2)%len(hostlist)])
				}
				i++
			}
		}()
	}
	s.rLock.Lock()
	s.maxRounds = nRounds
	s.rLock.Unlock()

	var nextRole string // next role when view changes
	for {
		switch role {

		case "root":
			log.Println("running as root")
			nextRole = s.runAsRoot(nRounds)
		case "regular":
			log.Println("running as regular")
			nextRole = s.runAsRegular()
		case "test":
			log.Println("running as test")
			ticker := time.Tick(2000 * time.Millisecond)
			for _ = range ticker {
				s.AggregateCommits(0)
			}
		default:
			log.Println("UNABLE TO RUN AS ANYTHING")
			return
		}

		// log.Println(s.Name(), "nextRole: ", nextRole)
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

func (s *Server) OnAnnounce() sign.CommitFunc {
	return func(view int) []byte {
		//log.Println("Aggregating Commits")
		return s.AggregateCommits(view)
	}
}

func (s *Server) OnDone() sign.DoneFunc {
	return func(view int, SNRoot hashid.HashId, LogHash hashid.HashId, p proof.Proof) {
		s.mux.Lock()
		for i, msg := range s.Queue[s.PROCESSING] {
			// proof to get from s.Root to big root
			combProof := make(proof.Proof, len(p))
			copy(combProof, p)

			// add my proof to get from a leaf message to my root s.Root
			combProof = append(combProof, s.Proofs[i]...)

			// proof that i can get from a leaf message to the big root
			if sign.DEBUG == true {
				proof.CheckProof(s.Signer.(*sign.Node).Suite().Hash, SNRoot, s.Leaves[i], combProof)
			}

			respMessg := TimeStampMessage{
				Type:  StampReplyType,
				ReqNo: msg.Tsm.ReqNo,
				Srep:  &StampReply{Sig: SNRoot, Prf: combProof}}

			s.PutToClient(msg.To, respMessg)
		}
		s.mux.Unlock()
	}

}

func (s *Server) AggregateCommits(view int) []byte {
	//log.Println(s.Name(), "calling AggregateCommits")
	s.mux.Lock()
	// get data from s once to avoid refetching from structure
	Queue := s.Queue
	READING := s.READING
	PROCESSING := s.PROCESSING
	// messages read will now be processed
	READING, PROCESSING = PROCESSING, READING
	s.READING, s.PROCESSING = s.PROCESSING, s.READING
	s.Queue[READING] = s.Queue[READING][:0]

	// give up if nothing to process
	if len(Queue[PROCESSING]) == 0 {
		s.mux.Unlock()
		s.Root = make([]byte, hashid.Size)
		s.Proofs = make([]proof.Proof, 1)
		return s.Root
	}

	// pull out to be Merkle Tree leaves
	s.Leaves = make([]hashid.HashId, 0)
	for _, msg := range Queue[PROCESSING] {
		s.Leaves = append(s.Leaves, hashid.HashId(msg.Tsm.Sreq.Val))
	}
	s.mux.Unlock()

	// non root servers keep track of rounds here
	if !s.IsRoot(view) {
		s.rLock.Lock()
		lsr := s.LastRound()
		mr := s.maxRounds
		s.rLock.Unlock()
		// if this is our last round then close the connections
		if lsr >= mr && mr >= 0 {
			s.closeChan <- true
		}
	}

	// create Merkle tree for this round's messages and check corectness
	s.Root, s.Proofs = proof.ProofTree(s.Suite().Hash, s.Leaves)
	if sign.DEBUG == true {
		if proof.CheckLocalProofs(s.Suite().Hash, s.Root, s.Leaves, s.Proofs) == true {
			log.Println("Local Proofs of", s.Name(), "successful for round "+strconv.Itoa(int(s.LastRound())))
		} else {
			panic("Local Proofs" + s.Name() + " unsuccessful for round " + strconv.Itoa(int(s.LastRound())))
		}
	}

	return s.Root
}

// Send message to client given by name
func (s *Server) PutToClient(name string, data coconet.BinaryMarshaler) {
	err := s.Clients[name].Put(data)
	if err == coconet.ErrClosed {
		s.Close()
		return
	}
	if err != nil && err != coconet.ErrNotEstablished {
		log.Warnf("%p error putting to client: %v", s, err)
	}
}
