package main

import (

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/crypto/abstract"
	//"github.com/dedis/crypto/ppsi_crypto_utils"
	)

var Name = "PPSI"

//0 root-initiateing authority
//1-n-2 authorities
//n-1 repository

func init() {
	sda.GlobalProtocolRegister(Name, NewPPSI)
}

type PPSI struct {
	
	hasPlain int
	
	finalIntersection []string
	
	setsToIntersect int
	
	sets int

	id int //?
	
	numAuthorities int
	
	tempIntersection []abstract.Point
	
	IdsToInterset []int
	
	SetsReq chan chanSetsRequest
	
	ElgEncrypted chan chanElgEncryptedMessage
	
	FullyPhEncrypted chan chanFullyPhEncryptedMessage
	
	EncryptedSets map[int][]map[int]abstract.Point
	
	PartiallyPhDecrypted chan chanPartiallyPhDecryptedMessage
	
	PlainMess chan chanPlainMessage
	
	done chan bool
  
        MAX_SIZE int // need to initialize it-user arguments? configuration file?

	next int 
	
	*sda.TreeNodeInstance
	
	treeNodeID sda.TreeNodeID
	
	ppsi *ppsi_crypto_utils.PPSI
	
	
	
}


	
func NewPPSI(node *sda.TreeNodeInstance) (sda.ProtocolInstance, error) { 
	var err error
	
	publics := make([]abstract.Point, len(node.Roster().List))
	for i, e := range node.Roster().List {
		publics[i] = e.Public
	}

	c := &PPSI{
		ppsi:             ppsi_crypto_utils.NewPPSI(node.Suite(), node.Private(), publics),
		TreeNodeInstance: node,
		done:             make(chan bool),
                
	}
	
	
	
	c.numnodes =len(node.Roster().List)
	c.next = c.id+1
	//c.next := c.id+1
	 if c.id== c.numnodes-2{ 
		c.next = 0
	}
	  if c.id== c.numnodes-1 {
		c.next = 0
	}
	
	
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



func (c *PPSI) Start() error {
	var id int
	out := &SetsRequest{
	    SetsIds: c.IdsToInterset,
	    numAuthorities : c.numAuthorities,
	
		}
	//id =c.numnodes-1
	return //c.SendTo(id  , out)
}


func (c *PPSI) handleSetsRequest(in *SetsRequest) error {
	
	
for i, e := range c.TreeNodeInstance.Roster().List {
		
	if e!=c.TreeNodeInstance{ //invalid operation: e != c.TreeNodeInstance (mismatched types *network.ServerIdentity and *sda.TreeNodeInstance)
		users := make(map[int]int, in.numAuthorities)
		
		out :=c.EncryptedSets[in.SetsIds[i]]
		outMsg := &ElgEncryptedMessage{
				Content: out,
				users: users,   
				mode: 0,
				numPhones: len(out),
				sets : len(SetsIds),
				}
		if err := c.SendTo(e, outMsg); err != nil {
			return err
		
	}
	}
}

return nil
}

func(c *PPSI) handleElgEncryptedMessage(in *ElgEncryptedMessage) error {
	c.sets= in.sets
     phones := in.Content
	 if !c.IsLastToDecElg(in.users[c.id]) {
	 	c.ppsi.initPPSI(in.numPhones)
	   
		out := c.ppsi.DecryptElgEncrptPH(phones, c.id)   //SHUFFLE?
	
		in.users[c.id] = in.users[c.id]+1 //need to decide on unique id-maybe the token that each node has?
	
		outMsg := &ElgEncryptedMessage{
			 Content: out,
			 users: in.users,   
			 mode: in.mode,
			}
		
		return c.SendTo(c.next, outMsg)
	}
	
	 if c.IsLastToDecElg(in.users[c.id]) {
	    encPH := c.ppsi.ExtractPHEncryptions(phones)
		c.tempIntersection = encPH
		outMsg := &FullyPhEncryptedMessage{
			Content: encPH,
			users: in.users,
			mode: 1, 
			}
		return c.handleFullyPhEncryptedMessage(outMsg)
	
	}

}

func (c *PPSI) handleFullyPhEncryptedMessage(in *FullyPhEncryptedMessage) error {

    phones := in.Content
	 
	if !c.IsLastToIntersect(in.users[c.id]) { //first time I get the message
	
		c.computeIntersection(phones)
	
		c.setsToIntersect = c.setsToIntersect+1
		
		if c.setsToIntersect<c.sets {
	
			in.users[c.id] = in.users[c.id]+1
			outMsg := &FullyPhEncryptedMessage{
				Content: in.Content,
				users: in.users,
				mode: in.mode,
					}
		
			return c.SendTo(c.next, outMsg)
		}
	
		if c.setsToIntersect==c.sets {
	    
			if !c.wantToDecrypt() {
				return c.handleIllegalIntersection()
			}
			
			if c.wantToDecrypt(){
				out := c.ppsi.DecryptPH( c.tempIntersection )
				in.users[c.id] = in.users[c.id]+1
	
				outMsg := &PartiallyPhDecryptedMessage{
					Content: out,
					users: in.users,
					mode: 2,
					}
	
				return c.SendTo(c.next, outMsg)
	
			}
		}
	}
	
	if c.IsLastToIntersect(in.users[c.id]) { // already receives the message-hence already computed the intersection
	
		if c.setsToIntersect==c.sets {
	        
			if !c.wantToDecrypt() {
				return c.handleIllegalIntersection()
			}
			
			if c.wantToDecrypt(){
				out := c.ppsi.DecryptPH( c.tempIntersection )
				in.users[c.id] = in.users[c.id]+1
	
				outMsg := &PartiallyPhDecryptedMessage{
					Content: out,
					users: in.users,
					mode: 2,
					}
	
				return c.SendTo(c.next, outMsg)
			}
	    }
	}
	
	return nil
}
	
	 




func (c *PPSI) handlePartiallyPhDecryptedMessage(in *PartiallyPhDecryptedMessage) error {
    phones := in.Content
	if !c.IsLastToDecPH(in.users[c.id]) {        //already decrypted it elgamal and encrypted it ph-so stage 0 is finished and if not-this is an error-need to add tests for those error scenarios
	  
		out := c.ppsi.DecryptPH( phones ) 
		in.users[c.id] = in.users[c.id]+1
		outMsg := &PartiallyPhDecryptedMessage{
			Content: out,
			users: in.users,
			mode: in.mode,
				}
		
		return c.SendTo(c.next, outMsg)
	}
	
	if c.IsLastToDecPH(in.users[c.id]) {
	    out := c.ppsi.ExtractPlains( phones )
		in.users[c.id] = in.users[c.id]+1
		outMsg := &PlainMessage{
			Content: out,
			users: in.users,
			mode: 3,
				}
	
		return c.handlePlainMessage(outMsg)
	}
return nil
}


func (c *PPSI) handlePlainMessage(in *PlainMessage) error {

	
	
	c.finalIntersection=in.Content
	
	if !c.IsLast(in.users[c.id]) {
		in.users[c.id] = in.users[c.id]+1
	
		outMsg := &PlainMessage{
			Content: in.Content,
			users: in.users,
			mode: in.mode,
			}
		
		return c.SendTo(c.next, outMsg)
	}
	
	if c.IsLast(in.users[c.id]) {
		if c.IsRoot() {
		   c.hasPlain =c.hasPlain+1
		   }
		if !c.IsRoot(){
	outMsg := &DoneMessage{
			Src: c.id,//??
			}
		
		return c.SendTo(c.Parent(), outMsg)
		}
	
	}
	return nil
}

func (c *PPSI) handleDoneMessage(in *DoneMessage) error {
	//if c.IsRoot() {
		//   c.done =c.done+1//locks??
//	} 
		
	//	if c.done < c.authorities{
	//		return nil
	//	}

	//	if c.done==  c.sets {
		//done <- true
		//	return nil
		//}
		
		defer func() {
		// protocol is finished
		close(c.done)
		c.Done()
	}()

return nil			
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
func (c *PPSI) IsLast(num int) bool{
	return num==4
}

func (c *PPSI) IsLastToDecPH( num int) bool{
	return num==3
}

func(c *PPSI)IsLastToIntersect(num int) bool{
	return num==2
}

func (c *PPSI)IsLastToDecElg(num int) bool {
	return num==1
}

func (c *PPSI) wantToDecrypt() bool{
	return len(c.tempIntersection)>c.MAX_SIZE
}


func (c *PPSI) computeIntersection(newSet []abstract.Point) {

	var newTempIntersection []abstract.Point
	OUTER:
	for i:=0; i<len(c.tempIntersection) ; i++ {
		for v := 0; v < len(newSet); v++ {
			if  c.tempIntersection[i]== newSet[v]{//equaity of points??
				newTempIntersection.append(newTempIntersection, c.tempIntersection[i])
				continue OUTER
			}
		}
	}
	
	c.tempIntersection = newTempIntersection 
}
	
