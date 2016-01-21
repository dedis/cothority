package jvss

import (
	"errors"
	"fmt"
	"github.com/dedis/cothority/lib/cliutils"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"github.com/dedis/crypto/poly"
	"github.com/satori/go.uuid"
	"time"
)

// SDA-based JVSS , a port of app/shamir

// JVSS Protocol Instance structure holding the information for a longterm JVSS
// signing mechanism
type JVSSProtocol struct {
	// The host we are running on
	Host *sda.Host
	// The tree we are using
	Tree *sda.Tree
	// The TreeNode denating ourself in the tree
	Node *sda.TreeNode
	// the token for this protocol instance
	Token *sda.Token
	// The EntityList we are using / this is needed to "bypass" the tree
	// structure for the internals communication, when we set up the shares and
	// everything. We directly send our share to everyone else directly by using
	// this entitylist instead of broadcasting into the tree.
	List *sda.EntityList
	// the index where we are in this entitylist
	index int
	// the mapping between TreeNode's peer id in the Tree to index in the entitylist,
	// since JVSS mostly use the entityList
	nodeToIndex map[uuid.UUID]int
	// list of public keys represented in the entityList (needed by poly.Deal)
	publicList []abstract.Point
	// keys of the Host set as config.KeyPair
	key config.KeyPair
	// The longterm shared private public key pair used in this JVSS.
	// The idea is that you can keep this protocol instance as a longterm JVSS
	// using it to distributively sign anything as long as it runs.
	longterm *poly.SharedSecret
	// The schnorr struct used to sign / verify using the longterm key
	schnorr *poly.Schnorr
	// polyinfo related to how the shares are generated and reconstructed
	Info poly.Threshold
	// requests holds all the requests that we asked
	requests map[int]*RequestBuffer
	// lastrequestnumber seen or executed
	lastRequestNo int
	// more tmp variables  that are needed because of the way we use protocol
	// isntance. For example, you want to generate a distributed secret, you
	// need a poly.Receiver struct. You first create that struct in Start()
	// since you are the leader, and then receive the dealers in Dispatch(),
	// so you need that intermediate variable.
	// longterm-receiver of the shares
	ltReceiver *poly.Receiver
	// channel through we notify that we have sucessfully computed the longterm
	// distributed secret
	longtermDone chan bool
	// callback to know when the longterm has been generated
	onLongtermDone func(*poly.SharedSecret)
	// The channel through we give the deal we receive for the longterm
	// generation
	ltChan chan Longterm
	// The channel through we give the deal we receive for the random generation
	rdChan chan Random
	// channel through we give the requests we receive for a signature
	reqChan chan *sda.SDAData
	// channel through we give the responses we receive for a signature request
	respChan chan *sda.SDAData
}

// NewJVSSProtocol returns a JVSSProtocol with the fields set. You can then
// change the fields or set specific ones etc. If you want to use JVSSProotocol
// directly with SDA, you just need to  register this function:
// ```func(h,t,tok) ProtocolInstance  { return NewJVSSProtocol(h,t,tok) }```
// For example, this function returns a JVSSProtocol with a default
// poly.Treshold. You can give a new one after calling this function.
func NewJVSSProtocol(h *sda.Host, t *sda.TreeNode, tok *sda.Token) *JVSSProtocol {
	// find ourself in the entityList
	var idx int = -1
	// at the same time create the public list
	tree, _ := h.GetTree(tok.TreeID)
	pubs := make([]abstract.Point, len(tree.EntityList.List))
	for i := range tree.EntityList.List {
		ent := tree.EntityList.Get(i)
		if ent.Equal(h.Entity) {
			idx = i
		}
		pubs[i] = ent.Public
	}
	// map the index
	maps := make(map[uuid.UUID]int)
	nodes := tree.ListNodes()
	for i := range tree.EntityList.List {
		for _, n := range nodes {
			if n.Entity.Equal(tree.EntityList.Get(i)) {
				maps[n.Id] = i
			}
		}
	}
	if idx == -1 {
		panic("JVSSProtocol could not find itself into the entitylist")
	}
	kp := config.KeyPair{Public: h.Entity.Public, Secret: h.Private(), Suite: h.Suite()}
	nbPeers := len(tree.EntityList.List)
	jv := &JVSSProtocol{
		Host:         h,
		Tree:         tree,
		Node:         t,
		List:         tree.EntityList,
		Token:        tok,
		index:        idx,
		Info:         poly.Threshold{T: nbPeers, R: nbPeers, N: nbPeers},
		publicList:   pubs,
		key:          kp,
		ltChan:       make(chan Longterm),
		rdChan:       make(chan Random),
		requests:     make(map[int]*RequestBuffer),
		reqChan:      make(chan *sda.SDAData),
		respChan:     make(chan *sda.SDAData),
		nodeToIndex:  maps,
		longtermDone: make(chan bool),
		schnorr:      new(poly.Schnorr),
	}
	go jv.waitForLongterm()
	go jv.waitForRandom()
	go jv.waitForRequests()
	go jv.waitForResponses()
	return jv
}

