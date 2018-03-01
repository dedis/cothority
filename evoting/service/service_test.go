package service

import (
	"testing"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/evoting"
	"github.com/dedis/cothority/evoting/lib"
	"github.com/dedis/kyber/util/key"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
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
	pin := "1234"
	s.pin = pin
	// token := s.state.register(0, false)

	// Creating master skipchain
	replyLink, err := s.Link(&evoting.Link{
		Pin:    pin,
		Roster: roster,
		Key:    nodeKP.Public,
		Admins: []uint32{idAdmin},
	})
	require.Nil(t, err)

	login := func(user uint32) *evoting.LoginReply {
		login := &evoting.Login{
			ID:   replyLink.ID,
			User: user,
		}
		require.Nil(t, login.Sign(nodeKP.Private))
		replyLogin, err := s.Login(login)
		require.Nil(t, err)
		return replyLogin
	}

	// Logging in as administrator
	loginAdmin := login(idAdmin)

	// Create a new election
	replyOpen, err := s.Open(&evoting.Open{
		Token: loginAdmin.Token,
		ID:    replyLink.ID,
		Election: &lib.Election{
			Name:        "bla",
			Creator:     idAdmin,
			Users:       []uint32{idUser1, idUser2, idUser3, idAdmin},
			Data:        append(bufCand1, bufCand2...),
			Description: "test",
			End:         "1/1/1970",
		},
	})
	require.Nil(t, err)

	vote := func(user uint32, bufCand []byte) *evoting.CastReply {
		loginUser := login(user)
		k, c := lib.Encrypt(replyOpen.Key, bufCand)
		log.Lvl2(cothority.Suite.Point().Sub(c, replyOpen.Key))
		ballot := &lib.Ballot{
			User:  user,
			Alpha: k,
			Beta:  c,
		}
		cast, err := s.Cast(&evoting.Cast{
			Token:  loginUser.Token,
			ID:     replyOpen.ID,
			Ballot: ballot,
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
		Token: loginAdmin.Token,
		ID:    replyOpen.ID,
	})
	require.Nil(t, err)

	// Decrypt all votes
	_, err = s.Decrypt(&evoting.Decrypt{
		Token: loginAdmin.Token,
		ID:    replyOpen.ID,
	})
	require.Nil(t, err)

	// Reconstruct votes
	reconstructReply, err := s.Reconstruct(&evoting.Reconstruct{
		Token: loginAdmin.Token,
		ID:    replyOpen.ID,
	})
	require.Nil(t, err)
	for _, p := range reconstructReply.Points {
		log.Lvl2("Point is:", p.String())
	}
}
