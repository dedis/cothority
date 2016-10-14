package byzcoin_ng

/*
Defines the service of Byzcoin-NG that intatiates one bfrcosi protocol per block
*/

import (
	"container/heap"
	"encoding/json"
	"errors"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/protocols/bftcosi"
	"github.com/dedis/cothority/protocols/byzcoin/blockchain"
	"github.com/dedis/cothority/protocols/byzcoin/blockchain/blkparser"
	"github.com/dedis/cothority/protocols/manage"
	"github.com/dedis/cothority/sda"
	"sync"
	"time"
)

// ServiceName is the name to refer to the Template service from another
// package.
const ServiceName = "ByzcoinNG"
const BNGBFT = "Byzcoin_NG_BFT"
const ReadFirstNBlocks = 66000

func init() {
	sda.RegisterNewService(ServiceName, newByzcoinNGService)
	network.RegisterPacketType(&bftcosi.MicroBlock{})
	sda.ProtocolRegisterName(BNGBFT, func(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
		return bftcosi.NewBFTCoSiProtocol(n, nil, nil)
	})
}

// Serivce handles the creation of new microblocks propsoed by the leader
type Service struct {
	// We need to embed the ServiceProcessor, so that incoming messages
	// are correctly handled.
	*sda.ServiceProcessor
	path string
	//Mutex that emulates the hardware bottleneck
	QMutex    sync.Mutex
	QMutexver sync.RWMutex
	PQueue    *bftcosi.PriorityQueue
	PQueuever *bftcosi.PriorityQueue
	HWMutex   sync.Mutex
	Vempty    bool
	//TODO push this inside the blocks
	Roster *sda.Roster

	SerilizeChan chan bftcosi.Item

	lastBlock    string
	lastKeyBlock string

	transaction *[]blkparser.Tx
}

var magicNum = [4]byte{0xF9, 0xBE, 0xB4, 0xD9}

func (s *Service) StartSimul(blocksPath string, nTxs int, Roster *sda.Roster) error {
	s.Roster = Roster
	log.Lvl2("ByzCoin will trigger up to", nTxs, "transactions")
	parser, err := blockchain.NewParser(blocksPath, magicNum)
	log.Lvl1(blocksPath)

	transactions, err := parser.Parse(0, ReadFirstNBlocks)
	if len(transactions) == 0 {
		return errors.New("Couldn't read any transactions.")
	}
	if err != nil {
		log.Error("Error: Couldn't parse blocks in", blocksPath,
			".\nPlease download bitcoin blocks as .dat files first and place them in",
			blocksPath, "Either run a bitcoin node (recommended) or using a torrent.")
		return err
	}
	if len(transactions) < nTxs {
		log.Errorf("Read only %v but caller wanted %v", len(transactions), nTxs)
	}

	s.transaction = &transactions

	return nil
}

func (s *Service) startEpoch(priority int, size int) (*bftcosi.MicroBlock, error) {
	//number of rounds... should be viariable
	block, err := GetBlock(size, *s.transaction, s.lastBlock, s.lastKeyBlock, priority)
	if err != nil {
		log.Lvl1("cannot get block")
		return nil, err
	}

	block.Roster = s.Roster
	s.signNewBlock(block)
	if err != nil {
		log.Lvl1("cannot sign block")
		return nil, err
	}
	err = block.BlockSig.Verify(network.Suite, block.Roster.Publics())
	if err != nil {
		log.Lvl1("cannot verify block")
		return nil, err
	}

	return block, nil
}

// signNewBlock should start a BFT-signature on the newest block
//it is invoked by the leader of the epoch
func (s *Service) signNewBlock(block *bftcosi.MicroBlock) (*bftcosi.MicroBlock, error) {
	log.Lvl4("Signing new block", block)
	if block == nil {
		log.Lvl3("Block is empty")

	} else {
		log.Lvl3("Got a block")

		// Sign it
		err := s.startBFTSignature(block)
		if err != nil {
			return nil, err
		}
		// Verify it
		err = block.BlockSig.Verify(network.Suite, s.Roster.Publics())
		if err != nil {
			return nil, err
		}
		//s.startPropagation(block)
		s.lastBlock = block.HeaderHash

		return block, nil
	}
	return nil, nil
}

