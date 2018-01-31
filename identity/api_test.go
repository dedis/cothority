package identity

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/cosi/protocol"
	"github.com/dedis/cothority/pop/service"
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/sign/anon"
	"github.com/dedis/kyber/sign/schnorr"
	"github.com/dedis/kyber/suites"
	"github.com/dedis/kyber/util/key"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var tSuite = cothority.Suite

func NewTestIdentity(cothority *onet.Roster, majority int, owner string, local *onet.LocalTest, kp *key.Pair) *Identity {
	id := NewIdentity(cothority, majority, owner, kp)
	id.Client = local.NewClient(ServiceName)
	return id
}

func TestIdentity_PinRequest(t *testing.T) {
	local := onet.NewTCPTest(tSuite)
	defer local.CloseAll()
	servers := local.GenServers(1)
	srvc := local.GetServices(servers, identityService)[0].(*Service)
	require.Equal(t, 0, len(srvc.auth.pins))
	pub := tSuite.Point().Pick(tSuite.XOF([]byte("test")))
	_, err := srvc.PinRequest(&PinRequest{"", pub})
	require.NotNil(t, err)
	require.NotEqual(t, 0, len(srvc.auth.pins))
	pin := ""
	for t := range srvc.auth.pins {
		pin = t
	}
	_, err = srvc.PinRequest(&PinRequest{pin, pub})
	log.Error(err)
	require.Equal(t, pub, srvc.auth.adminKeys[0])
}

func suiteSkip(t *testing.T) {
	// Some of these tests require Ed25519, so skip if we are currently
	// running with another suite.
	if tSuite != suites.MustFind("Ed25519") {
		t.Skip("current suite is not compatible with this test, skipping it")
		return
	}
}

func TestIdentity_StoreKeys(t *testing.T) {
	suiteSkip(t)
	local := onet.NewTCPTest(tSuite)
	defer local.CloseAll()
	servers := local.GenServers(1)
	el := local.GenRosterFromHost(servers...)
	srvc := local.GetServices(servers, identityService)[0].(*Service)
	keypairAdmin := key.NewKeyPair(tSuite)
	keypairUser := key.NewKeyPair(tSuite)

	popDesc := &service.PopDesc{}
	popDesc.Name = "test"
	popDesc.DateTime = "test"
	popDesc.Location = "test"
	popDesc.Roster = el
	popDesc.Parties = make([]*service.ShortDesc, 0)

	final := &service.FinalStatement{}
	final.Desc = popDesc
	final.Attendees = make([]kyber.Point, 1)
	final.Attendees[0] = keypairUser.Public
	hash, err := final.Hash()
	log.ErrFatal(err)

	//Sign Final
	tree := el.GenerateNaryTreeWithRoot(2, srvc.ServerIdentity())
	node, err := srvc.CreateProtocol(cosi.Name, tree)
	log.ErrFatal(err)
	signature := make(chan []byte)
	c := node.(*cosi.CoSi)
	c.RegisterSignatureHook(func(sig []byte) {
		log.Lvl3("sig", len(sig))
		signature <- sig[0 : len(sig)-1]
	})
	c.Message = hash
	go node.Start()

	final.Signature = <-signature

	srvc.auth.adminKeys = append(srvc.auth.adminKeys, keypairAdmin.Public)

	sig, err := schnorr.Sign(tSuite, keypairAdmin.Private, hash)
	log.ErrFatal(err)
	_, err = srvc.StoreKeys(&StoreKeys{PoPAuth, final, nil, sig})
	require.Nil(t, err)
	require.Equal(t, 1, len(srvc.auth.sets))
}

