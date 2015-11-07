package conode

import (
	"bytes"
	"encoding/binary"
	"strconv"
	"sync"

	log "github.com/Sirupsen/logrus"
	dbg "github.com/dedis/cothority/lib/debug_lvl"

	"github.com/dedis/cothority/lib/coconet"
	"github.com/dedis/cothority/lib/hashid"
	"github.com/dedis/cothority/lib/proof"
	"github.com/dedis/cothority/proto/sign"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/cothority/lib/cliutils"
	"net"
	"os"
	"sync/atomic"
)

type CallbacksStamper struct {
							   // for aggregating messages from clients
	mux        sync.Mutex
	Queue      [][]MustReplyMessage
	READING    int
	PROCESSING int

							   // Leaves, Root and Proof for a round
	Leaves     []hashid.HashId // can be removed after we verify protocol
	Root       hashid.HashId
	Proofs     []proof.Proof
							   // Timestamp message for this Round
	Timestamp  int64

	Clients    map[string]coconet.Conn
	peer       *sign.Peer
	Round      *sign.Round
	RoundNbr   int
}

func NewCallbacksStamper() *CallbacksStamper {
	cbs := &CallbacksStamper{}
	cbs.Queue = make([][]MustReplyMessage, 2)
	cbs.READING = 0
	cbs.PROCESSING = 1
	cbs.Queue[cbs.READING] = make([]MustReplyMessage, 0)
	cbs.Queue[cbs.PROCESSING] = make([]MustReplyMessage, 0)
	cbs.Clients = make(map[string]coconet.Conn)

	return cbs
}

// AnnounceFunc will keep the timestamp generated for this round
func (cs *CallbacksStamper) Announcement(sn *sign.Node, am *sign.AnnouncementMessage) ([]sign.AnnouncementMessage, error) {
	var t int64
	if err := binary.Read(bytes.NewBuffer(am.Message), binary.LittleEndian, &t); err != nil {
		dbg.Lvl1("Unmashaling timestamp has failed")
	}
	cs.Timestamp = t
	cs.RoundNbr = am.RoundNbr

	if err := sn.TryFailure(sn.ViewNo, am.RoundNbr); err != nil {
		return nil, err
	}

	if err := sign.RoundSetup(sn, sn.ViewNo, am); err != nil {
		return nil, err
	}
	// Store the message for the round
	round := sn.Rounds[am.RoundNbr]
	round.Msg = am.Message
	cs.Round = round

	// Inform all children of announcement
	messgs := make([]coconet.BinaryMarshaler, sn.NChildren(sn.ViewNo))
	for i := range messgs {
		sm := sign.SigningMessage{
			Type:         sign.Announcement,
			View:         sn.ViewNo,
			LastSeenVote: int(atomic.LoadInt64(&sn.LastSeenVote)),
			Am:           am}
		messgs[i] = &sm
	}
	return messgs, nil
}

func (cs *CallbacksStamper) Commitment(children []*sign.CommitmentMessage) *sign.CommitmentMessage {
	//dbg.Lvl4(cs.Name(), "calling AggregateCommits")
	cs.mux.Lock()
	// get data from s once to avoid refetching from structure
	Queue := cs.Queue
	READING := cs.READING
	PROCESSING := cs.PROCESSING
	// messages read will now be processed
	READING, PROCESSING = PROCESSING, READING
	cs.READING, cs.PROCESSING = cs.PROCESSING, cs.READING
	cs.Queue[READING] = cs.Queue[READING][:0]

	// give up if nothing to process
	if len(Queue[PROCESSING]) == 0 {
		cs.mux.Unlock()
		cs.Root = make([]byte, hashid.Size)
		cs.Proofs = make([]proof.Proof, 1)
	} else {
		// pull out to be Merkle Tree leaves
		cs.Leaves = make([]hashid.HashId, 0)
		for _, msg := range Queue[PROCESSING] {
			cs.Leaves = append(cs.Leaves, hashid.HashId(msg.Tsm.Sreq.Val))
		}
		cs.mux.Unlock()

		// create Merkle tree for this round's messages and check corectness
		cs.Root, cs.Proofs = proof.ProofTree(cs.peer.Suite().Hash, cs.Leaves)
		if dbg.DebugVisible > 2 {
			if proof.CheckLocalProofs(cs.peer.Suite().Hash, cs.Root, cs.Leaves, cs.Proofs) == true {
				dbg.Lvl4("Local Proofs of", cs.peer.Name(), "successful for round " + strconv.Itoa(int(cs.peer.LastRound())))
			} else {
				panic("Local Proofs" + cs.peer.Name() + " unsuccessful for round " + strconv.Itoa(int(cs.peer.LastRound())))
			}
		}
	}

	return &sign.CommitmentMessage{MTRoot:cs.Root}
}

