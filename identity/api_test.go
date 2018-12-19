package identity

import (
	"errors"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/blscosi/protocol"
	"github.com/dedis/cothority/pop/service"
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/pairing"
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
	require.Equal(t, 0, len(srvc.Storage.Auth.Pins))
	pub := tSuite.Point().Pick(tSuite.XOF([]byte("test")))
	_, err := srvc.PinRequest(&PinRequest{"", pub})
	require.NotNil(t, err)
	require.NotEqual(t, 0, len(srvc.Storage.Auth.Pins))
	pin := ""
	for t := range srvc.Storage.Auth.Pins {
		pin = t
	}
	_, err = srvc.PinRequest(&PinRequest{pin, pub})
	log.Error(err)
	require.Equal(t, pub, srvc.Storage.Auth.AdminKeys[0])
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
	roster := local.GenRosterFromHost(servers...)
	srvcIdentity := local.GetServices(servers, identityService)[0].(*Service)
	srvcPop := servers[0].Service(service.Name).(*service.Service)
	keypairAdmin := key.NewKeyPair(tSuite)
	keypairUser := key.NewKeyPair(tSuite)

	popDesc := &service.PopDesc{}
	popDesc.Name = "test"
	popDesc.DateTime = "test"
	popDesc.Location = "test"
	popDesc.Roster = roster
	popDesc.Parties = make([]*service.ShortDesc, 0)

	final := &service.FinalStatement{}
	final.Desc = popDesc
	final.Attendees = make([]kyber.Point, 1)
	final.Attendees[0] = keypairUser.Public
	hash, err := final.Hash()
	log.ErrFatal(err)

	// Sign Final
	protoName := "TestIdentity_StoreKeys"
	// Protocol is created using the pop service so that it
	// will use the correct key pair (e.i. the pop one)
	err = registerCosiProtocols(srvcPop.Context, protoName)
	require.Nil(t, err)

	rooted := roster.NewRosterWithRoot(srvcPop.ServerIdentity())
	require.NotNil(t, rooted)
	tree := rooted.GenerateNaryTree(len(roster.List))
	require.NotNil(t, tree)
	node, err := srvcPop.CreateProtocol(protoName, tree)
	require.Nil(t, err)

	c := node.(*protocol.BlsCosi)
	c.Msg = hash
	c.CreateProtocol = local.CreateProtocol
	c.Timeout = time.Second * 5

	err = node.Start()
	require.Nil(t, err)

	final.Signature = <-c.FinalSignature
	require.NotNil(t, final.Signature)
	srvcIdentity.Storage.Auth.AdminKeys = append(srvcIdentity.Storage.Auth.AdminKeys, keypairAdmin.Public)

	sig, err := schnorr.Sign(tSuite, keypairAdmin.Private, hash)
	require.Nil(t, err)
	_, err = srvcIdentity.StoreKeys(&StoreKeys{PoPAuth, final, nil, sig})
	require.Nil(t, err)
	require.Equal(t, 1, len(srvcIdentity.Storage.Auth.Sets))
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

	srvc.Storage.Auth.AdminKeys = append(srvc.Storage.Auth.AdminKeys, keypairAdmin.Public)
	sig, err := schnorr.Sign(tSuite, keypairAdmin.Private, hash)
	log.ErrFatal(err)
	_, err = srvc.StoreKeys(&StoreKeys{PublicAuth, nil, pubs, sig})
	require.Nil(t, err)
	require.Equal(t, N, len(srvc.Storage.Auth.Keys))
}

func TestIdentity_DataNewCheck(t *testing.T) {
	l := onet.NewTCPTest(tSuite)
	hosts, roster, _ := l.GenTree(5, true)
	services := l.GetServices(hosts, identityService)
	defer l.CloseAll()

	c1 := createIdentity(l, services, roster, "one")
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
	hosts, roster, _ := l.GenTree(5, true)
	services := l.GetServices(hosts, identityService)
	for _, s := range services {
		s.(*Service).clearIdentities()
	}
	defer l.CloseAll()

	c1 := createIdentity(l, services, roster, "one")

	c2 := NewTestIdentity(roster, 50, "two", l, nil)
	log.ErrFatal(c2.AttachToIdentity(c1.ID))
	for _, s := range services {
		is := s.(*Service)
		is.storageMutex.Lock()
		if len(is.Storage.Identities) != 1 {
			t.Fatal("The new data hasn't been proposed in all services")
		}
		is.storageMutex.Unlock()
	}
}

func TestIdentity_DataUpdate(t *testing.T) {
	l := onet.NewTCPTest(tSuite)
	hosts, roster, _ := l.GenTree(5, true)
	services := l.GetServices(hosts, identityService)
	defer l.CloseAll()

	c1 := createIdentity(l, services, roster, "one")

	c2 := NewTestIdentity(roster, 50, "two", l, nil)
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
	require.Equal(t, 1, len(s.Storage.Auth.Nonces))
}

func TestIdentity_CreateIdentity(t *testing.T) {
	l := onet.NewTCPTest(tSuite)
	hosts, roster, _ := l.GenTree(3, true)
	services := l.GetServices(hosts, identityService)
	defer l.CloseAll()
	c := createIdentity(l, services, roster, "one")
	// Check we're in the data
	assert.NotNil(t, c.Data)
}

