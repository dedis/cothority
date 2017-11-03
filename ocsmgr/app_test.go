package main

import (
	"testing"

	"gopkg.in/dedis/crypto.v0/config"
	"gopkg.in/dedis/onet.v1/crypto"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestPriv(t *testing.T) {
	for i := 0; i < 10; i++ {
		kp := config.NewKeyPair(network.Suite)
		priv := kp.Secret
		privStr := priv.String()
		log.Lvlf2("privStr: %x", privStr)
		privStr2, err := crypto.ScalarToStringHex(network.Suite, priv)
		log.ErrFatal(err)
		log.Lvl2("scatostr:", privStr2)
		privDec, err := crypto.StringHexToScalar(network.Suite, privStr)
		if err != nil {
			log.Error("Scalar.String() is missing leading zeroes")
		}
		log.Lvl2("privDec:", privDec)
		neg := network.Suite.Scalar().Neg(priv)
		log.Lvl2("Sum:", neg.Add(neg, priv))
		log.Lvl2("Sum:", neg.Add(priv, neg))
		log.Lvl2()
	}
}

func TestEndian(t *testing.T) {
	//privStr := "5046ADC1DBA838867B2BBBFDD0C3423E58B57970B5267A90F57960924A87F156"
	privStr := "77d8aa14f60a5e4c6d82769da56f536ccae145bfb55f2f59dcea67a336b45c7b"
	priv, err := crypto.StringHexToScalar(network.Suite, privStr)
	log.ErrFatal(err)
	log.Lvl2("private:", priv)
	str, err := crypto.ScalarToStringHex(network.Suite, priv)
	log.ErrFatal(err)
	log.Lvl2("private:", str)
	pub := network.Suite.Point().Mul(nil, priv)
	log.Lvl2("public:", pub)

	priv.Add(priv, network.Suite.Scalar().One())
	log.Lvl2("private+1:", priv)
	pub.Add(pub, network.Suite.Point().Base())
	log.Lvl2("public+1:", pub)

	priv.Add(priv, network.Suite.Scalar().One())
	log.Lvl2("private+2:", priv)
	pub = network.Suite.Point().Mul(nil, priv)
	log.Lvl2("public+2:", pub)
}

func TestNegate(t *testing.T) {
	privStr := "762755eb09f5a1b3927d89625a90ac93351eba404aa0d0a62315985cc94ba304"
	priv, err := crypto.StringHexToScalar(network.Suite, privStr)
	log.ErrFatal(err)

	log.Lvl2("Private:", priv)
	neg := network.Suite.Scalar().Neg(priv)
	log.Lvl2("Negative:", neg)
	priv.Add(priv, network.Suite.Scalar().One())
	sum := network.Suite.Scalar().Add(neg, priv)
	log.Lvl2("Sum:", sum)
}
