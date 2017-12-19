package skipchain

import (
	"bytes"
	"crypto/sha512"
	"io/ioutil"
	"os"
	"testing"

	bolt "github.com/coreos/bbolt"
	"github.com/dedis/cothority/bftcosi"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSkipBlock_GetResponsible(t *testing.T) {
	l := onet.NewTCPTest(tSuite)
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
	inter0.ParentBlockID = root1.Hash
	inter0.Roster = roster
	inter0.Hash = inter0.CalculateHash()
	db.Store(inter0)
	inter1 := inter0.Copy()
	inter1.Index++
	inter1.BackLinkIDs = []SkipBlockID{inter0.Hash}

	b, err := db.GetResponsible(root0)
	log.ErrFatal(err)
	assert.True(t, root0.Equal(b))

	b, err = db.GetResponsible(root1)
	log.ErrFatal(err)
	assert.True(t, root0.Equal(b))

	b, err = db.GetResponsible(inter0)
	log.ErrFatal(err)
	assert.Equal(t, root1.Hash, b.Hash)

	b, err = db.GetResponsible(inter1)
	log.ErrFatal(err)
	assert.True(t, inter0.Equal(b))
}

func TestSkipBlock_VerifySignatures(t *testing.T) {
	l := onet.NewTCPTest(tSuite)
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
	require.Nil(t, block1.VerifyForwardSignatures())
	require.NotNil(t, db.VerifyLinks(block1))
}

func TestSkipBlock_Hash1(t *testing.T) {
	sbd1 := NewSkipBlock()
	sbd1.Data = []byte("1")
	sbd1.Height = 4
	h1 := sbd1.updateHash()
	assert.Equal(t, h1, sbd1.Hash)

	sbd2 := NewSkipBlock()
	sbd2.Data = []byte("2")
	sbd1.Height = 2
	h2 := sbd2.updateHash()
	assert.NotEqual(t, h1, h2)
}

func TestSkipBlock_Hash2(t *testing.T) {
	local := onet.NewLocalTest(tSuite)
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

func TestBlockLink_Copy(t *testing.T) {
	// Test if copy is deep or only shallow
	b1 := &BlockLink{}
	b1.Signature = []byte{1}
	b2 := b1.Copy()
	b2.Signature = []byte{2}
	if bytes.Equal(b1.Signature, b2.Signature) {
		t.Fatal("They should not be equal")
	}

	sb1 := NewSkipBlock()
	sb1.ChildSL = append(sb1.ChildSL, []byte{3})
	sb2 := sb1.Copy()
	sb1.ChildSL[0] = []byte{1}
	sb2.ChildSL[0] = []byte{2}
	if bytes.Equal(sb1.ChildSL[0], sb2.ChildSL[0]) {
		t.Fatal("They should not be equal")
	}
	sb1.Height = 10
	sb2.Height = 20
	if sb1.Height == sb2.Height {
		t.Fatal("Should not be equal")
	}
}

func TestSign(t *testing.T) {
	l := onet.NewTCPTest(tSuite)
	servers, roster, _ := l.GenTree(10, true)
	msg := sha512.New().Sum(nil)
	sig, err := sign(msg, servers, l)
	log.ErrFatal(err)
	log.ErrFatal(sig.Verify(tSuite, roster.Publics()))
	sig.Msg = sha512.New().Sum([]byte{1})
	require.NotNil(t, sig.Verify(tSuite, roster.Publics()))
	defer l.CloseAll()
}

func sign(msg SkipBlockID, servers []*onet.Server, l *onet.LocalTest) (*bftcosi.BFTSignature, error) {
	aggScalar := tSuite.Scalar().Zero()
	aggPoint := tSuite.Point().Null()
	for _, s := range servers {
		aggScalar.Add(aggScalar, l.GetPrivate(s))
		aggPoint.Add(aggPoint, s.ServerIdentity.Public)
	}
	rand := tSuite.Scalar().Pick(tSuite.RandomStream())
	comm := tSuite.Point().Mul(rand, nil)
	sigC, err := comm.MarshalBinary()
	if err != nil {
		return nil, err
	}
	hash := sha512.New()
	hash.Write(sigC)
	aggPoint.MarshalTo(hash)
	hash.Write(msg)
	challBuff := hash.Sum(nil)
	chall := tSuite.Scalar().SetBytes(challBuff)
	resp := tSuite.Scalar().Mul(aggScalar, chall)
	resp = resp.Add(rand, resp)
	sigR, err := resp.MarshalBinary()
	if err != nil {
		return nil, err
	}
	sig := make([]byte, 64+len(servers)/8)
	copy(sig[:], sigC)
	copy(sig[32:64], sigR)
	return &bftcosi.BFTSignature{Sig: sig, Msg: msg, Exceptions: nil}, nil
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

	db.Update(func(tx *bolt.Tx) error {
		err := db.storeToTx(tx, sb0)
		require.Nil(t, err)

		err = db.storeToTx(tx, sb1)
		require.Nil(t, err)
		return nil
	})

	require.Nil(t, db.GetFuzzy(""))

	sb := db.GetFuzzy("01")
	require.NotNil(t, sb)
	require.Equal(t, sb.Data[0], sb0.Data[0])

	sb = db.GetFuzzy("02")
	require.NotNil(t, sb)
	require.Equal(t, sb.Data[0], sb1.Data[0])

	sb = db.GetFuzzy("03")
	require.Nil(t, sb)

	sb = db.GetFuzzy("04")
	require.Nil(t, sb)

	sb = db.GetFuzzy("05")
	require.NotNil(t, sb)
	require.Equal(t, sb.Data[0], sb0.Data[0])

	sb = db.GetFuzzy("06")
	require.Nil(t, sb)

	sb = db.GetFuzzy("0102030605")
	require.NotNil(t, sb)
	require.Equal(t, sb.Data[0], sb0.Data[0])
}

// setupSkipBlockDB initialises a database with a bucket called 'skipblock-test' inside.
// The caller is responsible to close and remove the database file after using it.
func setupSkipBlockDB(t *testing.T) (*SkipBlockDB, string) {
	f, err := ioutil.TempFile("", "skipblock-test")
	require.Nil(t, err)
	fname := f.Name()
	require.Nil(t, f.Close())

	db, err := bolt.Open(fname, 0600, nil)
	require.Nil(t, err)

	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucket([]byte("skipblock-test"))
		return err
	})
	require.Nil(t, err)

	return &SkipBlockDB{db, "skipblock-test"}, fname
}
