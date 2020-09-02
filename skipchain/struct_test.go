package skipchain

import (
	"bytes"
	"fmt"
	"go.dedis.ch/kyber/v3/util/random"
	"io/ioutil"
	"os"
	"testing"
	"time"

	uuid "github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoinx"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/pairing"
	"go.dedis.ch/kyber/v3/sign"
	"go.dedis.ch/kyber/v3/sign/bls"
	"go.dedis.ch/kyber/v3/suites"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
	bbolt "go.etcd.io/bbolt"
)

var pairingSuite = pairing.NewSuiteBn256()

func TestSkipBlock_GetResponsible(t *testing.T) {
	l := onet.NewTCPTest(suite)
	_, roster, _ := l.GenTree(3, true)
	defer l.CloseAll()

	db, fname := setupSkipBlockDB(t)
	defer db.Close()
	defer os.Remove(fname)

	root0 := NewSkipBlock()
	root0.Roster = roster
	root0.Hash = root0.CalculateHash()
	root0.BackLinkIDs = []SkipBlockID{root0.Hash}
	db.Store(root0)
	root1 := root0.Copy()
	root1.Index++
	db.Store(root1)
	inter0 := NewSkipBlock()
	inter0.Roster = roster
	inter0.Hash = inter0.CalculateHash()
	db.Store(inter0)
	inter1 := inter0.Copy()
	inter1.Index++
	inter1.BackLinkIDs = []SkipBlockID{inter0.Hash}

	b, err := db.GetResponsible(root0)
	require.NoError(t, err)
	assert.True(t, root0.Equal(b))

	b, err = db.GetResponsible(root1)
	require.NoError(t, err)
	assert.True(t, root0.Equal(b))

	b, err = db.GetResponsible(inter0)
	require.NoError(t, err)
	assert.Equal(t, root1.Hash, b.Hash)

	b, err = db.GetResponsible(inter1)
	require.NoError(t, err)
	assert.True(t, inter0.Equal(b))
}

func TestSkipBlock_VerifySignatures(t *testing.T) {
	l := onet.NewTCPTest(suite)
	_, roster3, _ := l.GenTree(3, true)
	defer l.CloseAll()
	roster2 := onet.NewRoster(roster3.List[0:2])

	db, fname := setupSkipBlockDB(t)
	defer db.Close()
	defer os.Remove(fname)

	root := NewSkipBlock()
	root.Roster = roster2
	root.BackLinkIDs = append(root.BackLinkIDs, SkipBlockID{1, 2, 3, 4})
	root.Hash = root.CalculateHash()
	db.Store(root)
	log.ErrFatal(root.VerifyForwardSignatures())
	log.ErrFatal(db.VerifyLinks(root))

	block1 := root.Copy()
	block1.BackLinkIDs = append(block1.BackLinkIDs, root.Hash)
	block1.Index++
	db.Store(block1)
	require.NotNil(t, block1.VerifyForwardSignatures())
	block1.updateHash()
	require.Nil(t, block1.VerifyForwardSignatures())
	require.NotNil(t, db.VerifyLinks(block1))

	block1.Roster = nil
	block1.updateHash()
	err := block1.VerifyForwardSignatures()
	require.NotNil(t, err)
	require.Equal(t, "Missing roster in the block", err.Error())
}

