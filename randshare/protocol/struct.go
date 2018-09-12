package randsharepvss

import (
	"sync"

	"github.com/dedis/kyber"
	"github.com/dedis/kyber/share"
	"github.com/dedis/kyber/share/pvss"
	"github.com/dedis/onet"
	"github.com/dedis/onet/network"
)

//Name can be used from other packages to refer to this protocol.
const Name = "RandShare"

//init registers the handlers
func init() {
	for _, p := range []interface{}{A1{}, V1{}, R1{},
		StructA1{}, StructV1{}, StructR1{}} {
		network.RegisterMessage(p)
	}
}

//Share is a wrapper used to send a share along with the sender id, i.e., its row in the shares-matrix. Position is thus (Row, PubVerShare.S.I)
type Share struct {
	Row         int               //The position of the share
	PubVerShare *pvss.PubVerShare //The share
}

//Vote is a struct used to gather infromation about a node during the voting process
type Vote struct {
	Voted bool //Did this node vote already ?
	Vote  int  //The collective vote associated to that node
}

// A1 is the announce.
type A1 struct {
	SessionID []byte              //SessionID to verify the validity of the message
	Purpose   string              //the purpose of the current ProtocolInstance
	Time      int64               //time given by initializer to compute sessionID
	Src       int                 //The sender
	B         kyber.Point         //Info about pubPoly of Src
	Commits   []kyber.Point       //Commits used with B to reconstruct pubPoly
	Shares    []*pvss.PubVerShare //The encrypted shares or src-th node
}

//StructA1 just contains Announce and the data necessary to identify and
// process the message in the sda framework.
type StructA1 struct {
	*onet.TreeNode //The tree
	A1             //The announce
}

//V1 is the vote
type V1 struct {
	SessionID []byte        //SessionID to verify the validity
	Src       int           //The sender
	Votes     map[int]*Vote //Its votes
}

// StructV1 just contains V1 and the data necessary to identify and
// process the message in the sda framework.
type StructV1 struct {
	*onet.TreeNode //The tree
	V1             //The reply
}

// R1 is the reply.
type R1 struct {
	SessionID []byte   //SessionID to verify the validity of the reply
	Src       int      //The sender
	Shares    []*Share //The decrypted shares of src-th node (src-th piece of each secret)
}

// StructR1 just contains R1 and the data necessary to identify and
// process the message in the sda framework.
type StructR1 struct {
	*onet.TreeNode //The tree
	R1             //The reply
}

// Transcript is given to a third party so that it can verify the process of creation of our random srting
type Transcript struct {
	SessionID []byte                            //The sessionID
	Suite     pvss.Suite                        //The suite (rs.Suite())
	Nodes     int                               //Number of nodes
	Faulty    int                               //Number of faulty nodes
	Purpose   string                            //The purpose
	Time      int64                             //the starting time
	X         []kyber.Point                     //The public keys
	H         kyber.Point                       //the 2nd base point
	EncShares map[int]map[int]*pvss.PubVerShare //The encrypted shares
	Votes     map[int]*Vote                     //The votes
	DecShares map[int]map[int]*pvss.PubVerShare //The decrypted shares
}

//RandShare is our protocol struct
type RandShare struct {
	*onet.TreeNodeInstance                                   //The tree of nodes
	mutex                  sync.Mutex                        //Mutex to avoid concurrency
	nodes                  int                               //Number of nodes
	faulty                 int                               //Number of faulty nodes
	threshold              int                               //The threshold to recover values
	purpose                string                            //The purpose of the protocol
	startingTime           int64                             //starting time of the randshare protocol run
	sessionID              []byte                            //The SessionID number (see method SessionID)
	H                      kyber.Point                       //Our second base point created with SessionID
	pubPolys               []*share.PubPoly                  //The pubPoly of every node
	X                      []kyber.Point                     //The public keys
	encShares              map[int]map[int]*pvss.PubVerShare //Matrix of encrypted shares : ES_i(j) = encShare[i][j]
	tracker                map[int]int                       //tracker[i] can be -1 not enough enc share verified, 0 nothing received, 1 we have enough enc shares
	votes                  map[int]*Vote                     //Indexes of good nodes is set at 1, sent when receieved an announce from everyone
	nPrime                 int                               //Number of "good nodes" after voting process
	decShares              map[int]map[int]*pvss.PubVerShare //Matrix of decrypted shares : DS_i(j) = decShare[i][j]
	secrets                map[int]kyber.Point               //Recovered secrets
	coStringReady          bool                              //Is the coString available ?
	coString               kyber.Point                       //Collective random string computed with the secrets
	Done                   chan bool                         //Is the protocol done ?
}