func (s *Service) startBFTSignature(block *bftcosi.MicroBlock) error {
	log.Lvl3("Starting bftsignature with root-node=", s.ServerIdentity())
	done := make(chan bool)
	// create the message we want to sign for this round
	msg := []byte(block.HeaderHash)
	el := block.Roster

	// Start the protocol
	tree := el.GenerateNaryTreeWithRoot(2, s.ServerIdentity())

	node, err := s.CreateProtocolService(BNGBFT, tree)
	if err != nil {
		return errors.New("Couldn't create new node: " + err.Error())
	}

	// Register the function generating the protocol instance
	root := node.(*bftcosi.ProtocolBFTCoSi)
	root.Msg = msg
	data, err := network.MarshalRegisteredType(block)
	if err != nil {
		return errors.New("Couldn't marshal block: " + err.Error())
	}
	root.Data = data
	root.ServiceChannel = s.SerilizeChan

	// in testing-mode with more than one host and service per cothority-instance
	// we might have the wrong verification-function, so set it again here.
	root.VerificationFunction = s.bftVerify
	// function that will be called when protocol is finished by the root
	root.RegisterOnDone(func() {
		done <- true
	})
	go node.Start()
	select {
	case <-done:
		block.BlockSig = root.Signature()
		if len(block.BlockSig.Exceptions) != 0 {
			return errors.New("Not everybody signed off the new block")
		}
		if err := block.BlockSig.Verify(network.Suite, el.Publics()); err != nil {
			return errors.New("Couldn't verify signature")
		}
	case <-time.After(time.Second * 600):
		return errors.New("Timed out while waiting for signature")
	}
	return nil
}

// NewProtocol is called on all nodes of a Tree (except the root, since it is
// the one starting the protocol) so it's the Service that will be called to
// generate the PI on all others node.
func (s *Service) NewProtocol(tn *sda.TreeNodeInstance, conf *sda.GenericConfig) (sda.ProtocolInstance, error) {
	var pi sda.ProtocolInstance
	var err error
	switch tn.ProtocolName() {
	case "Propagate":
		pi, err = manage.NewPropagateProtocol(tn)
		if err != nil {
			return nil, err
		}
		pi.(*manage.Propagate).RegisterOnData(s.PropagateSkipBlock)
	case BNGBFT:
		pi, err = bftcosi.NewBFTCoSiProtocol(tn, s.bftVerify, s.SerilizeChan)
	}
	return pi, err

}

// newTemplate receives the context and a path where it can write its
// configuration, if desired. As we don't know when the service will exit,
// we need to save the clconfiguration on our own from time to time.
func newByzcoinNGService(c *sda.Context, path string) sda.Service {
	s := &Service{
		ServiceProcessor: sda.NewServiceProcessor(c),
		path:             path,
		lastBlock:        "0",
		lastKeyBlock:     "0",
		transaction:      &[]blkparser.Tx{},
		Vempty:           true,
		PQueue:           &bftcosi.PriorityQueue{},
		PQueuever:        &bftcosi.PriorityQueue{},
		SerilizeChan:     make(chan bftcosi.Item),
	}
	heap.Init(s.PQueue)
	heap.Init(s.PQueuever)
	go func() {
		empty := true
		for {
			chanel := <-s.SerilizeChan
			if chanel.Priority != -1 {
				s.QMutex.Lock()
				if empty {
					empty = false
					chanel.NotifyChan <- true
					s.QMutex.Unlock()
				} else {
					heap.Push(s.PQueue, &chanel)
					s.QMutex.Unlock()
				}
			} else {
				s.QMutex.Lock()
				if s.PQueue.Len() != 0 {
					item := s.PQueue.Pop().(*bftcosi.Item)
					item.NotifyChan <- true
				} else {
					empty = true
				}
				s.QMutex.Unlock()

			}
		}
	}()
	return s
}

// GetBlock returns the next block available from the transaction pool.
func GetBlock(size int, transactions []blkparser.Tx, lastBlock string, lastKeyBlock string, priority int) (*bftcosi.MicroBlock, error) {
	if len(transactions) < 1 {
		return nil, errors.New("no transaction available")
	}

	trlist := blockchain.NewTransactionList(transactions, size)
	header := blockchain.NewHeader(trlist, lastBlock, lastKeyBlock)
	trblock := blockchain.NewTrBlock(trlist, header)
	block := &bftcosi.MicroBlock{}
	block.TrBlock = trblock
	block.Priority = priority
	return block, nil
}

