package conode

import (
	"bytes"
	"encoding/binary"
	"strconv"
	"sync"

	log "github.com/Sirupsen/logrus"
	dbg "github.com/dedis/cothority/lib/debug_lvl"

	"errors"
	"github.com/dedis/cothority/lib/cliutils"
	"github.com/dedis/cothority/lib/coconet"
	"github.com/dedis/cothority/lib/hashid"
	"github.com/dedis/cothority/lib/proof"
	"github.com/dedis/cothority/lib/sign"
	"github.com/dedis/crypto/abstract"
	"net"
	"os"
)

type RoundStamper struct {
	// for aggregating messages from clients
	mux        sync.Mutex
	Queue      [][]MustReplyMessage
	READING    int
	PROCESSING int

	// Leaves, Root and Proof for a round
	Leaves []hashid.HashId // can be removed after we verify protocol
	Root   hashid.HashId
	Proofs []proof.Proof
	// Timestamp message for this Round
	Timestamp int64

	Clients  map[string]coconet.Conn
	peer     *Peer
	Round    *sign.RoundMerkle
	RoundNbr int
}

func NewRoundStamper() *RoundStamper {
	cbs := &RoundStamper{}
	cbs.Queue = make([][]MustReplyMessage, 2)
	cbs.READING = 0
	cbs.PROCESSING = 1
	cbs.Queue[cbs.READING] = make([]MustReplyMessage, 0)
	cbs.Queue[cbs.PROCESSING] = make([]MustReplyMessage, 0)
	cbs.Clients = make(map[string]coconet.Conn)

	return cbs
}

// AnnounceFunc will keep the timestamp generated for this round
func (cs *RoundStamper) Announcement(sn *sign.Node, am *sign.AnnouncementMessage) ([]*sign.AnnouncementMessage, error) {
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

	// Inform all children of announcement - just copy the one that came in
	messgs := make([]*sign.AnnouncementMessage, sn.NChildren(sn.ViewNo))
	for i := range messgs {
		messgs[i] = am
	}
	return messgs, nil
}

func (cs *RoundStamper) Commitment(_ []*sign.CommitmentMessage) *sign.CommitmentMessage {
	// prepare to handle exceptions
	round := cs.Round
	round.ExceptionList = make([]abstract.Point, 0)

	// Create the mapping between children and their respective public key + commitment
	// V for commitment
	children := round.Children
	round.ChildV_hat = make(map[string]abstract.Point, len(children))
	// X for public key
	round.ChildX_hat = make(map[string]abstract.Point, len(children))

	// Commits from children are the first Merkle Tree leaves for the round
	round.Leaves = make([]hashid.HashId, 0)
	round.LeavesFrom = make([]string, 0)

	for key := range children {
		round.ChildX_hat[key] = round.Suite.Point().Null()
		round.ChildV_hat[key] = round.Suite.Point().Null()
	}

	commits := make([]*sign.CommitmentMessage, len(children))
	for _, sm := range round.Commits {
		from := sm.From
		commits = append(commits, sm.Com)
		// MTR ==> root of sub-merkle tree
		round.Leaves = append(round.Leaves, sm.Com.MTRoot)
		round.LeavesFrom = append(round.LeavesFrom, from)
		round.ChildV_hat[from] = sm.Com.V_hat
		round.ChildX_hat[from] = sm.Com.X_hat
		round.ExceptionList = append(round.ExceptionList, sm.Com.ExceptionList...)

		// Aggregation
		// add good child server to combined public key, and point commit
		round.Add(round.X_hat, sm.Com.X_hat)
		round.Add(round.Log.V_hat, sm.Com.V_hat)
		//dbg.Lvl4("Adding aggregate public key from ", from, " : ", sm.Com.X_hat)
	}

	dbg.Lvl4("sign.Node.Commit using Merkle")
	round.MerkleAddChildren()
	// compute the local Merkle root

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
		cs.Root, cs.Proofs = proof.ProofTree(cs.Round.Suite.Hash, cs.Leaves)
		if dbg.DebugVisible > 2 {
			if proof.CheckLocalProofs(cs.Round.Suite.Hash, cs.Root, cs.Leaves, cs.Proofs) == true {
				dbg.Lvl4("Local Proofs of", cs.peer.Name(), "successful for round "+strconv.Itoa(int(cs.peer.LastRound())))
			} else {
				panic("Local Proofs" + cs.peer.Name() + " unsuccessful for round " + strconv.Itoa(int(cs.peer.LastRound())))
			}
		}
	}

	round.MerkleAddLocal(cs.Root)
	round.MerkleHashLog()
	round.ComputeCombinedMerkleRoot()
	msg := round.Msg
	msg = append(msg, []byte(round.MTRoot)...)
	round.C = sign.HashElGamal(round.Suite, msg, round.Log.V_hat)

	round.Proof = make([]hashid.HashId, 0)

	return &sign.CommitmentMessage{MTRoot: cs.Root}
}