func TestSkipBlock_InvalidForwardLinks(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()
	_, ro, service := local.MakeSRS(cothority.Suite, 3, skipchainSID)

	s := service.(*Service)

	sbRoot, err := makeGenesisRosterArgs(s, ro, nil, VerificationStandard, 2, 2)
	require.NoError(t, err)

	sb := NewSkipBlock()
	sb.Roster = ro
	for i := 0; i < 2; i++ {
		_, err := s.StoreSkipBlock(&StoreSkipBlock{TargetSkipChainID: sbRoot.Hash, NewBlock: sb})
		require.NoError(t, err)
	}

	sb2 := s.db.GetByID(sbRoot.Hash)
	sb2.ForwardLink = append(sb2.ForwardLink, sb2.ForwardLink[0])

	log.OutputToBuf()
	defer log.OutputToOs()

	// Try to add a forward link at a wrong height.
	s.db.Store(sb2)
	require.Contains(t, log.GetStdErr(), "Received a forward link with an invalid height")

	gb2 := s.db.GetByID(sbRoot.Hash)
	gb2.BackLinkIDs = []SkipBlockID{[]byte{1, 2, 3}}
	gb2.updateHash()
	// Try to store a new block with old forward-links
	s.db.Store(gb2)
	require.Contains(t, log.GetStdErr(), "found inconsistent forward-link")

	gb2.Height = 0
	// Try to store a new block with too many forward-links
	s.db.Store(gb2)
	require.Contains(t, log.GetStdErr(), "found 1 forward-links for a height of 0")

	require.NoError(t, local.WaitDone(time.Second))
}

func TestSkipBlock_WrongSignatures(t *testing.T) {
	fl := ForwardLink{
		From:      SkipBlockID{},
		To:        SkipBlockID{},
		Signature: byzcoinx.FinalSignature{},
	}
	err := fl.VerifyWithScheme(suite, []kyber.Point{}, 0)
	require.Error(t, err)
	require.Equal(t, "wrong hash of forward link", err.Error())

	fl.Signature.Msg = fl.Hash()

	err = fl.VerifyWithScheme(suite, []kyber.Point{}, 123456789)
	require.Error(t, err)
	require.Equal(t, "unknown signature scheme", err.Error())
	err = fl.VerifyWithScheme(suite, []kyber.Point{}, BlsSignatureSchemeIndex)
	require.Error(t, err)
	require.NotEqual(t, "unknown signature scheme", err.Error())
	err = fl.VerifyWithScheme(suite, []kyber.Point{}, BdnSignatureSchemeIndex)
	require.Error(t, err)
	require.NotEqual(t, "unknown signature scheme", err.Error())
}

func TestSkipBlock_Hash1(t *testing.T) {
	// Needed for the roster.
	s := suites.MustFind("ed25519")
	si := network.NewServerIdentity(s.Point(), "tcp://127.0.0.1:2000")

	sbd1 := NewSkipBlock()
	sbd1.Data = []byte("1")
	sbd1.Height = 4
	sbd1.Roster = onet.NewRoster([]*network.ServerIdentity{si})
	h1 := sbd1.updateHash()
	assert.Equal(t, h1, sbd1.Hash)
	// Dump the current hash, to put it into the Java test.
	//t.Logf("%x", h1) // 1304bd5ecad8d54a2fd7b81a8864f698966308104b20780b634c4b237b843823

	// Dumping the skipblock, to put it into the Java test.
	// buf, err := protobuf.Encode(sbd1)
	// require.NoError(t, err)
	// t.Logf("%x", buf) // 08001008180020003a004201314a94010a106bc1027de8ef542e8b09219c287b2fde12560a2865642e706f696e7400000000000000000000000000000000000000000000000000000000000000001a103809e37975a45b4a865899668d645d9522147463703a2f2f3132372e302e302e313a323030302a003a001a2865642e706f696e74000000000000000000000000000000000000000000000000000000000000000052201304bd5ecad8d54a2fd7b81a8864f698966308104b20780b634c4b237b8438236200

	// Clone: equal
	sbd2 := sbd1.Copy()
	assert.Equal(t, sbd2.Hash, sbd1.Hash)
	// update with no changes: still equal
	h2 := sbd2.updateHash()
	assert.Equal(t, h1, h2)

	// Change height: not equal
	sbd2.Height++
	h2 = sbd2.updateHash()
	assert.NotEqual(t, h1, h2)

	// Clone, then change field Data: not equal
	sbd2 = sbd1.Copy()
	sbd2.Data[0]++
	h2 = sbd2.updateHash()
	assert.NotEqual(t, h1, h2)

	sbd2 = sbd1.Copy()
	sbd2.SignatureScheme++
	h2 = sbd2.updateHash()
	assert.NotEqual(t, h1, h2)
}

