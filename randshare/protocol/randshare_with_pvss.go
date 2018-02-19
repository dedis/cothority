package randsharepvss

import (
	"bytes"
	"errors"
	"fmt"

	"encoding/binary"
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/share"
	"github.com/dedis/kyber/share/pvss"
	"github.com/dedis/onet"
	//"gopkg.in/dedis/onet.v1"
)

func init() {
	onet.GlobalProtocolRegister("RandShare", NewRandShare)
}

//NewRandShare initialises the tree and network
func NewRandShare(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	t := &RandShare{
		TreeNodeInstance: n,
	}
	err := t.RegisterHandlers(t.HandleA1, t.HandleV1, t.HandleR1)
	return t, err
}

//Setup initializes RandShare struct, computes the private keys and the second base point based on the sessionID
func (rs *RandShare) Setup(nodes int, faulty int, purpose string, time int64) error {

	rs.startingTime = time
	rs.nodes = nodes
	rs.nPrime = 0
	rs.faulty = faulty
	rs.threshold = faulty + 1
	rs.purpose = purpose
	rs.X = rs.Roster().Publics()

	rs.sessionID = SessionID(rs.Suite(), rs.nodes, rs.faulty, rs.X, rs.purpose, time)
	rs.H = rs.Suite().Point().Pick(rs.Suite().RandomStream())

	rs.pubPolys = make([]*share.PubPoly, rs.nodes)
	rs.encShares = make(map[int]map[int]*pvss.PubVerShare)
	rs.tracker = make(map[int]int)
	rs.votes = make(map[int]*Vote)
	rs.decShares = make(map[int]map[int]*pvss.PubVerShare)
	for i := 0; i < rs.nodes; i++ {
		rs.encShares[i] = make(map[int]*pvss.PubVerShare)
		rs.decShares[i] = make(map[int]*pvss.PubVerShare)
		rs.votes[i] = &Vote{Voted: false, Vote: 0}
	}
	rs.secrets = make(map[int]kyber.Point)
	rs.coStringReady = false
	rs.Done = make(chan bool, 0)

	return nil
}

//Start initiates the protocol from node 0
func (rs *RandShare) Start() error {

	encShares, pubPoly, err := pvss.EncShares(rs.Suite().(pvss.Suite), rs.H, rs.X, nil, rs.threshold)
	if err != nil {
		return err
	}
	rs.mutex.Lock()
	rs.pubPolys[rs.Index()] = pubPoly
	b, commits := pubPoly.Info()

	announce := &A1{
		SessionID: rs.sessionID,
		Src:       rs.Index(),
		Shares:    encShares,
		B:         b,
		Commits:   commits,
		Purpose:   rs.purpose,
		Time:      rs.startingTime,
	}

	for j := 0; j < rs.nodes; j++ {
		//we know they are correct, we can store them, put the tracker to 1
		rs.encShares[rs.Index()][j] = encShares[j]
		rs.tracker[rs.Index()] = 1
	}
	rs.mutex.Unlock()

	errs := rs.Broadcast(announce) //[]err TODO
	if len(errs) > 0 {
		return errors.New("Start pb")
	}
	return nil
}

//HandleA1 handles the announces of the session
func (rs *RandShare) HandleA1(announce StructA1) error {

	msg := &announce.A1

	if rs.nodes == 0 { //we need to setup rs and brodcast our encrypted shares
		rs.mutex.Lock()
		nodes := len(rs.List())
		if err := rs.Setup(nodes, nodes/3, msg.Purpose, msg.Time); err != nil {
			return err
		}
		encShares, pubPoly, err := pvss.EncShares(rs.Suite(), rs.H, rs.X, nil, rs.threshold)
		if err != nil {
			return err
		}
		rs.pubPolys[rs.Index()] = pubPoly
		b, commits := pubPoly.Info()
		announce := &A1{
			SessionID: rs.sessionID,
			Src:       rs.Index(),
			Shares:    encShares,
			B:         b,
			Commits:   commits,
			Purpose:   rs.purpose,
			Time:      rs.startingTime,
		}
		for j := 0; j < rs.nodes; j++ {
			//we know they are correct, we can store them
			rs.encShares[rs.Index()][j] = encShares[j]
			rs.tracker[rs.Index()] = 1
		}
		rs.mutex.Unlock()

		errs := rs.Broadcast(announce) //[]err TODO
		if len(errs) > 0 {
			return errors.New("Start pb")
		}

	}

	if _, ok := rs.tracker[msg.Src]; ok || !bytes.Equal(msg.SessionID, rs.sessionID) {
		return nil //If the sessionID is not correct or we already got shares from that sender we don't deal with the announce
	}

	rs.mutex.Lock()
	pubPolySrc := share.NewPubPoly(rs.Suite(), msg.B, msg.Commits)
	rs.pubPolys[msg.Src] = pubPolySrc
	//rs.mutex.Unlock()
	for _, share := range msg.Shares {
		shareIndex := share.S.I
		value := pubPolySrc.Eval(shareIndex).V
		_, ok := rs.tracker[msg.Src]
		if err := pvss.VerifyEncShare(rs.Suite(), rs.H, rs.X[shareIndex], value, share); err == nil && !ok {
			//share is correct, we store it in the encShares map
			//	rs.mutex.Lock()
			rs.encShares[msg.Src][shareIndex] = share
			//	rs.mutex.Unlock()
		}

		if len(rs.encShares[msg.Src]) > 2*rs.faulty {
			//	rs.mutex.Lock()
			rs.tracker[msg.Src] = 1
			//	rs.mutex.Unlock()
		}

		if len(rs.tracker) == rs.nodes { //we had announce from everyone
			for index, vote := range rs.tracker {
				if vote == 1 { //we have at least 2*rs.faulty correct encrypted shares for that index
					//rs.mutex.Lock()
					rs.votes[index].Vote = 1
					//rs.mutex.Unlock()
				}
			}
			rs.votes[rs.Index()].Voted = true
			//we say that we are done by sending our votes
			step := &V1{SessionID: rs.sessionID, Src: rs.Index(), Votes: rs.votes}

			errs := rs.Broadcast(step) //[]err
			if len(errs) > 0 {
				return errors.New("Start pb")
			}
		}
	}
	rs.mutex.Unlock()
	return nil
}

