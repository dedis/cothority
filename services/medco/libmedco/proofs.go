package libmedco

import (
	"github.com/dedis/cothority/log"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/proof"
)

type CompleteProof struct {
	//suite abstract.Suite
	Proof []byte
	B1    abstract.Point
	B2    abstract.Point
	A     abstract.Point
	B     abstract.Point
	C     abstract.Point
}

//proof creation for scheme switching on a ciphervector
func VectSwitchSchemeProof(suite abstract.Suite, k abstract.Scalar, s abstract.Scalar, Rjs []abstract.Point, C1 CipherVector, C2 CipherVector) []CompleteProof {
	var result []CompleteProof
	if len(C1) != len(C2) {
		log.Errorf("two vectors should have same size")
		return nil
	} else {
		for i, v := range C1 {
			result = append(result, SwitchSchemeProof(suite, k, s, Rjs[i], v.C, C2[i].C))
		}
	}
	return result
}

//proof creation for scheme switching
func SwitchSchemeProof(suite abstract.Suite, k abstract.Scalar, s abstract.Scalar, Rj abstract.Point, C1 abstract.Point, C2 abstract.Point) CompleteProof {
	return SwitchKeyProof(suite, k, s, Rj, suite.Point().Base(), C1, C2)
}

//proof creation for key switching on a ciphervector
func VectSwitchKeyProof(suite abstract.Suite, k abstract.Scalar, s abstract.Scalar, Rjs []abstract.Point, B2orU abstract.Point, C1 CipherVector, C2 CipherVector) []CompleteProof {
	var result []CompleteProof
	if len(C1) != len(C2) {
		log.Errorf("two vectors should have same size")
		return nil
	} else {
		for i, v := range C1 {
			result = append(result, SwitchKeyProof(suite, k, s, Rjs[i], B2orU, v.C, C2[i].C))
		}
	}
	return result
}

func VectSwitchToProbProof(suite abstract.Suite, s abstract.Scalar, rjs []abstract.Scalar, newK abstract.Point, C1 CipherVector, C2 CipherVector) []CompleteProof {
	var result []CompleteProof
	if len(C1) != len(C2) {
		log.Errorf("two vectors should have same size")
		return nil
	} else {
		for i, v := range C1 {
			result = append(result, SwitchToProbProof(suite, s, rjs[i], newK, v.C, C2[i].C))
		}
	}
	return result
}

//proof creation for key switching
func SwitchKeyProof(suite abstract.Suite, k abstract.Scalar, s abstract.Scalar, Rj abstract.Point, B2orU abstract.Point, C1 abstract.Point, C2 abstract.Point) CompleteProof {
	pred := CreatePredicate()

	B1 := suite.Point().Neg(Rj) // B1 = -rjB

	B2 := B2orU //B or U

	A := suite.Point().Mul(B1, k) // a = -rjBk = -rjK

	B := suite.Point().Mul(B2, s) // b = sB2

	C := suite.Point().Sub(C2, C1) // c = Ci - C(i-1)

	sval := map[string]abstract.Scalar{"k": k, "s": s}
	pval := map[string]abstract.Point{"B1": B1, "B2": B2, "a": A, "b": B, "c": C}
	prover := pred.Prover(suite, sval, pval, nil) // computes: commitment, challenge, response

	rand := suite.Cipher(abstract.RandomKey)

	Proof, err := proof.HashProve(suite, "TEST", rand, prover)
	_ = Proof
	if err != nil {
		log.Errorf("---------Prover:", err.Error())
	} else {

	}

	//if we want binaries
	//b1,_ := B1.MarshalBinary()
	//b2,_ := B2.MarshalBinary()
	//A,_ := a.MarshalBinary()
	//B,_ := b.MarshalBinary()
	//C,_ := c.MarshalBinary()

	return CompleteProof{Proof, B1, B2, A, B, C}
}

func SwitchToProbProof(suite abstract.Suite, s abstract.Scalar, rj abstract.Scalar, newK abstract.Point, C1 abstract.Point, C2 abstract.Point) CompleteProof {
	return SwitchKeyProof( suite, s, rj, suite.Point().Base(), newK, C1, C2)
	/*pred := CreatePredicate()

	B1 := suite.Point().Neg(suite.Point().Base()) // B1 = -rjB

	B2 := newK

	A := suite.Point().Mul(B1, s) // a = -rjBk = -rjK

	B := suite.Point().Mul(B2, rj) // b = sB2

	C := suite.Point().Sub(C2, C1) // c = Ci - C(i-1)

	sval := map[string]abstract.Scalar{"k": k, "s": s}
	pval := map[string]abstract.Point{"B1": B1, "B2": B2, "a": A, "b": B, "c": C}
	prover := pred.Prover(suite, sval, pval, nil) // computes: commitment, challenge, response

	rand := suite.Cipher(abstract.RandomKey)

	Proof, err := proof.HashProve(suite, "TEST", rand, prover)
	_ = Proof
	if err != nil {
		log.Errorf("---------Prover:", err.Error())
	} else {

	}

	//if we want binaries
	//b1,_ := B1.MarshalBinary()
	//b2,_ := B2.MarshalBinary()
	//A,_ := a.MarshalBinary()
	//B,_ := b.MarshalBinary()
	//C,_ := c.MarshalBinary()

	return CompleteProof{Proof, B1, B2, A, B, C}*/
}

//check proof for scheme & key switching on ciphervector
func VectSwitchCheckProof(cps []CompleteProof) bool {
	for _, v := range cps {
		if !SwitchCheckProof(v) {
			return false
		}
	}
	return true
}

//check proof for scheme & key switching
func SwitchCheckProof(cp CompleteProof) bool {
	pred := CreatePredicate()
	pval := map[string]abstract.Point{"B1": cp.B1, "B2": cp.B2, "a": cp.A, "b": cp.B, "c": cp.C}
	verifier := pred.Verifier(suite, pval)
	if err := proof.HashVerify(suite, "TEST", verifier, cp.Proof); err != nil {
		log.Errorf("---------Verifier:", err.Error())
		return false
	} else {
		//log.LLvl1("Proof verified")
	}

	return true
}

func SwitchCheckMapProofs(m map[TempID][]CompleteProof) bool {
	for _,v := range m {
		if !VectSwitchCheckProof(v) {
			log.Errorf("ATTENTION, false proof detected")
			return false
		}
	}
	return true
}

func CreatePredicate() (pred proof.Predicate) {
	// For ZKP
	log1 := proof.Rep("a", "k", "B1")
	log2 := proof.Rep("b", "s", "B2")

	// Two-secret representation: prove c = kiB1 + siB2
	rep := proof.Rep("c", "k", "B1", "s", "B2")

	// and-predicate: prove that a = kiB1, b = siB2 and c = a + b
	and := proof.And(log1, log2)
	and = proof.And(and, rep)
	pred = proof.And(and)
	return
}
