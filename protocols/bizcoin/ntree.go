package bizcoin

import (
	"encoding/json"
	"github.com/dedis/cothority/lib/crypto"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/protocols/bizcoin/blockchain"
	"github.com/dedis/cothority/protocols/bizcoin/blockchain/blkparser"
	"github.com/satori/go.uuid"
	"math"
)

// Ntree is a basic implementation of a byzcoin consensus protocol using a tree
// and each verifiers will have independant signatures. The messages are then
// bigger and the verification time is also bigger.
type Ntree struct {
	*sda.Node
	// the block to sign
	block *blockchain.TrBlock
	// channel to notify the end of the verification of a block
	verifyBlockChan chan bool

	// channel to notify the end of the verification of a signature request
	verifySignatureRequestChan chan bool

	// the temps signature you receive in the first phase
	tempBlockSig         *NaiveBlockSignature
	tempBlockSigReceived int

	// the temps signature you receive in the second phase
	tempSignatureResponse         RoundSignatureResponse
	tempSignatureResponseReceived int

	announceChan chan struct {
		*sda.TreeNode
		BlockAnnounce
	}

	blockSignatureChan chan struct {
		*sda.TreeNode
		NaiveBlockSignature
	}

	roundSignatureRequestChan chan struct {
		*sda.TreeNode
		RoundSignatureRequest
	}

	roundSignatureResponseChan chan struct {
		*sda.TreeNode
		RoundSignatureResponse
	}

	onDoneCallback func()
}

func NewNtreeProtocol(node *sda.Node) (*Ntree, error) {
	nt := &Ntree{
		Node:                       node,
		verifyBlockChan:            make(chan bool),
		verifySignatureRequestChan: make(chan bool),
	}
	node.RegisterChannel(&nt.announceChan)
	node.RegisterChannel(&nt.blockSignatureChan)
	node.RegisterChannel(&nt.roundSignatureRequestChan)
	node.RegisterChannel(&nt.roundSignatureResponseChan)

	go nt.listen()
	return nt, nil
}

func NewNTreeRootProtocol(node *sda.Node, transactions []blkparser.Tx) (*Ntree, error) {
	nt, _ := NewNtreeProtocol(node)
	var err error
	nt.block, err = getBlock(transactions, "", "")
	return nt, err
}

// Announce the new block to sign
func (nt *Ntree) Start() error {
	go verifyBlock(nt.block, "", "", nt.verifyBlockChan)
	for _, tn := range nt.Children() {
		nt.SendTo(tn, &BlockAnnounce{nt.block})
	}
	return nil
}

func (nt *Ntree) Dispatch() error {
	// do nothing
	return nil
}

func (nt *Ntree) listen() {
	for {
		select {
		// Dispatch the block through the whole tree
		case msg := <-nt.announceChan:
			nt.block = msg.BlockAnnounce.Block
			// verify the block
			go verifyBlock(nt.block, "", "", nt.verifyBlockChan)
			if nt.IsLeaf() {
				nt.startBlockSignature()
				continue
			}
			for _, tn := range nt.Children() {
				nt.SendTo(tn, &msg.BlockAnnounce)
			}
			// generate your own signature / exception and pass that up to the
			// root
		case msg := <-nt.blockSignatureChan:
			nt.handleBlockSignature(&msg.NaiveBlockSignature)
			// Dispatch the signature + expcetion made before through the whole
			// tree
		case msg := <-nt.roundSignatureRequestChan:
			go nt.verifySignatureRequest(&msg.RoundSignatureRequest)

			if nt.IsLeaf() {
				nt.startSignatureResponse()
				continue
			}
			for _, tn := range nt.Children() {
				nt.SendTo(tn, &msg.RoundSignatureRequest)
			}
			// Decide if we want to sign this or not
		case msg := <-nt.roundSignatureResponseChan:
			nt.handleRoundSignatureResponse(&msg.RoundSignatureResponse)
		}
	}
}

func (nt *Ntree) startBlockSignature() {
	nt.computeBlockSignature()
	for _, tn := range nt.Children() {
		nt.SendTo(tn, &nt.tempBlockSig)
	}
}

func (nt *Ntree) computeBlockSignature() {
	// wait the end of verification of the block
	ok := <-nt.verifyBlockChan
	//marshal the blck
	marshalled, err := json.Marshal(nt.block)
	if err != nil {
		dbg.Error(err)
		return
	}

	// if stg is wrong, we put exceptions
	if !ok {
		nt.tempBlockSig.Exceptions = append(nt.tempBlockSig.Exceptions, Exception{nt.TreeNode().Id})
	} else { // we put signature
		schnorr, _ := crypto.SignSchnorr(nt.Suite(), nt.Private(), marshalled)
		nt.tempBlockSig.Sigs = append(nt.tempBlockSig.Sigs, schnorr)
	}
}