//HandleV1 sends the decrypted shares when has received everyone's vote (means they are done storing their encrypted shares)
func (rs *RandShare) HandleV1(step StructV1) error {

	msg := &step.V1

	if !bytes.Equal(msg.SessionID, rs.sessionID) || rs.votes[msg.Src].Voted {
		return nil //If the sessionID is not correct or we already have a vote from that node we don't deal with the message
	}

	rs.mutex.Lock()
	for index, vote := range msg.Votes {
		rs.votes[index].Vote += vote.Vote
	}
	rs.votes[msg.Src].Voted = true
	rs.mutex.Unlock()
	for _, vote := range rs.votes {
		if !vote.Voted { //one node hasn't voted yet
			return nil
		}
	}

	//if we reach this step, everyone voted so we can
	//Compute the number n' of good nodes (thos with a vote greater than faulty), clean our tracker to use it in the next step and brodcast our shares
	for _, vote := range rs.votes {
		if vote.Vote > rs.faulty { //good node
			rs.mutex.Lock()
			rs.nPrime++
			rs.mutex.Unlock()
		}
	}

	if rs.nPrime < rs.faulty {
		return errors.New("Too many faulty nodes")
	}
	rs.mutex.Lock()
	rs.tracker = make(map[int]int)
	rs.mutex.Unlock()

	var decShares []*Share //The list we will send
	for j := 0; j < rs.nodes; j++ {
		if encShare, ok := rs.encShares[j][rs.Index()]; ok { //we have an encrypted share, we can thus verify the decryted share
			decShare, err := pvss.DecShare(rs.Suite(), rs.H, rs.X[rs.Index()], rs.pubPolys[j].Eval(rs.Index()).V, rs.Private(), encShare)
			if err != nil {
				return err
			}
			//our shares are correct we store them and add them to the shares we'll send
			rs.mutex.Lock()
			rs.decShares[j][rs.Index()] = decShare
			rs.mutex.Unlock()
			decShareStruct := &Share{Row: j, PubVerShare: decShare}
			decShares = append(decShares, decShareStruct)
		}
	}
	reply := &R1{SessionID: rs.sessionID, Src: rs.Index(), Shares: decShares}

	errs := rs.Broadcast(reply) //[]err TODO
	if len(errs) > 0 {
		return errors.New("Start pb")
	}
	return nil
}

