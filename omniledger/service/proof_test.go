package service

import (
	"crypto/sha256"
	"fmt"
	"io/ioutil"
	"testing"

	bolt "github.com/coreos/bbolt"
	"github.com/stretchr/testify/require"
	"gopkg.in/dedis/cothority.v2"
	"gopkg.in/dedis/cothority.v2/byzcoinx"
	"gopkg.in/dedis/cothority.v2/skipchain"
	"gopkg.in/dedis/kyber.v2"
	"gopkg.in/dedis/kyber.v2/sign/cosi"
	"gopkg.in/dedis/kyber.v2/util/key"
	"gopkg.in/dedis/onet.v2"
	"gopkg.in/dedis/onet.v2/network"
)

func TestNewProof(t *testing.T) {
	s := createSC(t)
	p, err := NewProof(s.c, s.s, skipchain.SkipBlockID{}, []byte{})
	require.NotNil(t, err)
	p, err = NewProof(s.c, s.s, s.genesis.Hash, []byte{1})
	require.Nil(t, err)
	require.False(t, p.InclusionProof.Match())
	p, err = NewProof(s.c, s.s, s.genesis.Hash, s.key)
	require.Nil(t, err)
	require.True(t, p.InclusionProof.Match())
}

func TestVerify(t *testing.T) {
	s := createSC(t)
	p, err := NewProof(s.c, s.s, s.genesis.Hash, s.key)
	require.Nil(t, err)
	require.True(t, p.InclusionProof.Match())
	require.Nil(t, p.Verify(s.genesis.SkipChainID()))
	key, values, err := p.KeyValue()
	require.Nil(t, err)
	require.Equal(t, s.key, key)
	require.Equal(t, s.value, values[0])

	require.Equal(t, ErrorVerifySkipchain, p.Verify(s.genesis2.SkipChainID()))

	p.Latest.Data, err = network.Marshal(&DataHeader{
		CollectionRoot: getSBID("123"),
	})
	require.Nil(t, err)
	require.Equal(t, ErrorVerifyCollectionRoot, p.Verify(s.genesis.SkipChainID()))
}

type sc struct {
	c            *collectionDB          // a usable collectionDB to store key/value pairs
	s            *skipchain.SkipBlockDB // a usable skipchain DB to store blocks
	genesis      *skipchain.SkipBlock   // the first genesis block, doesn't hold any data
	genesisPrivs []kyber.Scalar         // private keys of genesis roster
	// second block of skipchain defined by 'genesis'. It holds a key/value
	// in its data and a roster different from the genesis-block.
	sb2      *skipchain.SkipBlock
	genesis2 *skipchain.SkipBlock // a second genesis block with a different roster
	key      []byte               // key stored in sb2
	value    []byte               // value stored in sb2
}

// sc creates an sc structure ready to be used in tests.
func createSC(t *testing.T) (s sc) {
	bnsc := []byte("skipblock-test")
	f, err := ioutil.TempFile("", string(bnsc))
	require.Nil(t, err)
	fname := f.Name()
	require.Nil(t, f.Close())

	db, err := bolt.Open(fname, 0600, nil)
	require.Nil(t, err)

	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucket(bnsc)
		return err
	})
	require.Nil(t, err)
	s.s = skipchain.NewSkipBlockDB(db, bnsc)

	bnol := []byte("omniledger-test")
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucket(bnol)
		return err
	})
	require.Nil(t, err)
	s.c = newCollectionDB(db, bnol)

	s.key = []byte("key")
	s.value = []byte("value")
	s.c.Store(&StateChange{StateAction: Create, ObjectID: s.key, Value: s.value})

	s.genesis = skipchain.NewSkipBlock()
	s.genesis.Roster, s.genesisPrivs = genRoster(1)
	s.genesis.Hash = s.genesis.CalculateHash()

	s.sb2 = skipchain.NewSkipBlock()
	s.sb2.Roster, _ = genRoster(2)
	s.sb2.Data, err = network.Marshal(&DataHeader{
		CollectionRoot: s.c.RootHash(),
	})
	require.Nil(t, err)
	s.sb2.Hash = s.sb2.CalculateHash()
	s.genesis.ForwardLink = genForwardLink(t, s.genesis, s.sb2, s.genesisPrivs)

	s.s.Store(s.genesis)
	s.s.Store(s.sb2)

	s.genesis2 = skipchain.NewSkipBlock()
	s.genesis2.Roster, _ = genRoster(2)
	s.genesis2.Hash = s.genesis2.CalculateHash()
	s.s.Store(s.genesis2)
	return
}

func genForwardLink(t *testing.T, from, to *skipchain.SkipBlock, privs []kyber.Scalar) []*skipchain.ForwardLink {
	fwd := &skipchain.ForwardLink{
		From: from.Hash,
		To:   to.Hash,
	}
	if !from.Roster.ID.Equal(to.Roster.ID) {
		fwd.NewRoster = to.Roster
	}
	v, V := cosi.Commit(cothority.Suite)
	ch, err := cosi.Challenge(cothority.Suite, V, from.Roster.Aggregate, fwd.Hash())
	require.Nil(t, err)
	resp, err := cosi.Response(cothority.Suite, privs[0], v, ch)
	require.Nil(t, err)
	mask, err := cosi.NewMask(cothority.Suite, from.Roster.Publics(), from.Roster.Publics()[0])
	require.Nil(t, err)
	sig, err := cosi.Sign(cothority.Suite, V, resp, mask)
	require.Nil(t, err)
	fwd.Signature = byzcoinx.FinalSignature{
		Msg: fwd.Hash(),
		Sig: sig,
	}
	require.Nil(t, err)
	return []*skipchain.ForwardLink{fwd}
}

func getSBID(s string) skipchain.SkipBlockID {
	s256 := sha256.Sum256([]byte(s))
	return skipchain.SkipBlockID(s256[:])
}

func genRoster(num int) (*onet.Roster, []kyber.Scalar) {
	var ids []*network.ServerIdentity
	var privs []kyber.Scalar
	for i := 0; i < num; i++ {
		n := network.Address(fmt.Sprintf("tls://0.0.0.%d:2000", 2*i+1))
		kp := key.NewKeyPair(cothority.Suite)
		ids = append(ids, network.NewServerIdentity(kp.Public, n))
		privs = append(privs, kp.Private)
	}
	return onet.NewRoster(ids), privs
}

func overwriteSB(t *testing.T, s sc, sb *skipchain.SkipBlock) {
	// We have to update manually, because s.s.Store checks
	// the validity of the block.
	err := s.s.Update(func(tx *bolt.Tx) error {
		key := sb.Hash
		val, err := network.Marshal(sb)
		if err != nil {
			return err
		}
		return tx.Bucket([]byte("skipblock-test")).Put(key, val)
	})
	require.Nil(t, err)
}
