package service

import (
	"testing"
	"time"

	"github.com/dedis/cothority/omniledger/darc"
	ol "github.com/dedis/cothority/omniledger/service"
	pop "github.com/dedis/cothority/pop/service"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/kyber"
	"github.com/dedis/kyber/sign/schnorr"
	"github.com/dedis/kyber/suites"
	"github.com/dedis/kyber/util/key"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/protobuf"
	"github.com/stretchr/testify/require"
)

var tSuite = suites.MustFind("Ed25519")

func TestMain(m *testing.M) {
	log.MainTest(m)
}

// Creates a party and links it, then verifies the account exists.
func TestService_LinkPoP(t *testing.T) {
	s := newS(t)
	defer s.Close()
	s.createParty(t, len(s.servers), 3)

	_, err := s.phs[0].LinkPoP(&LinkPoP{
		PopInstance: s.popI,
		Party: Party{
			OmniLedgerID:   s.olID,
			FinalStatement: s.party,
		},
	})
	require.Nil(t, err)
}

type sStruct struct {
	local     *onet.LocalTest
	servers   []*onet.Server
	roster    *onet.Roster
	services  []onet.Service
	phs       []*Service
	pops      []*pop.Service
	party     pop.FinalStatement
	orgs      []*key.Pair
	attendees []*key.Pair
	ols       *ol.Service
	olID      skipchain.SkipBlockID
	signer    darc.Signer
	gMsg      *ol.CreateGenesisBlock
	popI      ol.InstanceID
}

func newS(t *testing.T) (s *sStruct) {
	s = &sStruct{}
	s.local = onet.NewLocalTestT(tSuite, t)
	s.servers, s.roster, _ = s.local.GenTree(5, true)

	s.services = s.local.GetServices(s.servers, templateID)
	for _, p := range s.services {
		s.phs = append(s.phs, p.(*Service))
	}
	popsS := s.local.GetServices(s.servers, onet.ServiceFactory.ServiceID(pop.Name))
	for _, p := range popsS {
		s.pops = append(s.pops, p.(*pop.Service))
	}

	// Create OmniLedger
	s.ols = s.local.Services[s.roster.List[0].ID][onet.ServiceFactory.ServiceID(ol.ServiceName)].(*ol.Service)
	s.signer = darc.NewSignerEd25519(nil, nil)
	var err error
	s.gMsg, err = ol.DefaultGenesisMsg(ol.CurrentVersion, s.roster,
		[]string{"spawn:dummy", "spawn:popParty", "invoke:Finalize"}, s.signer.Identity())
	require.Nil(t, err)
	s.gMsg.BlockInterval = 500 * time.Millisecond

	resp, err := s.ols.CreateGenesisBlock(s.gMsg)
	require.Nil(t, err)
	s.olID = resp.Skipblock.SkipChainID()
	return
}

func (s *sStruct) Close() {
	s.local.CloseAll()
}

// Create a party with orgs organizers and attendees. It will store the party
// in omniLedger and finalize it.
func (s *sStruct) createParty(t *testing.T, orgs, attendees int) {
	if orgs > len(s.pops) {
		t.Fatal("cannot have more organizers than conodes")
	}
	for i := 0; i < orgs; i++ {
		org := key.NewKeyPair(tSuite)
		s.orgs = append(s.orgs, org)
		s.pops[i].StoreLink(org.Public)
	}
	for i := 0; i < attendees; i++ {
		s.attendees = append(s.attendees, key.NewKeyPair(tSuite))
	}
	s.party = pop.FinalStatement{
		Desc: &pop.PopDesc{
			Name:     "test-party",
			DateTime: "2018-08-28 08:08",
			Location: "BC208",
			Roster:   s.roster,
		},
	}

	// Publish the party
	log.Lvl2("Publishing the party to the pop-service")
	var atts []kyber.Point
	for _, att := range s.attendees {
		atts = append(atts, att.Public)
	}
	ph := s.party.Desc.Hash()
	for i, org := range s.orgs {
		sg, err := schnorr.Sign(tSuite, org.Private, ph)
		require.Nil(t, err)
		_, err = s.pops[i].StoreConfig(&pop.StoreConfig{
			Desc:      s.party.Desc,
			Signature: sg,
		})
		require.Nil(t, err)
	}

	// Store the party in OmniLedger
	s.createPoPSpawn(t)

	// Finalise the party
	log.Lvl2("Finalizing the party in the pop-service")
	for i, org := range s.orgs {
		req := &pop.FinalizeRequest{
			DescID:    ph,
			Attendees: atts,
		}
		reqH, err := req.Hash()
		require.Nil(t, err)
		req.Signature, err = schnorr.Sign(tSuite, org.Private, reqH)
		require.Nil(t, err)
		fr, err := s.pops[i].FinalizeRequest(req)
		if err != nil && i == len(s.orgs)-1 {
			t.Fatal("Shouldn't get error in last finalization-request: " + err.Error())
		} else if err == nil {
			s.party = *fr.Final
		}
	}

	// Store the finalized party in OmniLedger
	s.invokePoPFinalize(t)
}

func (s *sStruct) createPoPSpawn(t *testing.T) {
	log.Lvl2("Publishing the party to omniledger")

	fsBuf, err := protobuf.Encode(&s.party)
	require.Nil(t, err)
	dID := s.gMsg.GenesisDarc.GetBaseID()
	ctx := ol.ClientTransaction{
		Instructions: ol.Instructions{ol.Instruction{
			InstanceID: ol.NewInstanceID(dID),
			Index:      0,
			Length:     1,
			Spawn: &ol.Spawn{
				ContractID: pop.ContractPopParty,
				Args: ol.Arguments{{
					Name:  "FinalStatement",
					Value: fsBuf,
				}},
			},
		}},
	}
	err = ctx.Instructions[0].SignBy(dID, s.signer)
	require.Nil(t, err)
	_, err = s.ols.AddTransaction(&ol.AddTxRequest{
		Version:       ol.CurrentVersion,
		SkipchainID:   s.olID,
		Transaction:   ctx,
		InclusionWait: 10,
	})
	require.Nil(t, err)
	s.popI = ctx.Instructions[0].DeriveID("")
}

func (s *sStruct) invokePoPFinalize(t *testing.T) {
	log.Lvl2("finalizing the party in omniledger")
	fsBuf, err := protobuf.Encode(&s.party)
	require.Nil(t, err)
	ctx := ol.ClientTransaction{
		Instructions: ol.Instructions{ol.Instruction{
			InstanceID: s.popI,
			Index:      0,
			Length:     1,
			Invoke: &ol.Invoke{
				Command: "Finalize",
				Args: ol.Arguments{{
					Name:  "FinalStatement",
					Value: fsBuf,
				}},
			},
		}},
	}
	dID := s.gMsg.GenesisDarc.GetBaseID()
	err = ctx.Instructions[0].SignBy(dID, s.signer)
	require.Nil(t, err)
	_, err = s.ols.AddTransaction(&ol.AddTxRequest{
		Version:       ol.CurrentVersion,
		SkipchainID:   s.olID,
		Transaction:   ctx,
		InclusionWait: 10,
	})
	require.Nil(t, err)
}
