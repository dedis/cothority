package ppsi

import (
	"fmt"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/ppsi_crypto_utils"
)

func init() {
	sda.GlobalProtocolRegister("PPSI", NewPPSI)
}

type PPSI struct {
	*sda.TreeNodeInstance

	dec bool

	first bool

	hasPlain int

	publics []abstract.Point

	setreq chan chanSetsRequest

	donreq chan chanDoneMessage

	initreq chan chanInitiateRequest

	treeIndex int

	finalIntersection []string

	setsToIntersect int

	NumAuthorities int

	tempIntersection []abstract.Point

	IdsToInterset []int

	ElgEncrypted chan chanElgEncryptedMessage

	FullyPhEncrypted chan chanFullyPhEncryptedMessage

	EncryptedSets [][]map[int]abstract.Point

	PartiallyPhDecrypted chan chanPartiallyPhDecryptedMessage

	PlainMess chan chanPlainMessage

	done chan bool

	MAX_SIZE int

	next int

	treeNodeID sda.TreeNodeID

	ppsi *ppsi_crypto_utils.PPSI

	numnodes      int
	signatureHook SignatureHook
}

type SignatureHook func()

func NewPPSI(node *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
	var err error

	n := len(node.List())
	publics := make([]abstract.Point, n)

	var idx int

	for i, tn := range node.List() {
		if tn.ServerIdentity.Public.Equal(node.Public()) {
			idx = i
		}
		publics[i] = tn.ServerIdentity.Public
	}

	numnodes := len(node.List())
	NumAuthorities := numnodes - 1
	c := &PPSI{
		ppsi:             ppsi_crypto_utils.NewPPSI(node.Suite(), node.Private(), publics, NumAuthorities),
		TreeNodeInstance: node,

		publics:   publics,
		treeIndex: idx,
	}

	c.MAX_SIZE = 6 //how do i receive it
	c.dec = false
	c.first = true
	c.numnodes = numnodes
	c.NumAuthorities = NumAuthorities
	c.next = c.treeIndex + 1
	//c.next := c.id+1
	if c.treeIndex == c.numnodes-2 {
		c.next = 0
	}
	if c.treeIndex == c.numnodes-1 {
		c.next = 0
	}

	if err = node.RegisterChannel(&c.setreq); err != nil {
		return nil, err
	}
	if err = node.RegisterChannel(&c.initreq); err != nil {
		return nil, err
	}

	if err := node.RegisterChannel(&c.ElgEncrypted); err != nil {
		return c, err
	}
	if err := node.RegisterChannel(&c.FullyPhEncrypted); err != nil {
		return c, err
	}
	if err := node.RegisterChannel(&c.PartiallyPhDecrypted); err != nil {
		return c, err
	}
	if err := node.RegisterChannel(&c.PlainMess); err != nil {
		return c, err
	}
	if err = node.RegisterChannel(&c.donreq); err != nil {
		return nil, err
	}
	return c, err
}

func (c *PPSI) Index() int {
	return c.treeIndex
}

func (c *PPSI) Dispatch() error {
	for {
		var err error
		select {
		case packet := <-c.setreq:
			err = c.handleSetsRequest(&packet.SetsRequest)
		case packet := <-c.ElgEncrypted:
			err = c.handleElgEncryptedMessage(&packet.ElgEncryptedMessage)
		case packet := <-c.FullyPhEncrypted:
			err = c.handleFullyPhEncryptedMessage(&packet.FullyPhEncryptedMessage)
		case packet := <-c.initreq:
			err = c.handleInitiateRequest(&packet.Init)

		case packet := <-c.PartiallyPhDecrypted:
			err = c.handlePartiallyPhDecryptedMessage(&packet.PartiallyPhDecryptedMessage)
		case packet := <-c.donreq:
			err = c.handleDoneMessage(&packet.DoneMessage)
		case <-c.done:
			return nil
		}
		if err != nil {
			fmt.Printf("%v\n", err)
		}

	}
}

