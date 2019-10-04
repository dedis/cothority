package byzcoin

import (
	"testing"
	"time"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"

	"github.com/stretchr/testify/require"
)

func TestSecureDarc(t *testing.T) {
	local := onet.NewTCPTest(cothority.Suite)
	defer local.CloseAll()

	signer := darc.NewSignerEd25519(nil, nil)
	_, roster, _ := local.GenTree(3, true)

	genesisMsg, err := DefaultGenesisMsg(CurrentVersion, roster, []string{}, signer.Identity())
	require.Nil(t, err)
	gDarc := &genesisMsg.GenesisDarc
	genesisMsg.BlockInterval = time.Second
	cl, _, err := NewLedger(genesisMsg, false)
	require.Nil(t, err)

	restrictedSigner := darc.NewSignerEd25519(nil, nil)
	unrestrictedSigner := darc.NewSignerEd25519(nil, nil)
	invokeEvolve := darc.Action("invoke:" + ContractDarcID + "." + cmdDarcEvolve)
	invokeEvolveUnrestricted := darc.Action("invoke:" + ContractDarcID + "." + cmdDarcEvolveUnrestriction)

	log.Info("spawn a new secure darc with spawn:insecure_darc - fail")
	secDarc := gDarc.Copy()
	require.NoError(t, secDarc.Rules.AddRule("spawn:insecure_darc", []byte(restrictedSigner.Identity().String())))
	secDarcBuf, err := secDarc.ToProto()
	require.NoError(t, err)
	ctx, err := cl.CreateTransaction(Instruction{
		InstanceID: NewInstanceID(gDarc.GetBaseID()),
		Spawn: &Spawn{
			ContractID: ContractDarcID,
			Args: []Argument{{
				Name:  "darc",
				Value: secDarcBuf,
			}},
		},
		SignerCounter: []uint64{1},
	})
	require.NoError(t, err)
	require.NoError(t, ctx.FillSignersAndSignWith(signer))
	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.Error(t, err)

	log.Info("do the same but without spawn:insecure_darc - pass")
	require.NoError(t, secDarc.Rules.DeleteRules("spawn:insecure_darc"))
	require.NoError(t, secDarc.Rules.UpdateRule(invokeEvolve, []byte(restrictedSigner.Identity().String())))
	require.NoError(t, secDarc.Rules.UpdateRule(invokeEvolveUnrestricted, []byte(unrestrictedSigner.Identity().String())))
	secDarcBuf, err = secDarc.ToProto()
	require.NoError(t, err)
	ctx, err = cl.CreateTransaction(Instruction{
		InstanceID: NewInstanceID(gDarc.GetBaseID()),
		Spawn: &Spawn{
			ContractID: ContractDarcID,
			Args: []Argument{{
				Name:  "darc",
				Value: secDarcBuf,
			}},
		},
		SignerCounter: []uint64{1},
	})
	require.NoError(t, err)
	require.NoError(t, ctx.FillSignersAndSignWith(signer))
	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.NoError(t, err)

	log.Info("spawn a darc with a version > 0 - fail")
	secDarc.Version = 1
	secDarcBuf, err = secDarc.ToProto()
	ctx, err = cl.CreateTransaction(Instruction{
		InstanceID: NewInstanceID(gDarc.GetBaseID()),
		Spawn: &Spawn{
			ContractID: ContractDarcID,
			Args: []Argument{{
				Name:  "darc",
				Value: secDarcBuf,
			}},
		},
		SignerCounter: []uint64{2},
	})
	require.NoError(t, err)
	require.NoError(t, ctx.FillSignersAndSignWith(signer))
	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.Error(t, err)

	secDarc.Version = 0
	log.Info("evolve to add rules - fail")
	{
		secDarc2 := secDarc.Copy()
		require.NoError(t, secDarc2.EvolveFrom(secDarc))
		require.NoError(t, secDarc2.Rules.AddRule("spawn:coin", secDarc.Rules.Get(invokeEvolveUnrestricted)))
		secDarc2Buf, err := secDarc2.ToProto()
		ctx2, err := cl.CreateTransaction(Instruction{
			InstanceID: NewInstanceID(secDarc.GetBaseID()),
			Invoke: &Invoke{
				ContractID: ContractDarcID,
				Command:    cmdDarcEvolve,
				Args: []Argument{{
					Name:  "darc",
					Value: secDarc2Buf,
				}},
			},
			SignerCounter: []uint64{1},
		})
		require.NoError(t, err)
		require.NoError(t, ctx2.FillSignersAndSignWith(restrictedSigner))
		_, err = cl.AddTransactionAndWait(ctx2, 10)
		require.Error(t, err)
	}

	log.Info("evolve to modify the unrestrict_evolve rule - fail")
	{
		secDarc2 := secDarc.Copy()
		require.NoError(t, secDarc2.EvolveFrom(secDarc))
		// changing the signer to something else, then it should fail
		require.NoError(t, secDarc2.Rules.UpdateRule(invokeEvolveUnrestricted, []byte(restrictedSigner.Identity().String())))
		secDarc2Buf, err := secDarc2.ToProto()
		ctx2, err := cl.CreateTransaction(Instruction{
			InstanceID: NewInstanceID(secDarc.GetBaseID()),
			Invoke: &Invoke{
				ContractID: ContractDarcID,
				Command:    cmdDarcEvolve,
				Args: []Argument{{
					Name:  "darc",
					Value: secDarc2Buf,
				}},
			},
			SignerCounter: []uint64{1},
		})
		require.NoError(t, err)
		require.NoError(t, ctx2.FillSignersAndSignWith(restrictedSigner))
		_, err = cl.AddTransactionAndWait(ctx2, 10)
		require.Error(t, err)
	}

	var barrier *skipchain.SkipBlock

	log.Info("evolve to modify existing rules - pass")
	{
		secDarc2 := secDarc.Copy()
		require.NoError(t, secDarc2.EvolveFrom(secDarc))
		secDarc2Buf, err := secDarc2.ToProto()
		ctx2, err := cl.CreateTransaction(Instruction{
			InstanceID: NewInstanceID(secDarc.GetBaseID()),
			Invoke: &Invoke{
				ContractID: ContractDarcID,
				Command:    cmdDarcEvolve,
				Args: []Argument{{
					Name:  "darc",
					Value: secDarc2Buf,
				}},
			},
			SignerCounter: []uint64{1},
		})
		require.NoError(t, err)
		require.NoError(t, ctx2.FillSignersAndSignWith(restrictedSigner))
		atr, err := cl.AddTransactionAndWait(ctx2, 10)
		require.NoError(t, err)

		barrier = &atr.Proof.Latest
	}

	// get the latest darc
	resp, err := cl.GetProofAfter(secDarc.GetBaseID(), false, barrier)
	require.NoError(t, err)
	myDarc := darc.Darc{}
	require.NoError(t, resp.Proof.VerifyAndDecode(cothority.Suite, ContractDarcID, &myDarc))
	// secDarc is copied from genesis DARC, after one evolution the version
	// should increase by one
	require.Equal(t, myDarc.Version, gDarc.Version+1)

	log.Info("evolve_unrestricted fails with the wrong signer")
	{
		myDarc2 := myDarc.Copy()
		require.NoError(t, myDarc2.EvolveFrom(&myDarc))
		require.NoError(t, myDarc2.Rules.AddRule("spawn:coin", myDarc.Rules.Get(invokeEvolveUnrestricted)))
		myDarc2Buf, err := myDarc2.ToProto()
		ctx2, err := cl.CreateTransaction(Instruction{
			InstanceID: NewInstanceID(myDarc.GetBaseID()),
			Invoke: &Invoke{
				ContractID: ContractDarcID,
				Command:    cmdDarcEvolveUnrestriction,
				Args: []Argument{{
					Name:  "darc",
					Value: myDarc2Buf,
				}},
			},
			SignerCounter: []uint64{1},
		})
		require.NoError(t, err)
		require.NoError(t, ctx2.FillSignersAndSignWith(restrictedSigner)) // here we use the wrong signer
		_, err = cl.AddTransactionAndWait(ctx2, 10)
		require.Error(t, err)
	}

	log.Info("evolve_unrestricted to add rules - pass")
	{
		myDarc2 := myDarc.Copy()
		require.NoError(t, myDarc2.EvolveFrom(&myDarc))
		require.NoError(t, myDarc2.Rules.AddRule("spawn:coin", myDarc2.Rules.Get(invokeEvolveUnrestricted)))
		myDarc2Buf, err := myDarc2.ToProto()
		ctx2, err := cl.CreateTransaction(Instruction{
			InstanceID: NewInstanceID(myDarc.GetBaseID()),
			Invoke: &Invoke{
				ContractID: ContractDarcID,
				Command:    cmdDarcEvolveUnrestriction,
				Args: []Argument{{
					Name:  "darc",
					Value: myDarc2Buf,
				}},
			},
			SignerCounter: []uint64{1},
		})
		require.NoError(t, err)
		require.NoError(t, ctx2.FillSignersAndSignWith(unrestrictedSigner)) // here we use the correct signer
		atr, err := cl.AddTransactionAndWait(ctx2, 10)
		require.NoError(t, err)

		barrier = &atr.Proof.Latest
	}

	// try to get the DARC again and it should have the "spawn:coin" rule
	{
		resp, err := cl.GetProofAfter(secDarc.GetBaseID(), false, barrier)
		require.NoError(t, err)
		myDarc := darc.Darc{}
		require.NoError(t, resp.Proof.VerifyAndDecode(cothority.Suite, ContractDarcID, &myDarc))
		require.Equal(t, myDarc.Rules.Get("spawn:coin"), myDarc.Rules.Get("invoke:darc."+cmdDarcEvolveUnrestriction))
	}

	require.NoError(t, local.WaitDone(5*genesisMsg.BlockInterval))
}