func TestIdentity_StoreKeys2(t *testing.T) {
	local := onet.NewTCPTest(tSuite)
	defer local.CloseAll()
	servers := local.GenServers(1)
	srvc := local.GetServices(servers, identityService)[0].(*Service)
	keypairAdmin := key.NewKeyPair(tSuite)

	N := 5
	pubs := make([]kyber.Point, N)
	h := tSuite.Hash()
	for i := 0; i < N; i++ {
		kp := key.NewKeyPair(tSuite)
		pubs[i] = kp.Public
		b, err := kp.Public.MarshalBinary()
		log.ErrFatal(err)
		_, err = h.Write(b)
		log.ErrFatal(err)
	}
	hash := h.Sum(nil)

	srvc.auth.adminKeys = append(srvc.auth.adminKeys, keypairAdmin.Public)
	sig, err := schnorr.Sign(tSuite, keypairAdmin.Private, hash)
	log.ErrFatal(err)
	_, err = srvc.StoreKeys(&StoreKeys{PublicAuth, nil, pubs, sig})
	require.Nil(t, err)
	require.Equal(t, N, len(srvc.auth.keys))
}

func TestIdentity_DataNewCheck(t *testing.T) {
	l := onet.NewTCPTest(tSuite)
	hosts, el, _ := l.GenTree(5, true)
	services := l.GetServices(hosts, identityService)
	defer l.CloseAll()

	c1 := createIdentity(l, services, el, "one")

	data2 := c1.Data.Copy()
	kp2 := key.NewKeyPair(tSuite)
	data2.Device["two"] = &Device{kp2.Public}
	data2.Storage["two"] = "public2"
	log.ErrFatal(c1.ProposeSend(data2))

	log.ErrFatal(c1.ProposeUpdate())
	al := c1.Proposed
	assert.NotNil(t, al)

	o2, ok := al.Device["two"]
	assert.True(t, ok)
	assert.True(t, kp2.Public.Equal(o2.Point))
	pub2, ok := al.Storage["two"]
	assert.True(t, ok)
	assert.Equal(t, "public2", pub2)
}

func TestIdentity_AttachToIdentity(t *testing.T) {
	l := onet.NewTCPTest(tSuite)
	hosts, el, _ := l.GenTree(5, true)
	services := l.GetServices(hosts, identityService)
	for _, s := range services {
		s.(*Service).clearIdentities()
	}
	defer l.CloseAll()

	c1 := createIdentity(l, services, el, "one")

	c2 := NewTestIdentity(el, 50, "two", l, nil)
	log.ErrFatal(c2.AttachToIdentity(c1.ID))
	for _, s := range services {
		is := s.(*Service)
		is.identitiesMutex.Lock()
		if len(is.Identities) != 1 {
			t.Fatal("The new data hasn't been proposed in all services")
		}
		is.identitiesMutex.Unlock()
	}
}

func TestIdentity_DataUpdate(t *testing.T) {
	l := onet.NewTCPTest(tSuite)
	hosts, el, _ := l.GenTree(5, true)
	services := l.GetServices(hosts, identityService)
	defer l.CloseAll()

	c1 := createIdentity(l, services, el, "one")

	c2 := NewTestIdentity(el, 50, "two", l, nil)
	c2.ID = c1.ID
	log.ErrFatal(c2.DataUpdate())

	assert.NotNil(t, c2.Data)
	o1 := c2.Data.Device[c1.DeviceName]
	if !o1.Point.Equal(c1.Public) {
		t.Fatal("Owner is not c1")
	}
}

func TestIdentity_Authenticate(t *testing.T) {
	l := onet.NewTCPTest(tSuite)
	hosts, _, _ := l.GenTree(1, true)
	services := l.GetServices(hosts, identityService)
	s := services[0].(*Service)
	defer l.CloseAll()
	au := &Authenticate{[]byte{}, []byte{}}
	s.Authenticate(au)
	require.Equal(t, 1, len(s.auth.nonces))
}

func TestIdentity_CreateIdentity(t *testing.T) {
	l := onet.NewTCPTest(tSuite)
	hosts, el, _ := l.GenTree(3, true)
	services := l.GetServices(hosts, identityService)
	defer l.CloseAll()
	c := createIdentity(l, services, el, "one")
	// Check we're in the data
	assert.NotNil(t, c.Data)
}