func NewJVSSProtocolInstance(h *sda.Host, t *sda.TreeNode, tok *sda.Token) sda.ProtocolInstance {
	return NewJVSSProtocol(h, t, tok)
}

// Start will send the message to first compute the long term secret
// It's a blocking call  because we are supposed to launch that into a go
// routine anyway from sda.
func (jv *JVSSProtocol) Start() error {
	jv.setupLongtermReceiver()
	fmt.Println("Start() index=", jv.index, "for host", jv.Host.Entity.First())
	return jv.SetupDistributedSchnorr()
}

func (jv *JVSSProtocol) Dispatch(msgs []*sda.SDAData) error {
	// look the type
	switch msgs[0].MsgType {
	case LongtermType:
		// add the deals to the longterm
		for _, sda := range msgs {
			m := sda.Msg.(Longterm)
			// if I dont have a schnorr, it means someone is setting up a longterm
			// secret so I should set up mine also
			if jv.schnorr == nil || jv.ltReceiver == nil {
				jv.setupLongtermReceiver()
				go jv.SetupDistributedSchnorr()
			}
			jv.ltChan <- m
		}
	case RandomType:
		for _, sda := range msgs {
			m := sda.Msg.(Random)
			jv.rdChan <- m
		}
	case SignatureRequestType:
		for _, sda := range msgs {
			jv.reqChan <- sda
		}
	case SignatureResponseType:
		for _, sda := range msgs {
			jv.respChan <- sda
		}
	}
	return nil
}

// Verify returns true if a signature is valid or not
func (jv *JVSSProtocol) Verify(msg []byte, sig *poly.SchnorrSig) error {
	h := jv.Host.Suite().Hash()
	h.Write(msg)
	return jv.schnorr.VerifySchnorrSig(sig, h)
}

// Sign will make the JVSS protocol run and returns a SchnorrSig
func (jv *JVSSProtocol) Sign(msg []byte) (*poly.SchnorrSig, error) {
	// create new request number to generate random then signature
	request, err := jv.setupDistributedSecret()
	if err != nil {
		return nil, err
	}
	sigChan := make(chan *poly.SchnorrSig)
	request.SetSigChan(sigChan)
	// create signature request
	req := &SignatureRequest{
		RequestNo: request.requestNo,
		Msg:       msg,
	}
	// sends it
	jv.otherNodes(func(tn *sda.TreeNode) {
		jv.Host.SendSDAToTreeNode(jv.Token, tn, req)
	})

	// wait for the signature
	sig := <-sigChan
	request.ResetSigChan()
	return sig, nil
}

func (jv *JVSSProtocol) waitForResponses() {
	for sda := range jv.reqChan {
		sigResponse := sda.Msg.(SignatureResponse)
		var requestBuff *RequestBuffer
		var ok bool
		if requestBuff, ok = jv.requests[sigResponse.RequestNo]; !ok {
			// Not good, someone ask for a request we did not produce a shared
			// secret before .. ??
			dbg.Error("Receive signature request with request number nto matching any shared secret...")
			continue
		}
		requestBuff.AddSignatureResponse(sigResponse)
	}

}

// waitForRandom simply receives Random messages and put them in the appropriate
// RandomBuffer.
func (jv *JVSSProtocol) waitForRandom() {
	for random := range jv.rdChan {
		var reqBuff *RequestBuffer
		var ok bool
		if reqBuff, ok = jv.requests[random.RequestNo]; !ok {
			// we didn't started this new shared secret request so we should
			// pariticipate in.
			reqBuff = jv.initRequestBuffer(random.RequestNo)
			go jv.setupRequestSecret(random.RequestNo)
		}
		reqBuff.AddRandom(random)
	}
}

// waitForRequests waits for SignatureRequest message. It checks if we have the
// generated random for this request number and if so sends back a partialSig to
// the origin of the request.
func (jv *JVSSProtocol) waitForRequests() {
	for sda := range jv.reqChan {
		sigRequest := sda.Msg.(SignatureRequest)
		var requestBuff *RequestBuffer
		var ok bool
		if requestBuff, ok = jv.requests[sigRequest.RequestNo]; !ok {
			// Not good, someone ask for a request we did not produce a shared
			// secret before .. ??
			dbg.Error("Receive signature request with request number nto matching any shared secret...")
			continue
		}
		if requestBuff.secret == nil {
			dbg.Error("Received signature request with no secret generated :/")
			continue
		}
		if jv.schnorr != nil {
			dbg.Error("Received signature request without even the longterm secret set")
			continue
		}
		// create new round  == request
		h := jv.Host.Suite().Hash()
		h.Write(sigRequest.Msg)
		if err := jv.schnorr.NewRound(requestBuff.secret, h); err != nil {
			dbg.Error("Can not start new round")
			continue
		}
		// generate the partial sig
		ps := jv.schnorr.RevealPartialSig()
		sr := &SignatureResponse{
			RequestNo: sigRequest.RequestNo,
			Partial:   ps,
		}
		// send it back to the originator
		jv.Host.SendSDA(jv.Token, sda.From, sr)
	}
}

