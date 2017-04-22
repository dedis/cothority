package skipchain

import (
	"testing"

	"crypto/sha512"

	"github.com/dedis/cothority/bftcosi"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/dedis/crypto.v0/random"
	"gopkg.in/dedis/onet.v1"
)

func TestSkipBlock_GetResponsible(t *testing.T) {
	l := onet.NewTCPTest()
	_, roster, _ := l.GenTree(3, true)
	defer l.CloseAll()
	sbm := NewSkipBlockMap()
	root0 := NewSkipBlock()
	root0.Roster = roster
	root0.Hash = root0.calculateHash()
	sbm.StoreSkipBlock(root0)
	root1 := root0.Copy()
	root1.Index++
	root1.BackLinkIds = []SkipBlockID{root0.Hash}
	sbm.StoreSkipBlock(root1)
	inter0 := NewSkipBlock()
	inter0.ParentBlockID = root1.Hash
	inter0.Roster = roster
	inter0.Hash = inter0.calculateHash()
	sbm.StoreSkipBlock(inter0)
	inter1 := inter0.Copy()
	inter1.Index++
	inter1.BackLinkIds = []SkipBlockID{inter0.Hash}
	sbm.StoreSkipBlock(inter1)
	data0 := NewSkipBlock()
	data0.ParentBlockID = inter0.Hash
	sbm.StoreSkipBlock(data0)
	data1 := data0.Copy()
	data1.Index++
	data1.BackLinkIds = []SkipBlockID{data0.Hash}
	sbm.StoreSkipBlock(data1)

	b, err := root0.GetResponsible(sbm)
	log.ErrFatal(err)
	assert.True(t, root0.Equal(b))

	b, err = root1.GetResponsible(sbm)
	log.ErrFatal(err)
	assert.True(t, root0.Equal(b))

	b, err = inter0.GetResponsible(sbm)
	log.ErrFatal(err)
	assert.True(t, root1.Equal(b))

	b, err = inter1.GetResponsible(sbm)
	log.ErrFatal(err)
	assert.True(t, inter0.Equal(b))

	b, err = data0.GetResponsible(sbm)
	log.ErrFatal(err)
	assert.True(t, inter0.Equal(b))

	b, err = data1.GetResponsible(sbm)
	log.ErrFatal(err)
	assert.True(t, inter0.Equal(b))

}

func TestSkipBlock_VerifySignatures(t *testing.T) {
	l := onet.NewTCPTest()
	_, roster3, _ := l.GenTree(3, true)
	defer l.CloseAll()
	roster2 := onet.NewRoster(roster3.List[0:2])
	sbm := NewSkipBlockMap()
	root := NewSkipBlock()
	root.Roster = roster2
	root.BackLinkIds = append(root.BackLinkIds, SkipBlockID{1, 2, 3, 4})
	root.Hash = root.calculateHash()
	sbm.StoreSkipBlock(root)
	log.ErrFatal(root.VerifyForwardSignatures())
	log.ErrFatal(root.VerifyLinks(sbm))

	block1 := root.Copy()
	block1.BackLinkIds = append(block1.BackLinkIds, root.Hash)
	block1.Index++
	sbm.StoreSkipBlock(block1)
	require.Nil(t, block1.VerifyForwardSignatures())
	require.NotNil(t, block1.VerifyLinks(sbm))
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

func sign(msg SkipBlockID, servers []*onet.Server, l *onet.LocalTest) (*bftcosi.BFTSignature, error) {
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
	return &bftcosi.BFTSignature{sig, msg, nil}, nil
}
