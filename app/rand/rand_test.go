package main

import (
	//"log"
	"fmt"
	"github.com/dedis/crypto/edwards/ed25519"
	"github.com/dedis/crypto/random"
	"github.com/dedis/crypto/sig"
	"testing"
	"time"
)

func TestRand(t *testing.T) {

	net := newChanNet()
	suite := ed25519.NewAES128SHA256Ed25519(false)
	rand := random.Stream

	nservers := 10
	ntrustees := 5

	// Signing keypairs for client and servers
	clisec := sig.SchnorrScheme{Suite: suite}.SecretKey()
	clisec.Pick(rand)
	clipub := clisec.PublicKey()
	clipubb, _ := clipub.MarshalBinary()

	srvsec := make([]sig.SchnorrSecretKey, nservers)
	srvpub := make([]sig.SchnorrPublicKey, nservers)
	srvhost := make([]HostInfo, nservers)
	srv := make([]Server, nservers)
	srvname := make([]string, nservers)
	for i := 0; i < nservers; i++ {
		srvsec[i].Suite = suite
		srvsec[i].Pick(rand)
		srvpub[i] = srvsec[i].SchnorrPublicKey
		//log.Printf("server %d public key %v", i, srvpub[i])
		srvpubb, _ := srvpub[i].MarshalBinary()
		srvhost[i] = HostInfo{srvpubb, nil}
		//pri := suite.Secret().Pick(rand)
		srvname[i] = fmt.Sprintf("server%d", i)
		host := newChanHost(net, srvname[i], srv[i].serve)
		srv[i].init(host, suite, clipub, srvpub, srvsec[i], i)
	}

	clihost := HostInfo{clipubb, nil}
	session := &Session{clihost, "test", time.Now()}

	group := &Group{
		Servers: srvhost,
		F:       nservers / 3,
		L:       nservers - (nservers / 3),
		K:       ntrustees,
		T:       (ntrustees + 1) / 2}

	cli := Client{}
	//cpri := suite.Secret().Pick(rand)
	chost := newChanHost(net, "client", nil)
	cli.init(chost, suite, rand, session, group, clisec, srvname, srvpub)

	if err := cli.run(); err != nil {
		panic(err)
	}
}