func TestSkipBlock_Hash2(t *testing.T) {
	local := onet.NewLocalTest(suite)
	hosts, el, _ := local.GenTree(2, false)
	defer local.CloseAll()
	sbd1 := NewSkipBlock()
	sbd1.Roster = el
	sbd1.Height = 1
	h1 := sbd1.updateHash()
	assert.Equal(t, h1, sbd1.Hash)

	sbd2 := NewSkipBlock()
	sbd2.Roster = local.GenRosterFromHost(hosts[0])
	sbd2.Height = 1
	h2 := sbd2.updateHash()
	assert.NotEqual(t, h1, h2)
}

func TestSkipBlock_VerifierIDs(t *testing.T) {
	require.True(t, VerifierIDs(VerificationNone).Equal(VerificationNone))
	require.False(t, VerifierIDs(VerificationStandard).Equal(VerificationNone))
	require.True(t, VerifierIDs(VerificationStandard).Equal(VerificationStandard))

	vv1 := VerifierIDs{VerifyBase, VerifierID(uuid.NewV5(uuid.NamespaceURL, "abc"))}
	vv2 := make(VerifierIDs, 2)
	vv2[0] = vv1[1]
	vv2[1] = vv1[0]
	require.False(t, vv1.Equal(vv2))
}

func TestBlockLink_Copy(t *testing.T) {
	// Test if copy is deep or only shallow
	b1 := &ForwardLink{}
	b1.Signature.Sig = []byte{1}
	b2 := b1.Copy()
	b2.Signature.Sig[0] = byte(2)
	if bytes.Equal(b1.Signature.Sig, b2.Signature.Sig) {
		t.Fatal("They should not be equal")
	}

	sb1 := NewSkipBlock()
	sb2 := sb1.Copy()
	sb1.Height = 10
	sb2.Height = 20
	if sb1.Height == sb2.Height {
		t.Fatal("Should not be equal")
	}
}

func TestSkipBlock_GetFuzzy(t *testing.T) {
	db, fname := setupSkipBlockDB(t)
	defer db.Close()
	defer os.Remove(fname)

	sb0 := NewSkipBlock()
	sb0.Data = []byte{0}
	sb0.Hash = []byte{1, 2, 3, 6, 5}

	sb1 := NewSkipBlock()
	sb1.Data = []byte{1}
	sb1.Hash = []byte{2, 3, 4, 1, 5}

	db.Update(func(tx *bbolt.Tx) error {
		err := db.storeToTx(tx, sb0)
		require.NoError(t, err)

		err = db.storeToTx(tx, sb1)
		require.NoError(t, err)
		return nil
	})

	sb, err := db.GetFuzzy("")
	require.Nil(t, sb)
	require.NotNil(t, err)

	sb, err = db.GetFuzzy("01")
	require.NoError(t, err)
	require.NotNil(t, sb)
	require.Equal(t, sb.Data[0], sb0.Data[0])

	sb, err = db.GetFuzzy("02")
	require.NoError(t, err)
	require.NotNil(t, sb)
	require.Equal(t, sb.Data[0], sb1.Data[0])

	sb, err = db.GetFuzzy("03")
	require.NoError(t, err)
	require.Nil(t, sb)

	sb, err = db.GetFuzzy("04")
	require.NoError(t, err)
	require.Nil(t, sb)

	sb, err = db.GetFuzzy("05")
	require.NoError(t, err)
	require.NotNil(t, sb)
	require.Equal(t, sb.Data[0], sb0.Data[0])

	sb, err = db.GetFuzzy("06")
	require.NoError(t, err)
	require.Nil(t, sb)

	sb, err = db.GetFuzzy("0102030605")
	require.NoError(t, err)
	require.NotNil(t, sb)
	require.Equal(t, sb.Data[0], sb0.Data[0])
}

