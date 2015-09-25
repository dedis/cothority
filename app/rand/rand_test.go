package main

import (
	//"log"
	"fmt"
	"github.com/dedis/crypto/edwards/ed25519"
	"github.com/dedis/crypto/random"
	"github.com/dedis/crypto/sig"
	"testing"
)

func TestRand(t *testing.T) {

	net := newChanNet()
	suite := ed25519.NewAES128SHA256Ed25519(false)
	rand := random.Stream

	nservers := 3

	// Signing keypairs for client and servers
	clisec := sig.SchnorrScheme{Suite: suite}.SecretKey()
	clisec.Pick(rand)
	clipub := clisec.PublicKey()

	srvsec := make([]sig.SchnorrSecretKey, nservers)
	srvpub := make([]sig.SchnorrPublicKey, nservers)
	for i := range srvsec {
		srvsec[i].Suite = suite
		srvsec[i].Pick(rand)
		srvpub[i] = srvsec[i].SchnorrPublicKey
		//log.Printf("server %d public key %v", i, srvpub[i])
	}

	srv := make([]Server, nservers)
	srvname := make([]string, nservers)
	for i := 0; i < nservers; i++ {
		//pri := suite.Secret().Pick(rand)
		srvname[i] = fmt.Sprintf("server%d", i)
		host := newChanHost(net, srvname[i], srv[i].serve)
		srv[i].init(host, suite, clipub, srvpub, srvsec[i], i)
	}

	cli := Client{}
	//cpri := suite.Secret().Pick(rand)
	chost := newChanHost(net, "client", nil)
	cli.init(chost, suite, rand, clisec, srvname, srvpub)

	if err := cli.run(); err != nil {
		panic(err)
	}
}