// VerifyBlock is a simulation of a real verification block algorithm
//TODO change footprint to the bftcosi one
func (s *Service) bftVerify(msg []byte, data []byte) bool {
	//We measure the average block verification delays is 174ms for an average
	//block of 500kB.
	//To simulate the verification cost of bigger blocks we multiply 174ms
	//times the size/500*1024
	log.Lvlf4("%s verifying block %x", s.ServerIdentity(), msg)
	_, sbN, err := network.UnmarshalRegistered(data)
	if err != nil {
		log.Error("Couldn't unmarshal Block", data)
		return false
	}
	block := sbN.(*bftcosi.MicroBlock)
	item := &bftcosi.Item{
		Priority:   block.Priority,
		NotifyChan: make(chan bool),
	}
	s.QMutexver.Lock()
	if s.Vempty { //define s.Vempty
		s.Vempty = false
		s.QMutexver.Unlock()
	} else {
		heap.Push(s.PQueuever, item)
		s.QMutexver.Unlock()
		<-item.NotifyChan
	}

	// for {
	// 	s.HWMutex.Lock()
	// 	s.HWMutex.Unlock()
	// 	s.QMutexver.RLock()
	// 	temp := s.PQueuever.Peak()
	// 	if block.Priority == temp {
	// 		s.HWMutex.Lock()
	// 		s.QMutexver.RUnlock()
	// 		s.QMutexver.Lock()
	// 		item = s.PQueuever.Pop().(*bftcosi.Item)
	// 		if block.Priority != item.Priority {
	// 			heap.Push(s.PQueuever, item)
	// 			s.QMutexver.Unlock()

	// 			s.HWMutex.Unlock()
	// 			continue
	// 		}
	// 		s.QMutexver.Unlock()
	// 		break
	// 	} else {
	// 		s.QMutexver.RUnlock()
	// 	}

	// }

	b, _ := json.Marshal(block)
	s1 := len(b)
	var n time.Duration
	n = time.Duration(s1 / (500 * 1024))
	//s.HWMutex.Lock()
	time.Sleep(150 * time.Millisecond * n) //verification of 174ms per 500KB simulated
	//s.HWMutex.Unlock()
	s.QMutexver.Lock()
	if s.PQueuever.Len() != 0 {
		item := s.PQueuever.Pop().(*bftcosi.Item)
		item.NotifyChan <- true
	} else {
		s.Vempty = true
	}
	s.QMutexver.Unlock()

	// verification of the header
	verified := true
	//verified := block.Header.Parent == s.lastBlock //&& block.Header.ParentKey == s.lastKeyBlock
	verified = verified && block.Header.MerkleRoot == blockchain.HashRootTransactions(block.TransactionList)
	verified = verified && block.HeaderHash == blockchain.HashHeader(block.Header)
	// notify it
	log.Lvl3("Verification of the block done =", verified)
	if !verified {
		log.Lvl3("header", block.Header.Parent, "cached", s.lastBlock)
	}
	return verified
}

// notify other services about new/updated skipblock
func (s *Service) startPropagation(block *bftcosi.MicroBlock) error {
	log.Lvlf3("Starting to propagate for service %x", s.Context.ServerIdentity().ID[0:8])
	roster := block.Roster
	if roster == nil {
		return errors.New("Didn't find Roster")
	}
	replies, err := manage.PropagateStartAndWait(s.Context, roster,
		block, 100000, s.PropagateSkipBlock)
	if err != nil {
		return err
	}
	if replies != len(roster.List) {
		log.Warn("Did only get", replies, "out of", len(roster.List))
	}
	return nil
}

// PropagateSkipBlock will save a new SkipBlock
func (s *Service) PropagateSkipBlock(msg network.Body) {
	sb, ok := msg.(*bftcosi.MicroBlock)
	if !ok {
		log.Error("Couldn't convert to SkipBlock")
		return
	}
	if err := sb.VerifySignatures(); err != nil {
		log.Error(err)
		return
	}
	s.lastBlock = sb.HeaderHash
	//TODO: Handle Key blocks
	log.Lvlf3("Stored skip block %+v in %x", *sb, s.Context.ServerIdentity().ID[0:8])
}
