package libsc_test

import (
	"testing"

	"crypto/sha512"

	"bytes"

	"github.com/dedis/cothority/bftcosi"
	"github.com/dedis/cothority/skipchain/libsc"
	_ "github.com/dedis/cothority/skipchain/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/dedis/crypto.v0/random"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/log"
	"gopkg.in/dedis/onet.v1/network"
)

func TestSkipBlock_VerifySignatures(t *testing.T) {
	l := onet.NewTCPTest()
	_, roster3, _ := l.GenTree(3, true)
	defer l.CloseAll()
	roster2 := onet.NewRoster(roster3.List[0:2])
	root := libsc.NewSkipBlock()
	root.Roster = roster2
	root.BackLinkIDs = append(root.BackLinkIDs, libsc.SkipBlockID{1, 2, 3, 4})
	root.Hash = root.CalculateHash()
	sbm := libsc.NewSkipBlockBunch(root)
	log.ErrFatal(root.VerifyForwardSignatures())
	log.ErrFatal(sbm.VerifyLinks(root))

	block1 := root.Copy()
	block1.BackLinkIDs = append(block1.BackLinkIDs, root.Hash)
	block1.Index++
	sbm.Store(block1)
	require.Nil(t, block1.VerifyForwardSignatures())
	require.NotNil(t, sbm.VerifyLinks(block1))
}

func TestSkipBlock_Hash1(t *testing.T) {
	sbd1 := libsc.NewSkipBlock()
	sbd1.Data = []byte("1")
	sbd1.Height = 4
	h1 := sbd1.UpdateHash()
	assert.Equal(t, h1, sbd1.Hash)

	sbd2 := libsc.NewSkipBlock()
	sbd2.Data = []byte("2")
	sbd1.Height = 2
	h2 := sbd2.UpdateHash()
	assert.NotEqual(t, h1, h2)
}

func TestSkipBlock_Hash2(t *testing.T) {
	local := onet.NewLocalTest()
	hosts, el, _ := local.GenTree(2, false)
	defer local.CloseAll()
	sbd1 := libsc.NewSkipBlock()
	sbd1.Roster = el
	sbd1.Height = 1
	h1 := sbd1.UpdateHash()
	assert.Equal(t, h1, sbd1.Hash)

	sbd2 := libsc.NewSkipBlock()
	sbd2.Roster = local.GenRosterFromHost(hosts[0])
	sbd2.Height = 1
	h2 := sbd2.UpdateHash()
	assert.NotEqual(t, h1, h2)
}

func TestSkipBlock_Short(t *testing.T) {
	sb := libsc.NewSkipBlock()
	sb.Hash = []byte{1, 2, 3, 4, 5, 6, 7, 8, 9}
	sb.Roster = &onet.Roster{}
	require.Equal(t, "0102030405060708", sb.Short())

	require.Equal(t, "Genesis-block 010203040506070809 with roster []", sb.Sprint(false))
	require.Equal(t, "Genesis-block 01020304 with roster []", sb.Sprint(true))
}

func TestBlockLink_Copy(t *testing.T) {
	// Test if copy is deep or only shallow
	b1 := &libsc.BlockLink{}
	b1.Signature = []byte{1}
	b2 := b1.Copy()
	b2.Signature = []byte{2}
	if bytes.Equal(b1.Signature, b2.Signature) {
		t.Fatal("They should not be equal")
	}

	sb1 := libsc.NewSkipBlock()
	sb2 := sb1.Copy()
	sb1.Height = 10
	sb2.Height = 20
	if sb1.Height == sb2.Height {
		t.Fatal("Should not be equal")
	}
}

func TestSign(t *testing.T) {
	l := onet.NewTCPTest()
	servers, roster, _ := l.GenTree(10, true)
	msg := sha512.New().Sum(nil)
	sig, err := sign(msg, servers, l)
	log.ErrFatal(err)
	log.ErrFatal(sig.Verify(network.Suite, roster.Publics()))
	sig.Msg = sha512.New().Sum([]byte{1})
	require.NotNil(t, sig.Verify(network.Suite, roster.Publics()))
	defer l.CloseAll()
}

func TestSkipBlockID_Short(t *testing.T) {
	var sbid libsc.SkipBlockID
	require.Equal(t, "Nil", sbid.Short())

	sbid = libsc.SkipBlockID{1, 2, 3, 4, 5, 6, 7, 8, 9, 0xa, 0xb}
	require.Equal(t, "0102030405060708", sbid.Short())
}

func sign(msg libsc.SkipBlockID, servers []*onet.Server, l *onet.LocalTest) (*bftcosi.BFTSignature, error) {
	aggScalar := network.Suite.Scalar().Zero()
	aggPoint := network.Suite.Point().Null()
	for _, s := range servers {
		aggScalar.Add(aggScalar, l.GetPrivate(s))
		aggPoint.Add(aggPoint, s.ServerIdentity.Public)
	}
	rand := network.Suite.Scalar().Pick(random.Stream)
	comm := network.Suite.Point().Mul(nil, rand)
	sigC, err := comm.MarshalBinary()
	if err != nil {
		return nil, err
	}
	hash := sha512.New()
	hash.Write(sigC)
	aggPoint.MarshalTo(hash)
	hash.Write(msg)
	challBuff := hash.Sum(nil)
	chall := network.Suite.Scalar().SetBytes(challBuff)
	resp := network.Suite.Scalar().Mul(aggScalar, chall)
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