// waitForLongterm waits on a channel that receive every deals to be accepted
// for computeing the longterm distributed secret
func (jv *JVSSProtocol) waitForLongterm() {
	var nbDeal int
	fmt.Println("waitForLongterm()", jv.index)
	for lt := range jv.ltChan {
		deal := lt.Deal(jv.Host.Suite(), jv.Info)
		if _, err := jv.ltReceiver.AddDeal(lt.Index, deal); err != nil {
			dbg.Error("Error adding deal to longterm receiver")
			continue
		}
		nbDeal++
		if nbDeal >= jv.Info.T-1 {
			break
		}
	}
	sh, err := jv.ltReceiver.ProduceSharedSecret()
	if err != nil {
		dbg.Error("Could not produce shared secret", err)
	}
	jv.longterm = sh
	jv.schnorr.Init(jv.Host.Suite(), jv.Info, jv.longterm)
	// notify we have the longterm secret
	fmt.Println("waitForLOngterm() CHANNEL", jv.index)
	jv.longtermDone <- true
	// callbacks !
	if jv.onLongtermDone != nil {
		jv.onLongtermDone(jv.longterm)
	}
	fmt.Println("waitForLOngterm() DONE", jv.index)
}

func (jv *JVSSProtocol) setupLongtermReceiver() {
	// init the longterm with our deal
	fmt.Println("SetupLongTermReceiver()", jv.index)
	receiver := poly.NewReceiver(jv.Host.Suite(), jv.Info, &jv.key)
	jv.ltReceiver = receiver
}
func (jv *JVSSProtocol) SetupDistributedSchnorr() error {
	// produce your own deal
	deal := jv.newDeal()
	_, err := jv.ltReceiver.AddDeal(jv.index, deal)
	if err != nil {
		dbg.Error("Error adding our own deal:", err)
	}
	// Send the deal
	buf, err := deal.MarshalBinary()
	if err != nil {
		return err
	}
	jv.otherNodes(func(tn *sda.TreeNode) {
		lt := Longterm{
			Bytes: buf,
			Index: jv.nodeToIndex[tn.Id],
		}
		jv.Host.SendSDAToTreeNode(jv.Token, tn, &lt)
	})
	// wait until we know the longterm has been created

	fmt.Println("SetupDistributedSchnorr() JVSS WAITING", jv.index)
	select {
	case <-jv.longtermDone:
		fmt.Println("SetupDistributedSchnorr() JVSS DONE", jv.index)
		return nil
	case <-time.After(time.Second * 60):
		return errors.New("Could not have the longterm secret generated in time .. ??")
	}
	return nil
}

// setupDistributedSecret is called by the leader or the iniator that wants to
// start a new round, a new signing request, where we must first create a random
// distributed secret
func (jv *JVSSProtocol) setupDistributedSecret() (*RequestBuffer, error) {
	jv.lastRequestNo++
	return jv.setupRequestSecret(jv.lastRequestNo)

}

// setupRequestSecret sets up the random distributed secret for this request
// number. When the initiator starts a new request,peers will call this function
// so they also get the random dis. secret.
func (jv *JVSSProtocol) setupRequestSecret(requestNo int) (*RequestBuffer, error) {
	// prepare our deal
	requestBuff := jv.initRequestBuffer(requestNo)
	doneChan := make(chan *poly.SharedSecret)
	requestBuff.SetSecretChan(doneChan)
	deal := jv.newDeal()
	requestBuff.AddDeal(jv.index, deal)
	// send to everyone
	buf, err := deal.MarshalBinary()
	if err != nil {
		return nil, err
	}
	rand := Random{
		RequestNo: requestNo,
		Longterm: Longterm{
			Bytes: buf,
			Index: jv.index,
		},
	}
	jv.otherNodes(func(tn *sda.TreeNode) {
		jv.Host.SendSDAToTreeNode(jv.Token, tn, &rand)
	})
	// wait for the shared secret
	_ = <-doneChan
	requestBuff.ResetSecretChan()
	return requestBuff, nil

}

func (jv *JVSSProtocol) newDeal() *poly.Deal {
	dealKey := cliutils.KeyPair(jv.Host.Suite())
	deal := new(poly.Deal).ConstructDeal(&dealKey, &jv.key, jv.Info.T, jv.Info.R, jv.publicList)
	return deal
}