// Not used dummy-functions
func (cs *CallbacksStamper)    Challenge(*sign.ChallengeMessage) {}
func (cs *CallbacksStamper)    Response(*sign.ResponseMessage) {}

func (cs *CallbacksStamper) SignatureBroadcast(view int, SNRoot hashid.HashId, LogHash hashid.HashId, pr proof.Proof,
sb *sign.SignatureBroadcastMessage, suite abstract.Suite) {
	cs.mux.Lock()
	for i, msg := range cs.Queue[cs.PROCESSING] {
		// proof to get from s.Root to big root
		combProof := make(proof.Proof, len(pr))
		copy(combProof, pr)

		// add my proof to get from a leaf message to my root s.Root
		combProof = append(combProof, cs.Proofs[i]...)

		// proof that I can get from a leaf message to the big root
		if proof.CheckProof(cs.peer.Signer.(*sign.Node).Suite().Hash, SNRoot, cs.Leaves[i], combProof) {
			dbg.Lvl2("Proof is OK")
		} else {
			dbg.Lvl2("Inclusion-proof failed")
		}

		respMessg := &TimeStampMessage{
			Type:  StampReplyType,
			ReqNo: msg.Tsm.ReqNo,
			Srep:  &StampReply{SuiteStr: suite.String(), Timestamp: cs.Timestamp, MerkleRoot: SNRoot, Prf: combProof, SigBroad: *sb}}
		cs.PutToClient(cs.peer, msg.To, respMessg)
		dbg.Lvl2("Sent signature response back to client")
	}
	cs.mux.Unlock()
	cs.Timestamp = 0
}

// Send message to client given by name
func (cs *CallbacksStamper) PutToClient(p *sign.Peer, name string, data coconet.BinaryMarshaler) {
	err := cs.Clients[name].PutData(data)
	if err == coconet.ErrClosed {
		p.Close()
		return
	}
	if err != nil && err != coconet.ErrNotEstablished {
		log.Warnf("%p error putting to client: %v", cs, err)
	}
}

// Starts to listen for stamper-requests
func (cs *CallbacksStamper) Setup(p *sign.Peer) error {
	cs.peer = p
	global, _ := cliutils.GlobalBind(p.NameP)
	dbg.Lvl3("Listening in server at", global)
	ln, err := net.Listen("tcp4", global)
	if err != nil {
		panic(err)
	}

	go func() {
		for {
			dbg.Lvl2("Listening to sign-requests: %p", cs)
			conn, err := ln.Accept()
			if err != nil {
				// handle error
				dbg.Lvl3("failed to accept connection")
				continue
			}

			c := coconet.NewTCPConnFromNet(conn)
			dbg.Lvl2("Established connection with client:", c)

			if _, ok := cs.Clients[c.Name()]; !ok {
				cs.Clients[c.Name()] = c

				go func(co coconet.Conn) {
					for {
						tsm := TimeStampMessage{}
						err := co.GetData(&tsm)
						dbg.Lvl2("Got data to sign %+v - %+v", tsm, tsm.Sreq)
						if err != nil {
							dbg.Lvlf1("%p Failed to get from child: %s", p, err)
							co.Close()
							return
						}
						switch tsm.Type {
						default:
							dbg.Lvlf1("Message of unknown type: %v\n", tsm.Type)
						case StampRequestType:
							cs.mux.Lock()
							READING := cs.READING
							cs.Queue[READING] = append(cs.Queue[READING],
								MustReplyMessage{Tsm: tsm, To: co.Name()})
							cs.mux.Unlock()
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