func (c *PPSI) Start() error {
	if !c.IsRoot() {
		return fmt.Errorf("Called Start() on non-root ProtocolInstance")
	}
	if len(c.Children()) < 3 {
		return fmt.Errorf("At least 3 children needed ")
	}
	out := &Init{}
	return c.handleInitiateRequest(out)

}

func (c *PPSI) handleInitiateRequest(in *Init) error {
	for i, _ := range c.TreeNodeInstance.List() {

		out := c.EncryptedSets[i]
		users := make(map[int]int, c.NumAuthorities)

		for i := 0; i < c.NumAuthorities; i++ {
			users[i] = 0
		}

		//	out :=c.EncryptedSets[in.SetsIds[i]]
		//out :=c.EncryptedSets[i]

		outMsg := &ElgEncryptedMessage{
			Content:   out,
			Users:     users,
			NumPhones: len(out),
			Sets:      in.Num,
			ID:        i,
		}
		fmt.Printf("%v\n", outMsg.Sets)

		if i != c.Index() {
			if err := c.SendTo(c.List()[i], outMsg); err != nil {
				return err
			}
			if i == c.Index() {
				return c.handleElgEncryptedMessage(outMsg)
			}
			//	}

		}
	}
	return nil
}

//Decrypt one layer of elgamal with the conode private key
//Encrypt one layer with the conode PH key
//Send to next one
func (c *PPSI) handleElgEncryptedMessage(in *ElgEncryptedMessage) error {

	lim := c.NumAuthorities + 1

	phones := c.ConvertDim(in.Content, lim, in.numphones)

	if !c.IsLastToDecElg(in.Users[c.Index()]) {

		out := c.ppsi.DecryptElgEncrptPH(phones, c.Index())
		in.Users[c.Index()] = in.Users[c.Index()] + 1

		outMsg := &ElgEncryptedMessage{
			Content: out,
			Users:   in.Users,
			Sets:    in.Sets,
			ID:      in.ID,
		}

		return c.SendTo(c.List()[c.next], outMsg)
	}

	if c.IsLastToDecElg(in.Users[c.Index()]) {
		encPH := c.ppsi.ExtractPHEncryptions(phones)
		outMsg := &FullyPhEncryptedMessage{
			Content: encPH,
			Users:   in.Users,
			Mode:    1,
			Sets:    in.Sets,
			ID:      in.ID,
		}
		return c.handleFullyPhEncryptedMessage(outMsg)

	}
	return nil
}

//Compute the intersection of the new message with the temporary intersection currently stored in the conode.
func (c *PPSI) handleFullyPhEncryptedMessage(in *FullyPhEncryptedMessage) error {

	phones := in.Content

	if !c.IsLastToIntersect(in.Users[c.Index()]) { //first time I get the message

		c.computeIntersection(phones)

		if c.first == true {
			c.first = false
		}

		c.setsToIntersect = c.setsToIntersect + 1

		in.Users[c.Index()] = in.Users[c.Index()] + 1
		outMsg := &FullyPhEncryptedMessage{
			Content: in.Content,
			Users:   in.Users,
			Mode:    in.Mode,
			Sets:    in.Sets,
			ID:      in.ID,
		}

		c.SendTo(c.List()[c.next], outMsg)
	}

	if c.setsToIntersect == in.Sets && c.dec == false {
		c.dec = true
		if !c.wantToDecrypt() {
			return c.handleIllegalIntersection()
		}

		outMsg := &PartiallyPhDecryptedMessage{
			Content: c.tempIntersection,
			Users:   in.Users,
			Mode:    in.Mode,
			Sets:    in.Sets,
			ID:      in.ID,
		}
		return c.handlePartiallyPhDecryptedMessage(outMsg)
	}

	return nil
}