func TestSkipBlock_Payload(t *testing.T) {
	sb := NewSkipBlock()
	h := sb.CalculateHash()
	sb.Payload = []byte{1, 2, 3}
	require.Equal(t, h, sb.CalculateHash())
}

// Vector testing of the function to get the index of the next
// block when following the chain.
func TestSkipBlock_PathForIndex(t *testing.T) {
	sb := NewSkipBlock()

	vectors := []struct{ index, height, base, target, expected int }{
		{0, 6, 2, 32, 32},
		{0, 6, 4, 32, 16},
		{0, 2, 4, 32, 4},
		{1, 1, 2, 3, 2},
		{0, 6, 2, 31, 16},
		{0, 1, 2, 0, 0},
		{1, 1, 2, 1, 1},
		// backwards test
		{32, 6, 2, 0, 0},
		{32, 6, 2, 1, 16},
	}

	for _, v := range vectors {
		sb.Index = v.index
		sb.Height = v.height
		sb.BaseHeight = v.base

		_, idx := sb.pathForIndex(v.target)
		require.Equal(t, v.expected, idx, fmt.Sprintf("%v", v))
	}
}

// This checks if the it returns the shortest path or an error
// when blocks are missing
func TestSkipBlockDB_GetProof(t *testing.T) {
	local := onet.NewLocalTest(suite)
	_, ro, _ := local.GenTree(2, false)
	defer local.CloseAll()

	db, file := setupSkipBlockDB(t)
	defer os.Remove(file)

	root := NewSkipBlock()
	root.Roster = ro
	root.Index = 0
	root.Height = 2
	root.BaseHeight = 2
	root.updateHash()
	sb1 := NewSkipBlock()
	sb1.Roster = ro
	sb1.Index = 1
	sb1.Height = 1
	sb1.BaseHeight = 2
	sb1.BackLinkIDs = []SkipBlockID{root.Hash}
	sb1.updateHash()
	sb2 := NewSkipBlock()
	sb2.Roster = ro
	sb2.Index = 2
	sb2.BaseHeight = 2
	sb2.GenesisID = root.Hash
	sb2.BackLinkIDs = []SkipBlockID{sb1.Hash}
	sb2.updateHash()
	sb1.ForwardLink = []*ForwardLink{{From: sb1.Hash, To: sb2.Hash}}
	require.NoError(t, sb1.ForwardLink[0].sign(ro))
	root.ForwardLink = []*ForwardLink{
		{From: root.Hash, To: sb1.Hash},
		{From: root.Hash, To: sb2.Hash},
	}
	require.NoError(t, root.ForwardLink[0].sign(ro))
	require.NoError(t, root.ForwardLink[1].sign(ro))

	_, err := db.StoreBlocks([]*SkipBlock{root, sb1, sb2})
	require.NoError(t, err)

	blocks, err := db.GetProof(root.Hash)
	require.NoError(t, err)
	require.Equal(t, 2, len(blocks))
	require.True(t, blocks[1].Hash.Equal(sb2.Hash))

	blocks, err = db.GetProofForID(sb2.Hash)
	require.NoError(t, err)
	require.Equal(t, 2, len(blocks))
	require.True(t, blocks[1].Hash.Equal(sb2.Hash))

	err = db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(db.bucketName).Delete(sb2.Hash)
	})
	require.NoError(t, err)

	// last block is missing so it should return only until sb1.
	bb, err := db.GetProof(root.Hash)
	require.NoError(t, err)
	require.Equal(t, 2, len(bb))

	_, err = db.GetProofForID(sb2.Hash)
	require.Error(t, err)
}