//HandleR1 stores the decrypted shares and when we have enough, recovers the secret of good nodes
func (rs *RandShare) HandleR1(reply StructR1) error {

	msg := &reply.R1

	if _, ok := rs.tracker[msg.Src]; ok || !bytes.Equal(msg.SessionID, rs.sessionID) {
		return nil //If the sessionID is not correct or we had decrypted shares from that node already, we don't deal with the reply
	}
	rs.mutex.Lock()
	rs.tracker[msg.Src] = 1 //we received something
	rs.mutex.Unlock()
	for _, shareWr := range msg.Shares {
		if _, ok := rs.secrets[shareWr.Row]; !ok || (rs.votes[shareWr.Row].Vote <= rs.faulty) { //if the share.src-th secret is already recovered or has too many negative votes we don't deal with this share
			if encShare, ok := rs.encShares[shareWr.Row][msg.Src]; ok {
				if err := pvss.VerifyDecShare(rs.Suite(), nil, rs.X[msg.Src], encShare, shareWr.PubVerShare); err == nil {
					rs.mutex.Lock()
					rs.decShares[shareWr.Row][msg.Src] = shareWr.PubVerShare
					rs.mutex.Unlock()
				}

				if len(rs.decShares[shareWr.Row]) == rs.threshold { //we can recover src-th secret
					var encShareList []*pvss.PubVerShare
					var decShareList []*pvss.PubVerShare
					var keys []kyber.Point

					for i := 0; i < rs.nodes; i++ {
						if decShare, ok := rs.decShares[shareWr.Row][i]; ok {
							encShareList = append(encShareList, rs.encShares[shareWr.Row][i]) //we are sure to have an encShare as we verified it
							decShareList = append(decShareList, decShare)
							keys = append(keys, rs.X[i])
						}
					}

					secret, err := pvss.RecoverSecret(rs.Suite(), nil, keys, encShareList, decShareList, rs.threshold, rs.nPrime)
					if err != nil {
						return err
					}

					rs.mutex.Lock()
					rs.secrets[shareWr.Row] = secret
					rs.mutex.Unlock()
				}
			}

			if (len(rs.secrets) == rs.nPrime) && !rs.coStringReady { //we can recover the secret for all good nodes
				coString := rs.Suite().Point().Null()
				for j := range rs.secrets {
					kyber.Point.Add(coString, coString, rs.secrets[j])
				}
				rs.mutex.Lock()
				rs.coString = coString
				rs.mutex.Unlock()
				fmt.Printf("Collective String recovered at node %d %+v\n", rs.Index()+1, coString)
				rs.coStringReady = true
				rs.Done <- true
			}
		}
	}
	return nil
}

//Random returns the collective string created by our protocol and the
//associated transcript so that the secret can be verified by a third party
func (rs *RandShare) Random() ([]byte, *Transcript, error) {
	rs.mutex.Lock()
	defer rs.mutex.Unlock()

	if !rs.coStringReady {
		return nil, nil, errors.New("Not ready")
	}
	rb, err := rs.coString.MarshalBinary()
	if err != nil {
		return nil, nil, err
	}
	transcript := &Transcript{
		SessionID: rs.sessionID,
		Suite:     rs.Suite(),
		Nodes:     rs.nodes,
		Faulty:    rs.faulty,
		Purpose:   rs.purpose,
		Time:      rs.startingTime,
		X:         rs.X,
		H:         rs.H,
		EncShares: rs.encShares,
		DecShares: rs.decShares,
		Votes:     rs.votes,
	}
	return rb, transcript, nil
}

//Verify is a method that verifies that we created the random collective string following the transcript
func Verify(random []byte, transcript *Transcript) error {

	//verification of sessionID
	sid := SessionID(transcript.Suite, transcript.Nodes, transcript.Faulty, transcript.X, transcript.Purpose, transcript.Time)
	if !bytes.Equal(transcript.SessionID, sid) {
		return errors.New("Wrong session identifier")
	}

	//verification of the final coString
	//first we compute the secrets of honest nodes according to results of vote
	var secrets []kyber.Point
	for id, vote := range transcript.Votes {
		if vote.Vote > transcript.Faulty {
			var encShareList []*pvss.PubVerShare
			var decShareList []*pvss.PubVerShare
			var keys []kyber.Point
			for j, share := range transcript.DecShares[id] {
				encShareList = append(encShareList, transcript.EncShares[id][j])
				decShareList = append(decShareList, share)
				keys = append(keys, transcript.X[j])
			}

			secret, err := pvss.RecoverSecret(transcript.Suite, nil, keys, encShareList, decShareList, transcript.Faulty+1, transcript.Nodes)
			if err != nil {
				return err
			}
			secrets = append(secrets, secret)
		}
	}
	//then we combine them
	coString := transcript.Suite.Point().Null()
	for _, secret := range secrets {
		kyber.Point.Add(coString, coString, secret)
	}
	bs, err := coString.MarshalBinary()
	if err != nil {
		return err
	}
	if !bytes.Equal(bs, random) {
		return errors.New("CoString isn't correct")
	}

	//everything was correct
	return nil
}

//SessionID hashes the data(suite, nodes, faulty, public keys, purpose, strating time) that caracterizes a particualar randShare protocol into a session identifier
func SessionID(suite pvss.Suite, nodes int, faulty int, X []kyber.Point, purpose string, time int64) []byte {

	//We put all the data into a byte buffer
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.LittleEndian, uint32(nodes)); err != nil {
		return nil
	}
	if err := binary.Write(buf, binary.LittleEndian, uint32(faulty)); err != nil {
		return nil
	}
	for _, key := range X {
		keyB, err := key.MarshalBinary()
		if err != nil {
			return nil
		}
		if _, err := buf.Write((keyB)); err != nil {
			return nil
		}
	}
	if _, err := buf.WriteString(purpose); err != nil {
		return nil
	}
	if err := binary.Write(buf, binary.LittleEndian, uint32(time)); err != nil {
		return nil
	}
	//we hash our data and return it
	hash := kyber.HashFactory.Hash(buf.Bytes()) //TODO
	return hash
}