//Remove one layer of PH encryption with the conode PH key
func (c *PPSI) handlePartiallyPhDecryptedMessage(in *PartiallyPhDecryptedMessage) error {

	phones := in.Content
	if !c.IsLastToDecPH(in.Users[c.Index()]) { //already decrypted it elgamal and encrypted it ph-so stage 0 is finished and if not-this is an error-need to add tests for those error scenarios

		out := c.ppsi.DecryptPH(phones)
		in.Users[c.Index()] = in.Users[c.Index()] + 1
		outMsg := &PartiallyPhDecryptedMessage{
			Content: out,
			Users:   in.Users,
			Mode:    in.Mode,
			Sets:    in.Sets,
			ID:      in.ID,
		}

		return c.SendTo(c.List()[c.next], outMsg)
	}

	if c.IsLastToDecPH(in.Users[c.Index()]) {
		out := c.ppsi.ExtractPlains(phones)
		c.finalIntersection = out
		if c.IsRoot() {
			c.hasPlain = c.hasPlain + 1
		}
		if !c.IsRoot() {

			outMsg := &DoneMessage{
				Src:  c.Index(),
				Sets: in.Sets,
			}

			return c.SendTo(c.Parent(), outMsg)
		}

	}
	return nil
}

func (c *PPSI) handleDoneMessage(msg *DoneMessage) error {

	c.hasPlain = c.hasPlain + 1 //locks

	if c.hasPlain == msg.Sets {
		c.finish()

	}

	return nil

}

func (c *PPSI) finish() error {
	if c.signatureHook != nil {
		c.signatureHook()
	}

	return nil
}

func (c *PPSI) RegisterSignatureHook(fn SignatureHook) {
	c.signatureHook = fn
}

func (c *PPSI) handleIllegalIntersection() error {

	//		out := &IllegalIntersectionMessage{
	//		Content: size,
	//		}

	//	return c.SendTo(c.next, out)
	return nil

}

//func (c *PPSI) handleIllegalIntersectionMessage(in *IllegalIntersectionMessage) error {

//if c.IsRoot() {
//	 c.illegal=c.illegal+1

//}
//}
func (c *PPSI) IsLast(num int) bool {
	return num == 4
}

func (c *PPSI) IsLastToDecPH(num int) bool {
	return num == 3
}

func (c *PPSI) IsLastToIntersect(num int) bool {
	return num == 2
}

func (c *PPSI) IsLastToDecElg(num int) bool {
	return num == 1
}

func (c *PPSI) wantToDecrypt() bool {
	return len(c.tempIntersection) < c.MAX_SIZE
}

func (c *PPSI) computeIntersection(newSet []abstract.Point) {
	if c.first == true {
		c.tempIntersection = newSet
	}
	if c.first == false {
		var newTempIntersection []abstract.Point
	OUTER:
		for i := 0; i < len(c.tempIntersection); i++ {
			for v := 0; v < len(newSet); v++ {
				if c.tempIntersection[i].Equal(newSet[v]) {
					if !c.Contains(newTempIntersection, c.tempIntersection[i]) {
						newTempIntersection = append(newTempIntersection, c.tempIntersection[i])
						continue OUTER
					}
				}
			}
		}

		c.tempIntersection = newTempIntersection //possible?
	}
}

func (c *PPSI) Contains(elems []abstract.Point, elem abstract.Point) bool {
	for i := 0; i < len(elems); i++ {
		if elems[i].Equal(elem) {
			return true
		}
	}
	return false
}


func (c *PPSI) ConvertDim(phones []map[int]abstract.Point,lim int, numphones int) (
	phonesnew []map[int]abstract.Point){
	
	phonesnew:= make([]map[int]abstract.Point,0)

	for i:=0; i<numphones ; i++ {
		newarr:= make(map[int]abstract.Point)
		for j:=i*lim; j<i*lim+lim. ; j++ {
			for k := range phones[j] {
				 id:=k
				 val:=phones[j][k]
				 newarr[id]=val
			 }
		}
	
	
		phonesnew=append(phones,newarr)
	}

return
 
}
