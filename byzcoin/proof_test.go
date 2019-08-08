package byzcoin

import (
	"crypto/sha256"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3/byzcoinx"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/pairing"
	"go.dedis.ch/kyber/v3/sign/bls"
	"go.dedis.ch/kyber/v3/util/key"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/network"
	"go.dedis.ch/protobuf"
	bbolt "go.etcd.io/bbolt"
)

func TestNewProof(t *testing.T) {
	s := createSC(t)
	p, err := NewProof(s.c, s.s, skipchain.SkipBlockID{}, []byte{})
	require.NotNil(t, err)

	key := []byte{1}
	p, err = NewProof(s.c, s.s, s.genesis.Hash, key)
	require.Nil(t, err)
	require.False(t, p.InclusionProof.Match(key))

	p, err = NewProof(s.c, s.s, s.genesis.Hash, s.key)
	require.Nil(t, err)
	require.True(t, p.InclusionProof.Match(s.key))
}

func TestVerify(t *testing.T) {
	s := createSC(t)
	p, err := NewProof(s.c, s.s, s.genesis.Hash, s.key)
	require.Nil(t, err)
	require.True(t, p.InclusionProof.Match(s.key))
	require.Nil(t, p.Verify(s.genesis.SkipChainID()))
	key, val, _, _, err := p.KeyValue()
	require.Nil(t, err)
	require.Equal(t, s.key, key)
	require.Equal(t, s.value, val)

	require.Equal(t, ErrorVerifySkipchain, p.Verify(s.genesis2.SkipChainID()))

	p.Latest.BaseHeight = 123
	require.Equal(t, ErrorVerifyHash, p.Verify(s.genesis.SkipChainID()))

	p.Latest.Data, err = protobuf.Encode(&DataHeader{
		TrieRoot: getSBID("123"),
	})
	require.Nil(t, err)
	require.Equal(t, ErrorVerifyTrieRoot, p.Verify(s.genesis.SkipChainID()))
}

type sc struct {
	c            *stateTrie             // a usable collectionDB to store key/value pairs
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

	db, err := bbolt.Open(fname, 0600, nil)
	require.Nil(t, err)

	err = db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucket(bnsc)
		return err
	})
	require.Nil(t, err)
	s.s = skipchain.NewSkipBlockDB(db, bnsc)

	bucketName := []byte("a testing string")
	err = db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucket(bucketName)
		return err
	})
	require.Nil(t, err)
	s.c, err = newStateTrie(db, bucketName, []byte("nonce string"))
	require.NoError(t, err)

	s.key = []byte("key")
	s.value = []byte("value")
	s.c.StoreAll([]StateChange{{StateAction: Create, InstanceID: s.key, Value: s.value}}, 1, CurrentVersion)

	s.genesis = skipchain.NewSkipBlock()
	s.genesis.Height = 1
	s.genesis.Roster, s.genesisPrivs = genRoster(1)
	s.genesis.Hash = s.genesis.CalculateHash()

	s.sb2 = skipchain.NewSkipBlock()
	s.sb2.Height = 1
	s.sb2.Roster, _ = genRoster(2)
	s.sb2.Data, err = protobuf.Encode(&DataHeader{
		TrieRoot: s.c.GetRoot(),
	})
	require.Nil(t, err)
	s.sb2.Index = 1
	s.sb2.Hash = s.sb2.CalculateHash()
	s.genesis.ForwardLink = genForwardLink(t, s.genesis, s.sb2, s.genesisPrivs)

	_, err = s.s.StoreBlocks([]*skipchain.SkipBlock{s.genesis, s.sb2})
	require.Nil(t, err)

	s.genesis2 = skipchain.NewSkipBlock()
	s.genesis2.Height = 1
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
	sig, err := bls.Sign(pairing.NewSuiteBn256(), privs[0], fwd.Hash())
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
		kp := key.NewKeyPair(pairing.NewSuiteBn256())
		ids = append(ids, network.NewServerIdentity(kp.Public, n))
		privs = append(privs, kp.Private)
	}
	return onet.NewRoster(ids), privs
}
