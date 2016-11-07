
package ppsi 

import (
	
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/nist"
	"github.com/dedis/crypto/random"
	"github.com/dedis/crypto/ElGamal"
	"fmt"
)



type PPSI struct {
	
	ids int
	encKey abstract.Scalar
	decKey abstract.Scalar
	suite abstract.Suite
	publics []abstract.Point
	private abstract.Scalar 
	
	
	
}

func NewPPSI(suite abstract.Suite, private abstract.Scalar, publics []abstract.Point) *PPSI {
	ppsi := &PPSI{
		suite:   suite,
		private: private,
		publics: publics,
		
	}
	
	ppsi.createKeys()
	return ppsi
}

 
func (c *PPSI) initPPSI(numPhones int, ids int)  {
//		c.EncryptedPhoneSet =  make([]map[int]abstract.Point, numPhones)
		c.ids =  ids
		
		
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
	

func (c *PPSI) EncryptionOneSetOfPhones(set []string, ids int)(
		EncryptedPhoneSet []map[int]abstract.Point){
			
	EncryptedPhoneSet= make([]map[int]abstract.Point, len(set))
	for v := 0; v < len(set); v++ {
		 cipher := c.MultipleElgEncryption(set[v], ids)
		 EncryptedPhoneSet[v] = cipher
	
	}
	 
	return
	
}


func  (c *PPSI) DecryptElgEncrptPH(set []map[int]abstract.Point, id int)(
		 UpdatedSet []map[int]abstract.Point){
	
	     UpdatedSet =  make([]map[int]abstract.Point, len(set))
	     UpdatedSet=set
	   	 
	     for i:=0; i<len(set) ; i++ {
		cipher:= set[i]
		K := cipher[id]
		C := cipher[-1]
		
	 
		resElg, _ := ElGamal.PartialElGamalDecrypt(c.suite, c.private, K, C )
			
		resPH := c.PHEncrypt(resElg)
		
		UpdatedSet[i][-1] = resPH
			  
		     for j:=0; j<c.ids  ; j++ {
			  res2PH := c.PHEncrypt(cipher[j])
			  UpdatedSet[i][j] = res2PH
		     }
			  
		}
		
		return 
}
		 
		 
	
func (c *PPSI) ExtractPHEncryptions(set []map[int]abstract.Point )(
		  encryptedPH []abstract.Point){
	    encryptedPH =  make([]abstract.Point, len(set))
	   
	  
	    for i:=0; i<len(set) ; i++ {
			cipher:= set[i]
			encryptedPH[i] = cipher[-1]
			
			
		}
	
	    return
}
	


	
func (c *PPSI) DecryptPH(set []abstract.Point)(UpdatedSet []abstract.Point){
	
	     UpdatedSet =  make([]abstract.Point, len(set))
	     UpdatedSet=set
	
	  
	     for i:=0; i<len( UpdatedSet) ; i++ {
		    resPH := c.PHDecrypt(UpdatedSet[i])
		    UpdatedSet[i] = resPH		
		}
		
	     return
}
	
	
func (c *PPSI) ExtractPlains(set []abstract.Point)(
		  plain []string){
	    plain =  make([]string, len(set)) 
	
	 var byteMessage []byte
	 var message string
	   
	 for i:=0; i<len(set) ; i++ {
		byteMessage, _ = set[i].Data()   
		message=string(byteMessage)
		plain[i] = message
			
				
	 }
		
	return
		
}
		  
func   (c *PPSI) createKeys(){
		  	
	enckey:= c.suite.Scalar().Pick(random.Stream) 
	
	for  !c.suite.Scalar().Gcd(enckey).Equal(c.suite.Scalar().One()) {
		enckey= c.suite.Scalar().Pick(random.Stream)}
	
	c.encKey = enckey
	c.decKey= c.suite.Scalar().Inv(enckey)
	
}

func  (c *PPSI) PHDecrypt(cipher abstract.Point)(
	S abstract.Point) {
		
	  S = c.suite.Point().Mul(cipher, c.decKey)
	  return 
	  
	}
	
func  (c *PPSI) PHEncrypt(M abstract.Point)(
	S  abstract.Point) {
	
	  S = c.suite.Point().Mul(M, c.encKey) 
	  return
	}

func main() {
	
		var c1 *PPSI
		var c2 *PPSI
		var c3 *PPSI
		
		var rep *PPSI
		
		suite := nist.NewAES128SHA256P256()
		
		a := suite.Scalar().Pick(random.Stream) 
		A := suite.Point().Mul(nil, a)  
		b := suite.Scalar().Pick(random.Stream) 
		B := suite.Point().Mul(nil, b) 
		c := suite.Scalar().Pick(random.Stream) 
		C := suite.Point().Mul(nil, c) 
		
		d := suite.Scalar().Pick(random.Stream) 
//		D := suite.Point().Mul(nil, d) 
		
		set := []string{"543323345", "543323045", "843323345"}

		publics  := []abstract.Point{A,B,C}
		private1  := a
		private2  := b
		private3  := c
		private4  := d
		
		c1=NewPPSI(suite, private1, publics)
		c2=NewPPSI(suite, private2, publics)
		c3=NewPPSI(suite, private3, publics)
		rep=NewPPSI(suite, private4, publics)
		
		c1.initPPSI(3,3)
		c2.initPPSI(3,3)
		c3.initPPSI(3,3)
		rep.initPPSI(3,3)
		
	
		//c1.createKeys()
	//	c2.createKeys()
	//	c3.createKeys()
		// cipher := root.PHEncrypt(message)
		//  root.PHDecrypt(cipher)
		  
		
		var set1,set2,set3 []map[int]abstract.Point
		var set4,set5,set6,set7 []abstract.Point
		var set8 []string
		var set0 []map[int]abstract.Point
		
		set0=rep.EncryptionOneSetOfPhones(set, 3)
		set1 =c1.DecryptElgEncrptPH(set0,0)
	        set2   =c2.DecryptElgEncrptPH(set1,1)
        	set3  =c3.DecryptElgEncrptPH(set2,2)
        	set4  =c3.ExtractPHEncryptions(set3)
         
        	set5  = c3.DecryptPH(set4)
        	set6  = c1.DecryptPH(set5)
        	set7  = c2.DecryptPH(set6)
        
         	set8  = c2.ExtractPlains(set7)
         	println("Decryption : " + set8[0])
         	println("Decryption : " + set8[1])
         	println("Decryption : " + set8[2])
   
}
