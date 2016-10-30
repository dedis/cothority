
package ppsi 

import (
	
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/nist"
	"github.com/dedis/crypto/random"
	"github.com/dedis/crypto/ElGamal"
	"github.com/dedis/crypto/PohligHellman"
	
)

//ids of authorities should be contigous from 0
//the list of privates should be replaced with c.private

type PPSI struct {
	
	suite abstract.Suite
	publics []abstract.Point
	private []abstract.Scalar //private abstract.Scalar
	EncryptedPhoneSet []map[int]abstract.Point
	encPH []abstract.Point
	plain []string
	
}

func NewPPSI(suite abstract.Suite, private []abstract.Scalar, publics []abstract.Point) *PPSI {
	ppsi := &PPSI{
		suite:   suite,
		private: private,
		publics: publics,
		
	}
	
	return ppsi
}

 
func (c *PPSI) initPPSI(numPhones int)  {
		c.EncryptedPhoneSet =  make([]map[int]abstract.Point, numPhones)
		c.encPH =  make([]abstract.Point, numPhones)
		c.plain =  make([]string, numPhones)
}


func  (c *PPSI) MultipleElgEncryption( message string, ids int) (
	cipher map[int]abstract.Point) {
		
		 
		cipher = make(map[int]abstract.Point)
		
		
		messageByte := []byte(message)
		K,C,_ := ElGamal.ElGamalEncrypt(c.suite, c.publics[0], messageByte)
		cipher[0] =K
		cipher[-1] =C
		
		
		for v := 1; v < ids; v++ {
			data := cipher[-1]
			K,C,_ :=  ElGamal.PartialElGamalEncrypt(c.suite, c.publics[v],data)
				cipher[v] =K
				cipher[-1] =C
					
		}
		
		return cipher
		
	}
	

func (c *PPSI) EncryptionOneSetOfPhones(set []string, ids int){
	    
	for v := 0; v < len(set); v++ {
		 cipher := c.MultipleElgEncryption(set[v], ids)
		 c.EncryptedPhoneSet[v] = cipher
	
	}
	
}


	
	func  (c *PPSI) DecryptElgEncrptPH(set []map[int]abstract.Point, id int)(
		 UpdatedSet []map[int]abstract.Point){
	 
	     UpdatedSet =  make([]map[int]abstract.Point, len(set))
	   	 UpdatedSet=set
		for i:=0; i<len(set) ; i++ {
			cipher:= set[i]
			K := cipher[id]
			
			C := cipher[-1]
		
	   
			resElg, err := ElGamal.PartialElGamalDecrypt(c.suite, c.private[id], K, C )
			if err != nil {
		println("decryption failed: " + err.Error())}
	//		resPH, err := PohligHellman.PHEncrypt(resElg)
			//if err != nil {
	//	panic("decryption failed: " + err.Error())
	//} 
			 UpdatedSet[i][-1] = resElg
			 // UpdatedSet[i][-1] = resPH
			//  var byteMessage []byte
			//byteMessage, _ =    UpdatedSet[i][-1].Data()
		  // var message string
		  // message=string(byteMessage)
			//println("Decryption succeeded: " + message)
		}
		
		return 
		
	}
	
	func (c *PPSI) ExtractPHEncryptions(set []map[int]abstract.Point ){
	
	  
		for i:=0; i<len(set) ; i++ {
			cipher:= set[i]
			c.encPH[i] = cipher[-1]
			 var byteMessage []byte
			byteMessage, _ =  set[i][-1].Data()
		   var message string
		   message=string(byteMessage)
			println("Decryption succeeded: " + message)
			
		}
	}
	

	
	func (c *PPSI) DecryptPH(set []abstract.Point){
	
	  
		for i:=0; i<len(set) ; i++ {
			resPH, err := PohligHellman.PHDecrypt(set[i])
				if err != nil {
		panic("decryption failed: " + err.Error())
		}
		set[i] = resPH		
		}
	}
	
	func (c *PPSI) ExtractPlains(set []abstract.Point) {
	
	    var byteMessage []byte
	    var message string
	    a := c.suite.Scalar().Pick(random.Stream)
	    A := c.suite.Point().Mul(nil, a)
	    byteMessage, _ = A.Data()
		for i:=0; i<len(set) ; i++ {
			byteMessage, _ = set[i].Data()   
			message=string(byteMessage)
			c.plain[i] = message
			println("Decryption succeeded: " + c.plain[i])
				
		}
		
		
		
	}

func main() {
		var root *PPSI
		suite := nist.NewAES128SHA256P256()
		
		a := suite.Scalar().Pick(random.Stream) 
		A := suite.Point().Mul(nil, a)  
		b := suite.Scalar().Pick(random.Stream) 
		B := suite.Point().Mul(nil, b) 
		c := suite.Scalar().Pick(random.Stream) 
		C := suite.Point().Mul(nil, c) 
		set := []string{"543323345", "543323045", "843323345"}

		publics  := []abstract.Point{A,B,C}
		private  := []abstract.Scalar{a,b,c}
		root=NewPPSI(suite, private, publics)
		root.initPPSI(3)
		root.EncryptionOneSetOfPhones(set, 3)
		var set1,set2,set3 []map[int]abstract.Point
		set1 =root.DecryptElgEncrptPH(root.EncryptedPhoneSet,0)
	    set2   =root.DecryptElgEncrptPH(set1,1)
        set3  =root.DecryptElgEncrptPH(set2,2)
        root.ExtractPHEncryptions(set3)
        root.ExtractPlains(root.encPH)
        println("Decryption : " + root.plain[0])
        println("Decryption : " + root.plain[1])
        println("Decryption : " + root.plain[2])
   
}