func TestIdentity_DataNewPropose(t *testing.T) {
	l := onet.NewTCPTest(tSuite)
	hosts, el, _ := l.GenTree(2, true)
	services := l.GetServices(hosts, identityService)
	defer l.CloseAll()

	c1 := createIdentity(l, services, el, "onet")

	data2 := c1.Data.Copy()
	kp2 := key.NewKeyPair(tSuite)
	data2.Device["two"] = &Device{kp2.Public}
	log.ErrFatal(c1.ProposeSend(data2))

	for _, s := range services {
		is := s.(*Service)
		id1 := is.getIdentityStorage(c1.ID)
		id1.Lock()
		if id1 == nil {
			t.Fatal("Didn't find")
		}
		assert.NotNil(t, id1.Proposed)
		if len(id1.Proposed.Device) != 2 {
			t.Fatal("The proposed data should have 2 entries now")
		}
		id1.Unlock()
	}
}

func TestIdentity_ProposeVote(t *testing.T) {
	l := onet.NewTCPTest(tSuite)
	hosts, el, _ := l.GenTree(5, true)
	services := l.GetServices(hosts, identityService)
	defer l.CloseAll()
	for _, s := range services {
		log.Lvl3(s.(*Service).Identities)
	}

	c1 := createIdentity(l, services, el, "one1")
	data2 := c1.Data.Copy()
	kp2 := key.NewKeyPair(tSuite)
	data2.Device["two2"] = &Device{kp2.Public}
	data2.Storage["two2"] = "public2"
	log.ErrFatal(c1.ProposeSend(data2))
	log.ErrFatal(c1.ProposeUpdate())
	log.ErrFatal(c1.ProposeVote(true))

	if len(c1.Data.Device) != 2 {
		t.Fatal("Should have two owners now")
	}
}

func TestIdentity_SaveToStream(t *testing.T) {
	l := onet.NewTCPTest(tSuite)
	_, el, _ := l.GenTree(5, true)
	defer l.CloseAll()
	id := NewIdentity(el, 50, "one1", nil)
	tmpfile, err := ioutil.TempFile("", "example")
	log.ErrFatal(err)
	defer os.Remove(tmpfile.Name())
	log.ErrFatal(id.SaveToStream(tmpfile))
	tmpfile.Close()
	tmpfile, err = os.Open(tmpfile.Name())
	log.ErrFatal(err)
	id2, err := NewIdentityFromStream(tmpfile)
	assert.NotNil(t, id2)
	log.ErrFatal(err)
	tmpfile.Close()

	if id.Data.Threshold != id2.Data.Threshold {
		t.Fatal("Loaded threshold is not the same")
	}
	p, p2 := id.Data.Device["one1"].Point, id2.Data.Device["one1"].Point
	if !p.Equal(p2) {
		t.Fatal("Public keys are not the same")
	}
	if id.Data.Storage["one1"] != id2.Data.Storage["one1"] {
		t.Fatal("Owners are not the same", id.Data.Storage, id2.Data.Storage)
	}
}

func TestCrashAfterRevocation(t *testing.T) {
	l := onet.NewTCPTest(tSuite)
	hosts, el, _ := l.GenTree(5, true)
	services := l.GetServices(hosts, identityService)
	defer l.CloseAll()
	keypair := key.NewKeyPair(tSuite)
	kp2 := key.NewKeyPair(tSuite)
	set := anon.Set([]kyber.Point{keypair.Public, kp2.Public})
	for _, srvc := range services {
		s := srvc.(*Service)
		log.Lvl3(s.Identities)
		s.auth.sets = append(s.auth.sets, set)
	}

	c1 := NewIdentity(el, 2, "one", keypair)
	c2 := NewIdentity(el, 2, "two", nil)
	c3 := NewIdentity(el, 2, "three", nil)
	defer c1.Client.Close()
	defer c2.Client.Close()
	defer c3.Client.Close()
	log.ErrFatal(c1.CreateIdentity(PoPAuth, set))
	log.ErrFatal(c2.AttachToIdentity(c1.ID))
	proposeUpVote(c1)
	log.ErrFatal(c3.AttachToIdentity(c1.ID))
	proposeUpVote(c1)
	proposeUpVote(c2)
	log.ErrFatal(c1.DataUpdate())
	log.Lvl2(c1.Data)

	data := c1.GetProposed()
	delete(data.Device, "three")
	log.Lvl2(data)
	log.ErrFatal(c1.ProposeSend(data))
	proposeUpVote(c1)
	proposeUpVote(c2)
	log.ErrFatal(c1.DataUpdate())
	log.Lvl2(c1.Data)

	log.Lvl1("C3 trying to send anyway")
	data = c3.GetProposed()
	c3.ProposeSend(data)
	if c3.ProposeVote(true) == nil {
		t.Fatal("Should not be able to vote")
	}
	log.ErrFatal(c1.ProposeUpdate())
}

