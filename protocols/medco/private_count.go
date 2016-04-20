package medco

import (
	"bufio"
	"errors"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
	"os"
	"strconv"
	"time"
)

type PrivateCountProtocol struct {
	*sda.Node
	ElGamalQueryChannel chan QueryStruct
	PHQueryChannel      chan ProcessableQueryStruct
	ElGamalDataChannel  chan HolderResponseDataStruct
	ResultChannel       chan []ResultStruct
	FeedbackChannel     chan CipherVector

	nextNodeInCircuit   *sda.TreeNode
	shortTermSecret     abstract.Secret
	bucketCount         int

	ClientPublicKey     *abstract.Point
	ClientQuery         *CipherText
	BucketDesc          *[]int64
	EncryptedData       *[]HolderResponseData
}

func init() {
	network.RegisterMessageType(QueryMessage{})
	network.RegisterMessageType(QueryStruct{})
	network.RegisterMessageType(HolderResponseDataMessage{})
	network.RegisterMessageType(HolderResponseDataStruct{})
	network.RegisterMessageType(ProcessableQueryMessage{})
	network.RegisterMessageType(ProcessableQueryStruct{})
	network.RegisterMessageType(ResultMessage{})
	network.RegisterMessageType(ResultStruct{})
	sda.ProtocolRegisterName("PrivateCount", NewPrivateCountProtocol)
}


func NewPrivateCountProtocol(n *sda.Node) (sda.ProtocolInstance, error) {
	newInstance := &PrivateCountProtocol{
		Node: n,
		FeedbackChannel: make(chan CipherVector),
		shortTermSecret: n.Suite().Secret().Pick(n.Suite().Cipher([]byte("Cothosecrets" + n.Name()))),
	}

	var channels = []interface{}{
		&newInstance.ElGamalQueryChannel, // Probabilistically-to-deterministically encrypted query conversion channel
		&newInstance.PHQueryChannel,	  // Deterministically encrypted query broadcast channel
		&newInstance.ElGamalDataChannel,  // Probabilistically-to-deterministically encrypted data conversion channel
		&newInstance.ResultChannel } // Encrypted, bucketed count reporting channel

	// Encrypted, bucketed count reporting channel
	for _, channel := range channels {
		err := newInstance.RegisterChannel(channel)
		if err != nil {
			return nil, errors.New("Could not register channel :\n" + err.Error())
		}
	}

	var i int
	var node *sda.TreeNode
	var nodeList = n.Tree().List()
	for i, node = range nodeList {
		if n.TreeNode().Equal(node) {
			newInstance.nextNodeInCircuit = nodeList[(i+1)%len(nodeList)]
			break
		}
	}

	return newInstance, nil
}

func (p *PrivateCountProtocol) Start() error {
	dbg.Lvl1("Started visitor protocol as node", p.Node.Name())

	if (p.ClientPublicKey == nil) {
		return errors.New("No public key was provided by the client.")
	}

	if (p.ClientQuery == nil) {
		return errors.New("No query was provided by the client.")
	}

	if (p.BucketDesc == nil) {
		return errors.New("No bucket description provided by the client.")
	}
	p.bucketCount = len(*p.BucketDesc) + 1


	// The starting node starts the protocol by sending the probabilistically encrypted query to the next node
	queryMessage := &QueryMessage{&VisitorMessage{0}, *p.ClientQuery, *p.BucketDesc,*p.ClientPublicKey}
	queryMessage.Filter.ReplaceContribution(p.Suite(), p.Private(), p.shortTermSecret)
	queryMessage.SetVisited(p.Node.TreeNode(), p.Node.Tree())

	p.sendToNext(queryMessage)
	return nil
}

func (p *PrivateCountProtocol) Dispatch() error {

	dbg.Lvl1("Began running protocol", p.Node.Name())


	// 1. Query crypto-switching phase
	deterministicQuery, buckets, _ := p.queryReplacementPhase()
	dbg.Lvl1(p.Node.Name(), "Finished query replacement phase.")

	// 2. Data crypto-switching phase
	go p.injectEncryptedData() // If the node has some local data, inject it to the system
	encryptedBuckets, _ := p.dataReplacementAndCountingPhase(deterministicQuery, buckets)
	dbg.Lvl1(p.Node.Name(), "Finished data replacement phase.")


	// 3. Match count reporting phase
	dbg.Lvl1(p.TreeNode().Name(), "Reporting its count")
	p.matchCountReportingPhase(&encryptedBuckets)

	return nil
}