func TestNewSkipBlockDB_getAllSkipchains(t *testing.T) {
	db, fname, scIDs := setupSkipchain(t, 10)
	defer db.Close()
	defer os.Remove(fname)

	start := time.Now()
	readIDs, err := db.getAllSkipchains()
	require.NoError(t, err)
	log.Lvl1("Time for reading:", time.Now().Sub(start))
	require.Equal(t, len(scIDs), len(readIDs))
searchIDs:
	for sc := range readIDs {
		for _, scOrig := range scIDs {
			if bytes.Equal([]byte(sc), scOrig) {
				continue searchIDs
			}
		}
		require.Fail(t, "found unknown skipchain-ID")
	}
}

// Writing this as a benchmark is difficult, as b.N gets way too big...
func TestBenchmarkSkipchains(t *testing.T) {
	// Increasing this value makes the new method even faster.
	// With 1000 blocks per chain, the speed-up is around 175 faster.
	nbrSkipBlocks := 10

	db, fname, _ := setupSkipchain(t, nbrSkipBlocks)
	defer db.Close()
	defer os.Remove(fname)

	start := time.Now()
	_, err := getAllSkipchainsOld(db)
	require.NoError(t, err)
	log.Lvl1("Time for reading old:", time.Now().Sub(start))

	start = time.Now()
	_, err = db.getAllSkipchains()
	require.NoError(t, err)
	log.Lvl1("Time for reading new:", time.Now().Sub(start))
}

// This creates skipchains with wrong forward-links and that would not pass
// tests.
// So it can only be used in the benchmark / test methods of
// getAllSkipchains, and not as generic skipchain-producer.
func setupSkipchain(t *testing.T, nbrSkipBlocks int) (db *SkipBlockDB,
	fname string, scIDs []SkipBlockID) {
	nbrNodes := 10
	nbrSkipChains := 10
	var dataSize uint = 1024
	var payloadSize uint = 1024

	l := onet.NewTCPTest(suite)
	_, roster, _ := l.GenTree(nbrNodes, true)
	defer l.CloseAll()

	db, fname = setupSkipBlockDB(t)

	log.Lvl1("Setting up skipchains")
	vids := []VerifierID{VerifierID(uuid.NewV4()),
		VerifierID(uuid.NewV4()),
		VerifierID(uuid.NewV4())}
	rand32 := random.Bits(32*8, true, random.New())
	rand64 := random.Bits(64*8, true, random.New())
	randData := random.Bits(dataSize*8, true, random.New())
	randPayload := random.Bits(payloadSize*8, true, random.New())
	blIDs := []SkipBlockID{rand32, rand32}
	fls := []*ForwardLink{
		{rand32, rand32, roster,
			byzcoinx.FinalSignature{Msg: rand32, Sig: rand64}},
		{rand32, rand32, roster,
			byzcoinx.FinalSignature{Msg: rand32, Sig: rand64}},
	}
	for sc := 0; sc < nbrSkipChains; sc++ {
		log.Lvl2("Setting up skipchain", sc)
		root := NewSkipBlock()
		root.Roster = roster
		root.BackLinkIDs = blIDs
		root.VerifierIDs = vids
		root.Data = append(randData, byte(sc))
		root.Height = 2
		root.MaximumHeight = 10
		root.BaseHeight = 1
		root.Payload = randPayload
		next := random.Bits(32*8, true, random.New())
		root.ForwardLink = fls
		root.ForwardLink[0].To = next
		root.Hash = root.CalculateHash()
		require.NoError(t, storeRaw(db, root))
		scIDs = append(scIDs, root.Hash)

		for sb := 1; sb < nbrSkipBlocks; sb++ {
			sblock := root.Copy()
			sblock.Index = sb
			sblock.GenesisID = root.Hash
			sblock.Hash = sblock.CalculateHash()
			require.NoError(t, storeRaw(db, sblock))
		}
	}

	require.NoError(t, db.Close())
	bb, err := bbolt.Open(fname, 0600, nil)
	require.NoError(t, err)
	db = NewSkipBlockDB(bb, []byte("skipblock-test"))
	return
}

