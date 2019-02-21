package personhood

import (
	"testing"
	"time"

	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/byzcoin/contracts"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/cothority/v3/darc/expression"

	"go.dedis.ch/kyber/v3/util/key"
	"go.dedis.ch/onet/v3/log"

	"github.com/stretchr/testify/require"
)

// Creates a party, activates the barrier point, finalizes it, and mines the coins.
func TestContractPopParty(t *testing.T) {
	s := newS(t)
	defer s.Close()
	s.createParty(t, len(s.servers), 3)
}

// Create a party with orgs organizers and attendees. It will store the party
// in the ledger and finalize it.
func (s *sStruct) createParty(t *testing.T, orgs, attendees int) {
	for i := 0; i < orgs; i++ {
		org := key.NewKeyPair(tSuite)
		s.orgs = append(s.orgs, org)
	}
	s.party = FinalStatement{
		Desc: &PopDesc{
			Name:     "test-party",
			DateTime: uint64(time.Now().Unix()),
			Location: "BC208",
			Purpose:  "test",
		},
	}
	s.service = key.NewKeyPair(tSuite)
	s.attendees = append(s.attendees, s.service)
	s.party.Attendees.Keys = append(s.party.Attendees.Keys, s.service.Public)
	for i := 0; i < attendees; i++ {
		kp := key.NewKeyPair(tSuite)
		s.attendees = append(s.attendees, kp)
		s.party.Attendees.Keys = append(s.party.Attendees.Keys, kp.Public)
	}

	// Store the party in the ledger
	log.Lvl2("Publishing the party to the ledger")

	var err error
	s.popI, err = PopPartySpawn(s.cl, s.party, s.genesisDarc.GetBaseID(), 1e6, s.signer)
	require.Nil(t, err)
	// Activate the barrier point
	log.Lvl2("activating barrier point")

	err = PopPartyBarrier(s.cl, s.popI, s.signer)
	require.Nil(t, err)

	// Store the finalized party in the ledger
	log.Lvl2("finalizing the party in the ledger")

	err = PopPartyFinalize(s.cl, s.popI, s.party.Attendees, s.signer)
	require.Nil(t, err)

	// Mine all coins
	s.attCoin = make([]byzcoin.InstanceID, len(s.attendees))
	s.attDarc = make([]*darc.Darc, len(s.attendees))
	s.attSig = make([]darc.Signer, len(s.attendees))
	for i, att := range s.attendees {
		s.attSig[i] = darc.NewSignerEd25519(nil, nil)
		id := s.attSig[i].Identity()
		rules := darc.InitRules([]darc.Identity{id}, []darc.Identity{id})
		rules.AddRule(darc.Action("invoke:"+contracts.ContractCoinID+".transfer"), expression.Expr(id.String()))
		s.attDarc[i] = darc.NewDarc(rules, []byte("Attendee darc for pop-party"))
		err = PopPartyMine(s.cl, s.popI, *att, nil, nil, s.attDarc[i])
		require.Nil(t, err)

		var coin byzcoin.Coin
		s.attCoin[i], coin, err = PopPartyMineDarcToCoin(s.cl, s.attDarc[i])
		require.Nil(t, err)
		require.NotNil(t, coin)
		require.Equal(t, uint64(1e6), coin.Value)
	}
	s.serDarc = s.attDarc[0]
	s.attDarc = s.attDarc[1:]
	s.serCoin = s.attCoin[0]
	s.attCoin = s.attCoin[1:]
	s.serSig = s.attSig[0]
	s.attSig = s.attSig[1:]

	_, err = s.phs[0].PartyList(&PartyList{
		NewParty: &Party{
			Roster:     *s.roster,
			ByzCoinID:  s.olID,
			InstanceID: s.popI,
			Signer:     s.serSig,
			CoinIID:    s.serCoin,
		},
	})
	require.Nil(t, err)
}
