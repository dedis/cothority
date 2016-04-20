package sshks_test

import (
	"strconv"
	"testing"

	"github.com/dedis/cothority/lib/cosi"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/services/sshks"
)

func TestServerCreation(t *testing.T) {
	srvApps := createServerKSs(2)
	for i, s := range srvApps {
		if s.This.Entity.Addresses[0] != "localhost:"+strconv.Itoa(2000+i) {
			t.Fatal("Couldn't verify server", i, s)
		}
	}
}

func TestServerAdd(t *testing.T) {
	srvApps := createServerKSs(2)
	srvApps[0].AddServer(srvApps[1].This)
	id1 := srvApps[1].This.Id()
	_, ok := srvApps[0].Config.Servers[id1]
	if !ok {
		t.Fatal("Didn't find server 1 in server 0")
	}
	srvApps[0].DelServer(srvApps[1].This)
	_, ok = srvApps[0].Config.Servers[id1]
	if ok {
		t.Fatal("Shouldn't find server 1 in server 0")
	}
}

func TestServerStart(t *testing.T) {
	srvApps := createServerKSs(2)
	srvApps[0].AddServer(srvApps[1].This)
	err := srvApps[0].Start()
	dbg.TestFatal(t, err, "Couldn't start server:")
	defer srvApps[0].Stop()
	err = srvApps[1].Start()
	dbg.TestFatal(t, err, "Couldn't start server:")
	defer srvApps[1].Stop()

	err = srvApps[0].Check()
	dbg.TestFatal(t, err, "Couldn't check servers:")
}

func TestConfigEntityList(t *testing.T) {
	srvApps := createServerKSs(2)
	c := srvApps[0]
	c.AddServer(srvApps[1].This)
	el := c.Config.EntityList(c.This.Entity)
	if len(el.List) == 0 {
		t.Fatal("List shouldn't be of length 0")
	}
	if el.List[0] != c.This.Entity {
		t.Fatal("First element should be server 0")
	}
	el = c.Config.EntityList(srvApps[1].This.Entity)
	if el.List[0] != srvApps[1].This.Entity {
		t.Fatal("First element should be server 1")
	}
}

func TestServerFunc(t *testing.T) {
	client1, servers := newTest(1)
	defer closeServers(t, servers)
	e1 := client1.This.Entity
	co1 := cosi.NewCosi(network.Suite, client1.Private)
	srv := servers[0]
	conf := sshks.NewConfig(1)
	conf.Clients[client1.This.SSHpub] = client1.This
	conf.Servers[srv.This.Entity.Addresses[0]] = srv.This

	// Client1 sends its commit
	comm1 := co1.CreateCommitment()
	srv.FuncSendFirstCommit(sendMsg(e1, sshks.SendFirstCommit{comm1}))

	// New config is sent and challenge is returned
	chMsg := srv.FuncSendNewConfig(sendMsg(e1, sshks.SendNewConfig{conf}))
	challenge := chMsg.(*sshks.SendNewConfigRet).Challenge
	co1.Challenge(&cosi.Challenge{Challenge: challenge})
	resp1, err := co1.CreateResponse()
	dbg.ErrFatal(err)
	co2 := cosi.NewCosi(network.Suite, client1.Private)
	respMsg := srv.FuncResponse(sendMsg(e1, sshks.Response{resp1, co2.CreateCommitment()}))
	resp := respMsg.(*sshks.ResponseRet)
	if resp.Config == nil {
		t.Fatal("The response should be complete now")
	}
	confMsg := srv.FuncGetConfig(sendMsg(e1, sshks.GetConfig{}))
	conf2 := confMsg.(*sshks.GetConfigRet).Config
	dbg.ErrFatal(conf2.VerifySignature())
}

func sendMsg(e *network.Entity, msg network.ProtocolMessage) *network.Message {
	return &network.Message{
		Msg:    msg,
		Entity: e,
	}
}

func TestServerSign(t *testing.T) {
	client1, servers := newTest(1)
	defer closeServers(t, servers)
	e1 := client1.This.Entity
	co1 := cosi.NewCosi(network.Suite, client1.Private)
	srv := servers[0]
	conf := sshks.NewConfig(1)
	conf.Clients[client1.This.SSHpub] = client1.This

	// Client1 sends its commit
	comm1 := co1.CreateCommitment()
	srv.NextConfig.AddCommit(e1, comm1)
	// New config is sent and challenge is returned
	challenge, err := srv.NextConfig.NewConfig(srv, conf)
	dbg.ErrFatal(err)
	co1.Challenge(&cosi.Challenge{Challenge: challenge})
	resp1, err := co1.CreateResponse()
	dbg.ErrFatal(err)
	ok := srv.NextConfig.AddResponse(e1, resp1)
	if !ok {
		t.Fatal("The response should be complete now")
	}
	conf = srv.NextConfig.GetConfig()
	dbg.ErrFatal(conf.VerifySignature())
	srv.Config = conf

	// Add a second client
	client2 := newClient(1)
	e2 := client2.This.Entity
	newConfig := sshks.NewConfig(2)
	newConfig.Clients[client1.This.SSHpub] = client1.This
	newConfig.Clients[client2.This.SSHpub] = client2.This
	comm1 = co1.CreateCommitment()
	srv.NextConfig.AddCommit(e1, comm1)
	challenge, err = srv.NextConfig.NewConfig(srv, newConfig)
	dbg.ErrFatal(err)
	co1.Challenge(&cosi.Challenge{Challenge: challenge})
	resp1, err = co1.CreateResponse()
	dbg.ErrFatal(err)
	ok = srv.NextConfig.AddResponse(e1, resp1)
	if !ok {
		t.Fatal("The response should be complete now")
	}
	conf = srv.NextConfig.GetConfig()
	dbg.ErrFatal(conf.VerifySignature())
	srv.Config = conf

	// Now have both clients sign a new config
	co2 := cosi.NewCosi(network.Suite, client2.Private)
	comm1 = co1.CreateCommitment()
	comm2 := co2.CreateCommitment()
	conf.Version++
	srv.NextConfig.AddCommit(e1, comm1)
	srv.NextConfig.AddCommit(e2, comm2)
	challenge, err = srv.NextConfig.NewConfig(srv, conf)
	dbg.ErrFatal(err)
	co1.Challenge(&cosi.Challenge{challenge})
	co2.Challenge(&cosi.Challenge{challenge})
	resp1, err = co1.CreateResponse()
	dbg.ErrFatal(err)
	resp2, err := co2.CreateResponse()
	dbg.ErrFatal(err)
	ok = srv.NextConfig.AddResponse(e1, resp1)
	if ok {
		t.Fatal("One signature should not be enough")
	}
	ok = srv.NextConfig.AddResponse(e2, resp2)
	if !ok {
		t.Fatal("Two signatures should be enough")
	}
	conf = srv.NextConfig.GetConfig()
	if conf.Version != 4 {
		t.Fatal("Version should now be 4")
	}
	dbg.ErrFatal(conf.VerifySignature())
}