func storeRaw(db *SkipBlockDB, sb *SkipBlock) error {
	return db.Update(func(tx *bbolt.Tx) error {
		return db.storeToTx(tx, sb)
	})
}

// getAllSkipchains returns each of the skipchains in the database
// in the form of a map from skipblock ID to the latest block.
func getAllSkipchainsOld(db *SkipBlockDB) (map[string]*SkipBlock, error) {
	gen := make(map[string]*SkipBlock)

	// Loop over all blocks. If we see a new genesis block we
	// have not seen, remember it. If we see a higher Index than what
	// we have, replace it.
	err := db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(db.bucketName))
		return b.ForEach(func(k, v []byte) error {
			_, sbMsg, err := network.Unmarshal(v, suite)
			if err != nil {
				return err
			}
			sb, ok := sbMsg.(*SkipBlock)
			if ok {
				k := string(sb.SkipChainID())
				if cur, ok := gen[k]; ok {
					if cur.Index < sb.Index {
						gen[k] = sb
					}
				} else {
					gen[k] = sb
				}
			}
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return gen, nil
}

// Test the edge cases of the verification function
func TestProof_Verify(t *testing.T) {
	sb := NewSkipBlock()
	sb.updateHash()

	require.NotNil(t, Proof{}.Verify())
	sb.Index = 1
	require.NotNil(t, Proof{sb}.Verify())

	require.NotNil(t, Proof{}.VerifyFromID(sb.Hash))
	require.NotNil(t, Proof{sb}.VerifyFromID(SkipBlockID{}))
}

// setupSkipBlockDB initialises a database with a bucket called 'skipblock-test' inside.
// The caller is responsible to close and remove the database file after using it.
func setupSkipBlockDB(t *testing.T) (*SkipBlockDB, string) {
	f, err := ioutil.TempFile("", "skipblock-test")
	require.NoError(t, err)
	fname := f.Name()
	require.NoError(t, f.Close())

	db, err := bbolt.Open(fname, 0600, nil)
	require.NoError(t, err)

	err = db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucket([]byte("skipblock-test"))
		return err
	})
	require.NoError(t, err)

	return NewSkipBlockDB(db, []byte("skipblock-test")), fname
}

// Checks if the buffer api works as expected
func TestBlockBuffer(t *testing.T) {
	bb := newSkipBlockBuffer()
	sid := []byte{1}
	bid := []byte{2}

	sb := NewSkipBlock()
	sb.Index = 1
	sb.GenesisID = sid
	sb.Hash = bid
	bb.add(sb)

	sb = bb.get(sid, bid)
	require.NotNil(t, sb)
	require.True(t, bb.has(sid))
	require.False(t, bb.has(bid))

	// wrong key
	sb = bb.get(bid, bid)
	require.Nil(t, sb)

	// wrong block id
	sb = bb.get(sid, sid)
	require.Nil(t, sb)

	bb.clear(sid)
	sb = bb.get(sid, bid)
	require.Nil(t, sb)
}

func (fl *ForwardLink) sign(ro *onet.Roster) error {
	msg := fl.Hash()
	mask, err := sign.NewMask(pairingSuite, ro.ServicePublics(ServiceName), nil)
	if err != nil {
		return err
	}
	sigs := make([][]byte, len(ro.List))
	for i, si := range ro.List {
		sig, err := bls.Sign(pairingSuite, si.ServicePrivate(ServiceName), msg)
		if err != nil {
			return err
		}
		sigs[i] = sig

		err = mask.SetBit(i, true)
		if err != nil {
			return err
		}
	}

	agg, err := bls.AggregateSignatures(pairingSuite, sigs...)
	if err != nil {
		return err
	}

	fl.Signature = byzcoinx.FinalSignature{
		Msg: msg,
		Sig: append(agg, mask.Mask()...),
	}

	return nil
}