// handleBlockSignature will look if the block is valid. If it is, we sign it.
// if it is not, we don't sign it and we put up an exception.
func (nt *Ntree) handleBlockSignature(msg *NaiveBlockSignature) {
	nt.tempBlockSig.Sigs = append(nt.tempBlockSig.Sigs, msg.Sigs...)
	nt.tempBlockSig.Exceptions = append(nt.tempBlockSig.Exceptions, msg.Exceptions...)
	nt.tempBlockSigReceived++
	// not enough signatures for the moment
	if nt.tempBlockSigReceived < len(nt.Children()) {
		return
	}
	nt.computeBlockSignature()
	// if we are root => going further in the protocol
	if nt.IsRoot() {
		nt.startSignatureRequest(msg)
		return
	}
	// send msg up the tree
	nt.SendTo(nt.Parent(), &nt.tempBlockSig)
}

// startSignatureRequest is the root starting the new phase. It will broadcast
// the signature of everyone amongt the tree.
func (nt *Ntree) startSignatureRequest(msg *NaiveBlockSignature) {
	dbg.Lvl3(nt.Name(), "Start Signature Request")
	for _, tn := range nt.Children() {
		nt.SendTo(tn, nt.tempBlockSig)
	}
}

// Go routine that will do the verification of the signature request in
// parrallele
func (nt *Ntree) verifySignatureRequest(msg *RoundSignatureRequest) {
	// verification if we have too much exceptions
	threshold := int(math.Ceil(float64(len(nt.Tree().ListNodes())) / 3.0))
	if len(msg.Exceptions) > threshold {
		nt.verifySignatureRequestChan <- false
	}

	// verification of all the signatures
	var goodSig int
	marshalled, _ := json.Marshal(nt.block)
	for _, sig := range msg.Sigs {
		if err := crypto.VerifySchnorr(nt.Suite(), nt.Public(), marshalled, sig); err == nil {
			goodSig++
		}
	}

	// enough good signatures ?
	if goodSig <= 2*threshold {
		nt.verifySignatureRequestChan <- false
	}

	nt.verifySignatureRequestChan <- true
}

func (nt *Ntree) startSignatureResponse() {
	nt.computeSignatureResponse()
	for _, tn := range nt.Children() {
		nt.SendTo(tn, &nt.tempSignatureResponse)
	}
}

func (nt *Ntree) computeSignatureResponse() {
	// wait for the verification to be done
	ok := <-nt.verifySignatureRequestChan
	if !ok {
		nt.tempSignatureResponse.Exceptions = append(nt.tempSignatureResponse.Exceptions, Exception{nt.TreeNode().Id})
	} else {
		// compute the message out of the previous signature
		// marshal only the header here (so signature between the two phases are
		// garanteed to be different)
		marshalled, err := json.Marshal(nt.block.Header)
		if err != nil {
			dbg.Error(err)
			return
		}
		sig, err := crypto.SignSchnorr(nt.Suite(), nt.Private(), marshalled)
		if err != nil {
			return
		}
		nt.tempSignatureResponse.Sigs = append(nt.tempSignatureResponse.Sigs, sig)
	}
}

// SignatureResponse is the last phase where the final signature goes up until
// the root
func (nt *Ntree) handleRoundSignatureResponse(msg *RoundSignatureResponse) {
	// do we have received it all
	nt.tempSignatureResponse.Sigs = append(nt.tempSignatureResponse.Sigs, msg.Sigs...)
	nt.tempSignatureResponse.Exceptions = append(nt.tempSignatureResponse.Exceptions, msg.Exceptions...)
	nt.tempSignatureResponseReceived++

	if nt.tempSignatureResponseReceived < len(nt.Children()) {
		return
	}

	nt.computeSignatureResponse()

	// if i'm root I'm finished
	if nt.IsRoot() {
		if nt.onDoneCallback != nil {
			nt.onDoneCallback()
		}
		return
	}
	nt.SendTo(nt.Parent(), msg)
}

func (nt *Ntree) RegisterOnDone(fn func()) {
	nt.onDoneCallback = fn
}

// BlockAnnounce is used to signal the block to the whole tree.
type BlockAnnounce struct {
	Block *blockchain.TrBlock
}

// the signatureS of a block goes up the tree using this message
type NaiveBlockSignature struct {
	Sigs       []crypto.SchnorrSig
	Exceptions []Exception
}

// Exception is  just representing the notion that a peers does not accept to
// sign something. It justs passes its TreeNodeId inside. No need for public key
// or whatever because each signatures is independant.
type Exception struct {
	Id uuid.UUID
}

// RoundSignatureRequest basically is the the block sisgnature broadcasting
// downt he tree
type RoundSignatureRequest struct {
	NaiveBlockSignature
}

// The final signatures
type RoundSignatureResponse struct {
	NaiveBlockSignature
}
