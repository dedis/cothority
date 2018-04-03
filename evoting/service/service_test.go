package service

import (
	"strconv"
	"testing"
	"time"

	"github.com/dedis/cothority"
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/sign/schnorr"
	"github.com/dedis/kyber/util/key"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"

	"github.com/stretchr/testify/require"

	"github.com/dedis/cothority/evoting"
	"github.com/dedis/cothority/evoting/lib"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func generateSignature(private kyber.Scalar, ID []byte, sciper uint32) ([]byte, error) {
	message := ID
	for _, c := range strconv.Itoa(int(sciper)) {
		d, _ := strconv.Atoi(string(c))
		message = append(message, byte(d))
	}
	return schnorr.Sign(cothority.Suite, private, message)
}

func TestService(t *testing.T) {
	local := onet.NewLocalTest(cothority.Suite)
	defer local.CloseAll()

	idAdmin := uint32(111111)
	idUser1 := uint32(111112)
	idUser2 := uint32(111113)
	idUser3 := uint32(111114)
	idCand1 := uint32(123456)
	bufCand1 := []byte{byte(idCand1 & 0xff), byte((idCand1 >> 8) & 0xff), byte((idCand1 >> 16) & 0xff)}
	idCand2 := uint32(123457)
	bufCand2 := []byte{byte(idCand2 & 0xff), byte((idCand2 >> 8) & 0xff), byte((idCand2 >> 16) & 0xff)}

	nodeKP := key.NewKeyPair(cothority.Suite)

	nodes, roster, _ := local.GenBigTree(3, 3, 1, true)
	s := local.GetServices(nodes, serviceID)[0].(*Service)

	// Creating master skipchain
	replyLink, err := s.Link(&evoting.Link{
		Pin:    s.pin,
		Roster: roster,
		Key:    nodeKP.Public,
		Admins: []uint32{idAdmin},
	})
	require.Nil(t, err)

	signature, err := generateSignature(nodeKP.Private, replyLink.ID, idAdmin)
	require.Nil(t, err)

	// Create a new election
	replyOpen, err := s.Open(&evoting.Open{
		ID: replyLink.ID,
		Election: &lib.Election{
			Creator: idAdmin,
			Users:   []uint32{idUser1, idUser2, idUser3, idAdmin},
			Roster:  roster,
			End:     time.Now().Unix() + 86400,
		},
		User:      idAdmin,
		Signature: signature,
	})
	require.Nil(t, err)

	vote := func(user uint32, bufCand []byte) *evoting.CastReply {
		k, c := lib.Encrypt(replyOpen.Key, bufCand)
		sig, err := generateSignature(nodeKP.Private, replyLink.ID, user)
		require.Nil(t, err)
		ballot := &lib.Ballot{
			User:  user,
			Alpha: k,
			Beta:  c,
		}
		cast, err := s.Cast(&evoting.Cast{
			ID:        replyOpen.ID,
			Ballot:    ballot,
			User:      user,
			Signature: sig,
		})
		require.Nil(t, err)
		return cast
	}

	// User votes
	vote(idUser1, bufCand1)
	vote(idUser2, bufCand1)
	vote(idUser3, bufCand2)

	// Shuffle all votes
	_, err = s.Shuffle(&evoting.Shuffle{
		ID:        replyOpen.ID,
		User:      idAdmin,
		Signature: signature,
	})
	require.Nil(t, err)

	// Decrypt all votes
	_, err = s.Decrypt(&evoting.Decrypt{
		ID:        replyOpen.ID,
		User:      idAdmin,
		Signature: signature,
	})
	require.Nil(t, err)

	// Reconstruct votes
	reconstructReply, err := s.Reconstruct(&evoting.Reconstruct{
		ID: replyOpen.ID,
	})
	require.Nil(t, err)

	for _, p := range reconstructReply.Points {
		log.Lvl2("Point is:", p.String())
	}
}