// RequestBuffer holds every info for the many distributed secrets we may need to
// compute in parallel. It also holds the partials signatures related to this
// request used for signing.
type RequestBuffer struct {
	// for which request number this buffer is
	requestNo int
	// The deals we have received so far for generating this rndom secret
	deals []*poly.Deal
	// the receiver aggregating them
	receiver *poly.Receiver
	// the generated secret if any
	secret *poly.SharedSecret
	// generated secret flag
	secretGend bool
	// channel to say the random secret has been generated
	secretChan chan *poly.SharedSecret
	// the channel to say the final signature related has been generated
	sigChan chan *poly.SchnorrSig
	// generated signature flag
	sigGend bool
	// The partial signatures aggregated until now
	partials []*poly.SchnorrPartialSig
	// the schnorr struct used to aggregate the partial sig
	schnorr *poly.Schnorr
	// the signature itself
	signature *poly.SchnorrSig
	// the info about the JVSS config
	info poly.Threshold
	// the suite used
	suite abstract.Suite
}

func (rb *RequestBuffer) SetSecretChan(ch chan *poly.SharedSecret) {
	rb.secretChan = ch
}
func (rb *RequestBuffer) ResetSecretChan() {
	close(rb.secretChan)
	rb.secretChan = nil
}
func (rb *RequestBuffer) SetSigChan(ch chan *poly.SchnorrSig) {
	rb.sigChan = ch
}
func (rb *RequestBuffer) ResetSigChan() {
	close(rb.sigChan)
	rb.sigChan = nil
}

// AddDeal is same as AddRandom but for Deal  (struct vs []byte)
func (rb *RequestBuffer) AddDeal(index int, deal *poly.Deal) {
	rb.deals = append(rb.deals, deal)
	_, err := rb.receiver.AddDeal(index, deal)
	if err != nil {
		dbg.Error("Could not add deal", err)
		return
	}
	if len(rb.deals) >= rb.info.T {
		// did we already generated it
		if !rb.secretGend {
			sh, err := rb.receiver.ProduceSharedSecret()
			if err != nil {
				dbg.Error("Could not produce shared secret:", err)
				return
			}
			rb.secret = sh
			rb.secretGend = true
		}
		// notify any interested party
		if rb.secretChan != nil {
			go func() { rb.secretChan <- rb.secret }()
		}
	}
}

// AddRandom add the RandomMessage and check if we can generate the secret
// already
func (rb *RequestBuffer) AddRandom(rand Random) {
	if rand.RequestNo != rb.requestNo {
		return
	}
	deal := rand.Deal(rb.suite, rb.info)
	rb.AddDeal(rand.Index, deal)

}

func (rb *RequestBuffer) AddSignatureResponse(partialSig SignatureResponse) {
	if partialSig.RequestNo != rb.requestNo {
		return
	}
	rb.partials = append(rb.partials, partialSig.Partial)
	if err := rb.schnorr.AddPartialSig(partialSig.Partial); err != nil {
		dbg.Error("Could not add partial signature to request buffer")
		return
	}
	if len(rb.partials) >= rb.info.T {
		if !rb.sigGend {
			sign, err := rb.schnorr.Sig()
			if err != nil {
				dbg.Error("Could not generated final signature:", err)
				return
			}
			rb.signature = sign
			rb.sigGend = true
			// notify interested party
			if rb.sigChan != nil {
				go func() { rb.sigChan <- sign }()
			}
		}
	}
}

// initrequestBuffer init a random buffer for this request number
func (jv *JVSSProtocol) initRequestBuffer(rNo int) *RequestBuffer {
	rd := &RequestBuffer{
		requestNo:  rNo,
		deals:      make([]*poly.Deal, 0),
		receiver:   poly.NewReceiver(jv.Host.Suite(), jv.Info, &jv.key),
		schnorr:    jv.schnorr,
		secretChan: nil,
		sigChan:    nil,
		partials:   make([]*poly.SchnorrPartialSig, 0),
		info:       jv.Info,
		suite:      jv.Host.Suite(),
	}
	jv.requests[rNo] = rd
	return rd
}

func (jv *JVSSProtocol) otherNodes(fn func(*sda.TreeNode)) {
	if !jv.Tree.Root.Entity.Equal(jv.Host.Entity) {
		fn(jv.Tree.Root)
	}
	for _, tn := range jv.Tree.Root.Children {
		if !tn.Entity.Equal(jv.Host.Entity) {
			fn(tn)
		}
	}
}

func (jv *JVSSProtocol) RegisterOnLongtermDone(fn func(*poly.SharedSecret)) {
	jv.onLongtermDone = fn
}
