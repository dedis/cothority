package main

import (
	"testing"

	"github.com/dedis/cothority"
	"github.com/dedis/kyber/util/encoding"
	"github.com/dedis/kyber/util/key"
	"github.com/dedis/onet/log"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestPriv(t *testing.T) {
	for i := 0; i < 10; i++ {
		kp := key.NewKeyPair(cothority.Suite)
		priv := kp.Private
		b, _ := priv.MarshalBinary()
		if len(b) != 32 {
			t.Fatal("wrong marshal len", len(b))
		}
		privStr := priv.String()
		if len(privStr) != 64 {
			t.Fatal("privStr is too short: ", len(privStr))
		}
		log.Lvlf2("privStr: %x", privStr)

		privStr2, err := encoding.ScalarToStringHex(cothority.Suite, priv)
		log.ErrFatal(err)

		log.Lvl2("scatostr:", privStr2, len(privStr2))
		privDec, err := encoding.StringHexToScalar(cothority.Suite, privStr)
		if err != nil {
			t.Fatal("StringHexToScalar failed: ", err)
		}
		log.Lvl2("privDec:", privDec)
		neg := cothority.Suite.Scalar().Neg(priv)
		log.Lvl2("Sum:", neg.Add(neg, priv))
		log.Lvl2("Sum:", neg.Add(priv, neg))
		log.Lvl2()
	}
}

func TestEndian(t *testing.T) {
	privStr := "77d8aa14f60a5e4c6d82769da56f536ccae145bfb55f2f59dcea67a336b45c7b"
	priv, err := encoding.StringHexToScalar(cothority.Suite, privStr)
	log.ErrFatal(err)
	log.Lvl2("private:", priv)
	str, err := encoding.ScalarToStringHex(cothority.Suite, priv)
	log.ErrFatal(err)
	log.Lvl2("private:", str)
	pub := cothority.Suite.Point().Mul(priv, nil)
	log.Lvl2("public:", pub)

	priv.Add(priv, cothority.Suite.Scalar().One())
	log.Lvl2("private+1:", priv)
	pub.Add(pub, cothority.Suite.Point().Base())
	log.Lvl2("public+1:", pub)

	priv.Add(priv, cothority.Suite.Scalar().One())
	log.Lvl2("private+2:", priv)
	pub = cothority.Suite.Point().Mul(priv, nil)
	log.Lvl2("public+2:", pub)
}

func TestNegate(t *testing.T) {
	privStr := "762755eb09f5a1b3927d89625a90ac93351eba404aa0d0a62315985cc94ba304"
	priv, err := encoding.StringHexToScalar(cothority.Suite, privStr)
	log.ErrFatal(err)

	log.Lvl2("Private:", priv)
	neg := cothority.Suite.Scalar().Neg(priv)
	log.Lvl2("Negative:", neg)
	priv.Add(priv, cothority.Suite.Scalar().One())
	sum := cothority.Suite.Scalar().Add(neg, priv)
	log.Lvl2("Sum:", sum)
}