func TestIdentity_DataNewPropose(t *testing.T) {
	l := onet.NewTCPTest(tSuite)
	hosts, roster, _ := l.GenTree(2, true)
	services := l.GetServices(hosts, identityService)
	defer l.CloseAll()

	c1 := createIdentity(l, services, roster, "onet")

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
	hosts, roster, _ := l.GenTree(5, true)
	services := l.GetServices(hosts, identityService)
	defer l.CloseAll()
	for _, s := range services {
		log.Lvl3(s.(*Service).Storage.Identities)
	}

	c1 := createIdentity(l, services, roster, "one1")
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
	_, roster, _ := l.GenTree(5, true)
	defer l.CloseAll()
	id := NewIdentity(roster, 50, "one1", nil)
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
	hosts, roster, _ := l.GenTree(5, true)
	services := l.GetServices(hosts, identityService)
	defer l.CloseAll()
	kp1 := key.NewKeyPair(tSuite)
	kp2 := key.NewKeyPair(tSuite)
	set := anon.Set([]kyber.Point{kp1.Public, kp2.Public})
	for _, srvc := range services {
		s := srvc.(*Service)
		log.Lvl3(s.Storage.Identities)
		s.Storage.Auth.Sets = append(s.Storage.Auth.Sets, anonSet1{Set: set})
	}

	c1 := NewIdentity(roster, 2, "one", kp1)
	c2 := NewIdentity(roster, 2, "two", nil)
	c3 := NewIdentity(roster, 2, "three", nil)
	defer c1.Client.Close()
	defer c2.Client.Close()
	defer c3.Client.Close()
	log.ErrFatal(c1.CreateIdentity(PoPAuth, set, kp1.Private))
	log.ErrFatal(c2.AttachToIdentity(c1.ID))
	log.ErrFatal(proposeUpVote(c1))
	log.ErrFatal(c3.AttachToIdentity(c1.ID))
	log.ErrFatal(proposeUpVote(c1))
	log.ErrFatal(proposeUpVote(c2))
	log.ErrFatal(c1.DataUpdate())
	log.Lvl2(c1.Data)

	data := c1.GetProposed()
	delete(data.Device, "three")
	log.Lvl2(data)
	log.ErrFatal(c1.ProposeSend(data))
	log.ErrFatal(proposeUpVote(c1))
	log.ErrFatal(proposeUpVote(c2))
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
	hosts, roster, _ := l.GenTree(2, true)
	services := l.GetServices(hosts, identityService)
	s0 := services[0].(*Service)
	defer l.CloseAll()
	for _, s := range services {
		log.Lvl3(s.(*Service).Storage.Identities)
	}

	c1 := createIdentity(l, services, roster, "one1")

	// Hack: create own data-structure with twice our signature
	// and send it directly to the skipblock. Without a proper
	// verification-function, this would pass.
	log.Lvl1("hack data in conode")
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
	sb := id.LatestSkipblock.Copy()
	sb.GenesisID = sb.SkipChainID()
	_, err = s0.storeSkipBlock(sb, data2)
	require.NotNil(t, err, "Skipchain accepted our fake block!")

	log.Lvl1("Trying wrong signature")
	// Gibberish signature
	sig, err = schnorr.Sign(tSuite, c1.Private, hash)
	log.ErrFatal(err)
	// Change one bit in the signature
	sig[len(sig)-1] ^= 1
	data2.Votes["one1"] = sig
	sb = id.LatestSkipblock.Copy()
	sb.GenesisID = sb.SkipChainID()
	_, err = s0.storeSkipBlock(sb, data2)
	require.NotNil(t, err, "Skipchain accepted our fake signature!")

	// Unhack: verify that the correct way of doing it works, even if
	// we bypass the identity.
	log.Lvl1("Using all correct now")
	sig, err = schnorr.Sign(tSuite, c1.Private, hash)
	log.ErrFatal(err)
	data2.Votes["one1"] = sig
	sb = id.LatestSkipblock.Copy()
	sb.GenesisID = sb.SkipChainID()
	_, err = s0.storeSkipBlock(sb, data2)
	log.ErrFatal(err)
	log.ErrFatal(c1.DataUpdate())

	if len(c1.Data.Device) != 2 {
		t.Fatal("Should have two owners now")
	}
}

func proposeUpVote(i *Identity) error {
	if err := i.ProposeUpdate(); err != nil {
		return errors.New("update-error: " + err.Error())
	}
	if err := i.ProposeVote(true); err != nil {
		return errors.New("vote-error: " + err.Error())
	}
	return nil
}

func createIdentity(l *onet.LocalTest, services []onet.Service, roster *onet.Roster, name string) *Identity {
	kp1 := key.NewKeyPair(tSuite)
	kp2 := key.NewKeyPair(tSuite)
	set := anon.Set([]kyber.Point{kp1.Public, kp2.Public})
	for _, srvc := range services {
		s := srvc.(*Service)
		s.Storage.Auth.Sets = append(s.Storage.Auth.Sets, anonSet1{Set: set})
	}

	c := NewTestIdentity(roster, 50, name, l, kp1)
	log.Lvl2("popauth", PoPAuth)
	log.Lvl2("set", set)
	log.ErrFatal(c.CreateIdentity(PoPAuth, set, kp1.Private))
	return c
}

func registerCosiProtocols(c *onet.Context, protoName string) error {
	vf := func(a, b []byte) bool { return true }
	suite := pairing.NewSuiteBn256()
	cosiSubProtoName := protoName + "_sub"

	cosiProto := func(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
		return protocol.NewBlsCosi(n, vf, cosiSubProtoName, suite)
	}
	cosiSubProto := func(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
		return protocol.NewSubBlsCosi(n, vf, suite)
	}

	if _, err := c.ProtocolRegister(protoName, cosiProto); err != nil {
		return err
	}
	if _, err := c.ProtocolRegister(cosiSubProtoName, cosiSubProto); err != nil {
		return err
	}
	return nil
}
