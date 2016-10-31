package protocol

import (
	"fmt"
	"sync"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/ppsi"
)

var Name = "PPSI"

func init() {
	sda.ProtocolRegisterName(Name, NewPPSI)
}

type PPSI struct {

	IdsToInterset []int
	
	SetsReq chan chanSetsRequest
	
	ElgEncrypted chan chanElgEncryptedMessage
	
	FullyPhEncrypted chan chanFullyPhEncryptedMessage
	
	EncryptedSets map[int][]abstract.Scalar
	
	PartiallyPhDecrypted chan chanPartiallyPhDecryptedMessage
	
	PlainMess chan chanPlainMessage
	
	done chan bool
  
        MAX_SIZE int // need to initialize it-user arguments? configuration file?
	
	FinalIntersectionHook finalIntersectionHook

	next *TreeNode 
	
	*sda.TreeNodeInstance
	
	treeNodeID sda.TreeNodeID
	
	ppsi *ppsi.PPSI
	
}

type  finalIntersectionHook func(in []abstract.Scalar) 
	
func NewPPSI(node *sda.TreeNodeInstance) (sda.ProtocolInstance, error) { 
	var err error

	c := &ppsi{
		ppsi:             ppsi.NewPPSI(node.Suite(), node.Private()),
		TreeNodeInstance: node,
		done:             make(chan bool),
                
	}
	
	c.next := // need to initialize it-user arguments? configuration file?
	
    if err := node.RegisterChannel(&c.SetsReq); err != nil {
		return c, err
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

	return c, err
}


func (c *PPSI) Dispatch() error {           
                                               
	for {
		var err error
		select {
		case packet := <-c.SetsReq:
			err = c.handleSetsRequest(&packet.SetsRequest)
		case packet := <-c.ElgEncrypted:
			err = c.handleElgEncryptedMessage(&packet.ElgEncryptedMessage)
		case packet := <-c.FullyPhEncrypted:
			err = c.handleFullyPhEncryptedMessage(&packet.FullyPhEncryptedMessage)
		case packet := <-c.PartiallyPhDecrypted:
			err = c.handlePartiallyPhDecryptedMessage(&packet.PartiallyPhDecryptedMessage)
		case packet := <-c.PlainMess:
			err = c.handlePlainMessage(&packet.PlainMessage)
		case <-c.done:
			return nil
		}
		if err != nil {
			log.Error("ProtocolPPSI -> err treating incoming:", err)
		}
	}
}


func (c *ppsi) Start() error {
	out := &SetsRequest{
	    SetsIds: IdsToInterset,
		}
	return c.SendTo(c.Parent(), out)
}



func (c *ppsi) handleSetsRequest(in *SetsRequest) error {
	var index i

	for _, node := range c.Children() {
		out=EncryptedSets[SetsIds[i]]
		outMsg := &ElgEncryptedMessage{
				Content: out,
				users: 0,   
				mode: 0,
				}
		if err := c.SendTo(node, outMsg); err != nil {
			return err
		}
		i := i+1
	}
	return nil

	
  

func (c *ppsi) handleElgEncryptedMessage(in *ElgEncryptedMessage) error {
        phones := in.Content
	if !c.IsLastToDecElg() {//already decrypted it elgamal and encrypted it ph-so stage 0 is finished and if not-this is an error-need to add tests for those error scenarios
	
		out := c.DecryptElgEncrptPH( phones)   
	
		in.users[id] = in.users[id]+1 //need to decide on unique id-maybe the token that each node has?
	
		outMsg := &ElgEncryptedMessage{
			 Content: out,
			 users: in.users,   //reference? value?
			 mode: in.mode,
			}
		
		return c.SendTo(c.next, outMsg)
	}
	
	else {
	        encPH := ExtractPHEncryptions(phones)
		outMsg := &FullyPhEncryptedMessage{
			Content: encPH,
			users: in.users,
			mode: 1, 
			}
		return c.handleFullyPhEncryptedMessage(outMsg)
	
	}

}

func (c *ppsi) handleFullyPhEncryptedMessage(in *FullyPhEncryptedMessage) error {
        phones := in.Content
	if !c.IsLastToIntersect() {
		intersect, size=c.computeIntersection( phones)
	
		if !wantToDecrypt(size) {
			return c.handleIllegalIntersection(size)}
		}
	
		else {

		in.users[id] = in.users[id]+1
		outMsg := &FullyPhEncryptedMessage{
			Content: in.Content,
			users: in.users,
			mode: in.mode,
				}
		
		return c.SendTo(c.next, outMsg)
		}
	
	else{
	
		out := c.DecryptPH( phones)
		in.users[id] = in.users[id]+1
	
		outMsg := &PartiallyPhDecryptedMessage{
			Content: out,
			users: in.users,
			mode: 2,
			}
	
		return c.SendTo(c.next, outMsg)
	
	}
}



func (c *ppsi) handlePartiallyPhDecryptedMessage(in *PartiallyPhDecryptedMessage) error {
        phones := in.Content
	if !c.IsLastToDecPH() {        //already decrypted it elgamal and encrypted it ph-so stage 0 is finished and if not-this is an error-need to add tests for those error scenarios
	
		out := c.DecryptPH( phones)  
		in.users[id] = in.users[id]+1
		outMsg := &PartiallyPhDecryptedMessage{
			Content: out,
			users: in.users,
			mode: in.mode,
				}
		
		return c.SendTo(c.next, outMsg)
	}
	
	else {
		out := c.ExtractPlains( phones )
		in.users[id] = in.users[id]+1
		outMsg := &PlainMessage{
			Content: out,
			users: in.users,
			mode: 3
				}
	
		return c.handlePlainMessage(outMsg)
	}

}


func (c *ppsi) handlePlainMessage(in *PlainMessage) error {

	defer func() {
		close(c.done)
		c.Done()
	}()
	
	c.finalIntersection=in.Content
	
	if !c.IsLast() {
		in.users[id] = in.users[id]+1
	
		outMsg := &PlainMessage{
			Content: in.Content
			users: in.users,
			mode: in.mode
			}
		
		return c.SendTo(c.next, outMsg)
	}
	
	else {
	//query some accumulated field using c.FinalIntersectionHook
	return nil
	
	}
	
}

func (c *ppsi) handleIllegalIntersection(size int) error {
  //to be implemented
}

func (c *ppsi) IsLast() {
	return users[c.id]==4
}

func (c *ppsi) IsLastToDecPH() {
	return users[c.id]==3
}

func (c *ppsi) IsLastToIntersect() {
	return users[c.id]==2
}

func (c *ppsi) IsLastToDecElg() {
	return users[c.id]==1
}

func (c *ppsi) wantToDecrypt(size int) {
	return size>MAX_SIZE
}
	
func (c *ppsi) RegisterfinalIntersectionHook(fn finalIntersectionHook) {
	c.FinalIntersectionHook = fn
}

func (c *ppsi) computeIntersection(newSet []abstract.Point) {
	
var newTempIntersection []abstract.Point
	OUTER:
	for i:=0; i<len(tempIntersection) ; i++ {
		for v := 0; v < len(newSet); v++ {
			if  tempIntersection[i]== newSet[v]{//equaity of points??
				newTempIntersection.append(newTempIntersection, tempIntersection[i])
				continue OUTER
			}
		}
	}
	
	tempIntersection := newTempIntersection //possible?
}

	