// 1. Query crypto-switching phase
func (p *PrivateCountProtocol) queryReplacementPhase() (*DeterministCipherText, []int64,error) {
	for {
		select {
		// Node receives a Query for deterministic conversion
		case encQuery := <-p.ElGamalQueryChannel:
			dbg.Lvl1(p.Node.Name(), "recieved msg")
			// Removes its own probabilistic contribution contribution
			encQuery.Filter.ReplaceContribution(p.Suite(), p.Private(), p.shortTermSecret)
			encQuery.SetVisited(p.TreeNode(), p.Tree())
			p.ClientPublicKey = &encQuery.QuerierPublicKey
			p.BucketDesc = &encQuery.BucketsDescription
			p.bucketCount = len(encQuery.BucketsDescription)+1

			// If Node is the last probabilistic contribution in ciphertext, complete the deterministic
			// conversion and broadcast the deterministic query
			if !p.sendToNext(&encQuery.QueryMessage) {
				deterministicQuery := DeterministCipherText{encQuery.Filter.C}
				msg := &ProcessableQueryMessage{deterministicQuery, encQuery.BucketsDescription,encQuery.QuerierPublicKey}
				p.broadcast(msg)
				return &msg.Query, encQuery.BucketsDescription,nil
			}

		// Node receives a deterministic query to be matched against data
		case deterministicQuery := <-p.PHQueryChannel:
			p.ClientPublicKey = &deterministicQuery.Public
			return &deterministicQuery.Query, deterministicQuery.Buckets ,nil
		}
	}
}

// 2. Data crypto-switching phase
func (p *PrivateCountProtocol) dataReplacementAndCountingPhase(query *DeterministCipherText, buckets []int64) (CipherVector, error) {
	encryptedBuckets := NullCipherVector(p.Suite(), p.bucketCount, *p.ClientPublicKey)
	i := 0
	for {
		select {
		// Node receives a new probabilistically encrypted data and its counting vector
		case encDataMessage := <-p.ElGamalDataChannel:

			// Remove its probabilistic contribution
			encDataMessage.Code.ReplaceContribution(p.Suite(), p.Private(), p.shortTermSecret)
			encDataMessage.SetVisited(p.TreeNode(), p.Tree())
			i +=  1
			if i%1000 == 0 {
				dbg.Lvl1(i, "messages processed.")
			}
			//dbg.Lvl1(i)
			// If node is the last probabilitic contribution and the ciphertext matches the query,
			// sums the current counting vector with the one of the data.
			if 	!p.sendToNext(&encDataMessage.HolderResponseDataMessage) &&
				query.Equals(&DeterministCipherText{encDataMessage.Code.C}) {

				encryptedBuckets.Add(encryptedBuckets, encDataMessage.Buckets)
			}

		case <-time.After(3*time.Second):
			return encryptedBuckets, nil
		}
	}
}

// 3. Match count reporting phase
func (p *PrivateCountProtocol) matchCountReportingPhase(encryptedBuckets *CipherVector) {
	reportedBuckets := *encryptedBuckets

	// If node is not a leaf, waits for its children to report their counts
	if !p.IsLeaf() {
		results := <-p.ResultChannel
		for _, result := range results {
			reportedBuckets.Add(reportedBuckets, result.Result)
		}
	}

	// If node is not the root, sends count to its parent, else report the count to report channel
	if !p.IsRoot() {
		p.SendTo(p.Parent(), &ResultMessage{*encryptedBuckets})
	} else {
		p.FeedbackChannel <- *encryptedBuckets
	}

}

// Sends the message msg to the next randomly chosen node. If such node exist (all node already received the message),
// returns false. Otherwise, return true.
func (p *PrivateCountProtocol) sendToNext(msg VisitorMessageI) bool {
	if !msg.AlreadyVisited(p.nextNodeInCircuit, p.Tree()) {
		err := p.SendTo(p.nextNodeInCircuit, msg)
		if err != nil {
			dbg.Lvl1("Had an error sending a message: ", err)
		}
		return true;
	}
	return false
}

// Sends the given message to all node except self
func (p *PrivateCountProtocol) broadcast(msg interface{}) {
	for _, node := range p.Tree().List() {
		if !node.Equal(p.TreeNode()) {
			p.SendTo(node, msg)
		}
	}
}

// For testing purpose, read a file with the test data and inject them.
func (p *PrivateCountProtocol) injectEncryptedData() {

	// For testing purpose, let consider buckets to be age buckets
	bucket := func (age int64) int {
		var i int
		for i=0; i < p.bucketCount-1 && age >= (*p.BucketDesc)[i]; i++ {}
		return i
	}

	filep, err := os.Open(p.TreeNode().Id.String()+".txt")
	defer filep.Close()

	if p.EncryptedData == nil {
		if err == nil {
			dbg.Lvl1(p.TreeNode().Name(), "Is STARTING to inject its data.")
			filereader := bufio.NewScanner(filep)
			filereader.Split(bufio.ScanWords)
			for filereader.Scan() {
				code := filereader.Text()
				filereader.Scan()
				age,_ := strconv.ParseInt(filereader.Text(), 10, 0)
				cipherText, _ := EncryptBytes(p.Suite(), p.EntityList().Aggregate, []byte(code))
				encBuckets := NullCipherVector(p.Suite(), p.bucketCount, *p.ClientPublicKey)
				encBuckets[bucket(int64(age))] = *EncryptInt(p.Suite(), *p.ClientPublicKey, 1)
				elGamalData := HolderResponseData{encBuckets, *cipherText}
				p.sendToNext(&HolderResponseDataMessage{&VisitorMessage{0}, elGamalData})
			}
			dbg.Lvl1(p.TreeNode().Name(), "Is DONE injecting its data.")
		}
	} else {
		for _, cipherText := range *p.EncryptedData {
			p.sendToNext(&HolderResponseDataMessage{&VisitorMessage{0}, cipherText})
		}
	}
}


