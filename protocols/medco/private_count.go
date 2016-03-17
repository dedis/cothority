package medco

import (
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/lib/dbg"
	"math/rand"
	"github.com/dedis/cothority/lib/network"
	"errors"
	"github.com/dedis/crypto/abstract"
	"time"
)

type PrivateCountProtocol struct {
	*sda.Node
	ElGamalQueryChannel chan ElGamalQueryStruct
	PHQueryChannel      chan PHQueryStruct
	ElGamalDataChannel  chan ElGamalDataStruct
	ResultChannel       chan []ResultStruct
	FeedbackChannel     chan CipherVector

	shortTermSecret     abstract.Secret

	ClientPublicKey     *abstract.Point
	ClientQuery	    *CipherText
	Buckets		    *[]string
	EncryptedData       *[]ElGamalData
}

func init() {
	network.RegisterMessageType(ElGamalQueryMessage{})
	network.RegisterMessageType(ElGamalQueryStruct{})
	network.RegisterMessageType(ElGamalDataMessage{})
	network.RegisterMessageType(ElGamalDataStruct{})
	network.RegisterMessageType(PHQueryMessage{})
	network.RegisterMessageType(PHQueryStruct{})
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

	errs := make([]error, 0)
	errs = append(errs, newInstance.RegisterChannel(&newInstance.ElGamalQueryChannel))
	errs = append(errs, newInstance.RegisterChannel(&newInstance.PHQueryChannel))
	errs = append(errs, newInstance.RegisterChannel(&newInstance.ElGamalDataChannel))
	errs = append(errs, newInstance.RegisterChannel(&newInstance.ResultChannel))

	for _, err := range errs {
		if err != nil {
			return nil, errors.New("Could not register channel :\n" + err.Error())
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

	queryMessage := &ElGamalQueryMessage{&VisitorMessage{0}, *p.ClientQuery, *p.Buckets ,*p.ClientPublicKey}
	queryMessage.Query.ReplaceContribution(p.Suite(), p.Private(), p.shortTermSecret)
	queryMessage.SetVisited(p.Node.TreeNode(), p.Node.Tree())

	p.sendToNext(queryMessage)
	return nil
}

func (p *PrivateCountProtocol) Dispatch() error {

	dbg.Lvl1("Began running protocol", p.Node.Name())
	deterministicQuery, buckets, _ := p.queryReplacementPhase()
	dbg.Lvl1(p.Node.Name(), "Finished query replacement phase.")

	if p.EncryptedData != nil {
		dbg.Lvl1(p.Node.Name(), "Injecting its data.")
		go func() {
			for _, cipherText := range *p.EncryptedData {
				p.sendToNext(&ElGamalDataMessage{&VisitorMessage{0}, cipherText})
			}
			dbg.Lvl1(p.Node.Name(), "Finished injecting its data.")
		}()
	}

	encryptedBuckets, _ := p.dataReplacementAndCountingPhase(deterministicQuery, buckets)
	dbg.Lvl1(p.Node.Name(), "Finished data replacement phase.")


	dbg.Lvl1("Reporting its count")
	p.matchCountReportingPhase(&encryptedBuckets)

	return nil
}

func (p *PrivateCountProtocol) queryReplacementPhase() (*DeterministCipherText, []string,error) {
	for {
		select {
		case encQuery := <-p.ElGamalQueryChannel:
			encQuery.Query.ReplaceContribution(p.Suite(), p.Private(), p.shortTermSecret)
			encQuery.SetVisited(p.TreeNode(), p.Tree())
			p.ClientPublicKey = &encQuery.Public
			if !p.sendToNext(&encQuery.ElGamalQueryMessage) {
				deterministicQuery := DeterministCipherText{encQuery.Query.C}
				msg := &PHQueryMessage{deterministicQuery, encQuery.Buckets,encQuery.Public}
				p.broadcast(msg)
				return &msg.Query, encQuery.Buckets ,nil
			}
		case deterministicQuery := <-p.PHQueryChannel:
			p.ClientPublicKey = &deterministicQuery.Public
			return &deterministicQuery.Query, deterministicQuery.Buckets ,nil
		}
	}
}

func (p *PrivateCountProtocol) dataReplacementAndCountingPhase(query *DeterministCipherText, buckets []string) (CipherVector, error) {
	encryptedBuckets := NullCipherVector(p.Suite(), len(buckets), *p.ClientPublicKey)
	for {
		select {
		case encDataMessage := <-p.ElGamalDataChannel:
			encDataMessage.Code.ReplaceContribution(p.Suite(), p.Private(), p.shortTermSecret)
			encDataMessage.SetVisited(p.TreeNode(), p.Tree())
			if !p.sendToNext(&encDataMessage.ElGamalDataMessage) {
				if query.Equals(&DeterministCipherText{encDataMessage.Code.C}) {
					encryptedBuckets.Add(encryptedBuckets, encDataMessage.Buckets)
				}
			}
		case <-time.After(time.Second * 3):
			return encryptedBuckets, nil
		}
	}
}

func (p *PrivateCountProtocol) matchCountReportingPhase(encryptedBuckets *CipherVector) {
	reportedBuckets := *encryptedBuckets
	if !p.IsLeaf() {
		results := <-p.ResultChannel
		for _, result := range results {
			reportedBuckets.Add(reportedBuckets, result.Result)
		}
	}
	if p.IsRoot() {
		p.FeedbackChannel <- *encryptedBuckets
	} else {
		p.SendTo(p.Parent(), &ResultMessage{*encryptedBuckets})
	}

}

func (p *PrivateCountProtocol) sendToNext(msg VisitorMessageI) bool {
	candidates := make([]*sda.TreeNode, 0)
	for _, node := range p.Tree().ListNodes() {
		if !msg.AlreadyVisited(node, p.Node.Tree()) {
			candidates = append(candidates, node)
		}
	}
	if len(candidates) > 0 {
		err := p.SendTo(candidates[rand.Intn(len(candidates))], msg)
		if err != nil {
			dbg.Lvl1("Had an error sending a message: ", err)
		}
		return true;
	}
	return false
}

func (p *PrivateCountProtocol) broadcast(msg interface{}) {
	for _, node := range p.Tree().ListNodes() {
		if !node.Equal(p.TreeNode()) {
			p.SendTo(node, msg)
		}
	}
}