func TestVerificationFunction(t *testing.T) {
	l := onet.NewTCPTest(tSuite)
	hosts, el, _ := l.GenTree(2, true)
	services := l.GetServices(hosts, identityService)
	s0 := services[0].(*Service)
	defer l.CloseAll()
	for _, s := range services {
		log.Lvl3(s.(*Service).Identities)
	}

	c1 := createIdentity(l, services, el, "one1")

	// Hack: create own data-structure with twice our signature
	// and send it directly to the skipblock. Without a proper
	// verification-function, this would pass.
	data2 := c1.Data.Copy()
	kp2 := key.NewKeyPair(tSuite)
	data2.Device["two2"] = &Device{kp2.Public}
	data2.Storage["two2"] = "public2"
	hash, err := data2.Hash(tSuite)
	log.ErrFatal(err)
	sig, err := schnorr.Sign(tSuite, kp2.Private, hash)
	log.ErrFatal(err)
	data2.Votes["one1"] = sig
	data2.Votes["two2"] = sig
	id := s0.getIdentityStorage(c1.ID)
	require.NotNil(t, id, "Didn't find identity")
	_, err = s0.skipchain.StoreSkipBlock(id.SCData, nil, data2)
	require.NotNil(t, err, "Skipchain accepted our fake block!")

	// Gibberish signature
	sig, err = schnorr.Sign(tSuite, c1.Private, hash)
	log.ErrFatal(err)
	// Change one bit in the signature
	sig[len(sig)-1] ^= 1
	data2.Votes["one1"] = sig
	_, err = s0.skipchain.StoreSkipBlock(id.SCData, nil, data2)
	require.NotNil(t, err, "Skipchain accepted our fake signature!")

	// Unhack: verify that the correct way of doing it works, even if
	// we bypass the identity.
	sig, err = schnorr.Sign(tSuite, c1.Private, hash)
	log.ErrFatal(err)
	data2.Votes["one1"] = sig
	_, err = s0.skipchain.StoreSkipBlock(id.SCData, nil, data2)
	log.ErrFatal(err)
	log.ErrFatal(c1.DataUpdate())

	if len(c1.Data.Device) != 2 {
		t.Fatal("Should have two owners now")
	}
}

func proposeUpVote(i *Identity) {
	log.ErrFatal(i.ProposeUpdate())
	log.ErrFatal(i.ProposeVote(true))
}

func createIdentity(l *onet.LocalTest, services []onet.Service, el *onet.Roster, name string) *Identity {
	keypair := key.NewKeyPair(tSuite)
	kp2 := key.NewKeyPair(tSuite)
	set := anon.Set([]kyber.Point{keypair.Public, kp2.Public})
	for _, srvc := range services {
		s := srvc.(*Service)
		s.auth.sets = append(s.auth.sets, set)
	}

	c := NewTestIdentity(el, 50, name, l, keypair)
	log.Error("popauth", PoPAuth)
	log.Error("set", set)
	log.ErrFatal(c.CreateIdentity(PoPAuth, set))
	return c
}