func (cs *RoundStamper) Challenge(chm *sign.ChallengeMessage) error {
	// register challenge
	cs.Round.C = chm.C
	cs.Round.InitResponseCrypto()
	dbg.Lvl4("challenge: using merkle proofs")
	// messages from clients, proofs computed
	if cs.Round.Log.Getv() != nil {
		if err := cs.Round.StoreLocalMerkleProof(chm); err != nil {
			return err
		}

	}
	return nil
}

func (cs *RoundStamper) Response(sms []*sign.SigningMessage) error {
	// initialize exception handling
	exceptionV_hat := cs.Round.Suite.Point().Null()
	exceptionX_hat := cs.Round.Suite.Point().Null()
	cs.Round.ExceptionList = make([]abstract.Point, 0)
	nullPoint := cs.Round.Suite.Point().Null()

	children := cs.Round.Children
	for _, sm := range sms {
		from := sm.From
		switch sm.Type {
		default:
			// default == no response from child
			// dbg.Lvl4(sn.Name(), "default in respose for child", from, sm)
			if children[from] != nil {
				cs.Round.ExceptionList = append(cs.Round.ExceptionList, children[from].PubKey())

				// remove public keys and point commits from subtree of failed child
				cs.Round.Add(exceptionX_hat, cs.Round.ChildX_hat[from])
				cs.Round.Add(exceptionV_hat, cs.Round.ChildV_hat[from])
			}
			continue
		case sign.Response:
			// disregard response from children who did not commit
			_, ok := cs.Round.ChildV_hat[from]
			if ok == true && cs.Round.ChildV_hat[from].Equal(nullPoint) {
				continue
			}

			// dbg.Lvl4(sn.Name(), "accepts response from", from, sm.Type)
			cs.Round.R_hat.Add(cs.Round.R_hat, sm.Rm.R_hat)

			cs.Round.Add(exceptionV_hat, sm.Rm.ExceptionV_hat)

			cs.Round.Add(exceptionX_hat, sm.Rm.ExceptionX_hat)
			cs.Round.ExceptionList = append(cs.Round.ExceptionList, sm.Rm.ExceptionList...)

		case sign.Error:
			if sm.Err == nil {
				log.Errorln("Error message with no error")
				continue
			}

			// Report up non-networking error, probably signature failure
			log.Errorln(cs.Round.Name, "Error in respose for child", from, sm)
			err := errors.New(sm.Err.Err)
			return err
		}
	}

	// remove exceptions from subtree that failed
	cs.Round.Sub(cs.Round.X_hat, exceptionX_hat)
	cs.Round.ExceptionV_hat = exceptionV_hat
	cs.Round.ExceptionX_hat = exceptionX_hat

	dbg.Lvl4(cs.Round.Name, "got all responses")
	err := cs.Round.VerifyResponses()

	return err
}

func (cs *RoundStamper) SignatureBroadcast(sb *sign.SignatureBroadcastMessage) {
	cs.mux.Lock()
	for i, msg := range cs.Queue[cs.PROCESSING] {
		// proof to get from s.Root to big root
		combProof := make(proof.Proof, len(cs.Round.Proof))
		copy(combProof, cs.Round.Proof)

		// add my proof to get from a leaf message to my root s.Root
		combProof = append(combProof, cs.Proofs[i]...)

		// proof that I can get from a leaf message to the big root
		if proof.CheckProof(cs.Round.Suite.Hash, cs.Round.MTRoot,
			cs.Leaves[i], combProof) {
			dbg.Lvl2("Proof is OK")
		} else {
			dbg.Lvl2("Inclusion-proof failed")
		}

		respMessg := &TimeStampMessage{
			Type:  StampSignatureType,
			ReqNo: msg.Tsm.ReqNo,
			Srep: &StampSignature{
				SuiteStr:   cs.Round.Suite.String(),
				Timestamp:  cs.Timestamp,
				MerkleRoot: cs.Round.MTRoot,
				Prf:        combProof,
				Response:   sb.R0_hat,
				Challenge:  sb.C,
				AggCommit:  sb.V0_hat,
				AggPublic:  sb.X0_hat,
			}}
		cs.PutToClient(msg.To, respMessg)
		dbg.Lvl2("Sent signature response back to client")
	}
	cs.mux.Unlock()
	cs.Timestamp = 0
}

// Send message to client given by name
func (cs *RoundStamper) PutToClient(name string, data coconet.BinaryMarshaler) {
	err := cs.Clients[name].PutData(data)
	if err == coconet.ErrClosed {
		cs.Clients[name].Close()
		return
	}
	if err != nil && err != coconet.ErrNotEstablished {
		log.Warnf("%p error putting to client: %v", cs, err)
	}
}

// Starts to listen for stamper-requests
func (cs *RoundStamper) Setup(address string) error {
	global, _ := cliutils.GlobalBind(address)
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
							dbg.Lvlf1("%p Failed to get from child: %s", address, err)
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
