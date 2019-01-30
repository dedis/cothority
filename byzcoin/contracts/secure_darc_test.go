package contracts

import (
	"testing"
	"time"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"

	"github.com/stretchr/testify/require"
)

func TestSecureDarc(t *testing.T) {
	local := onet.NewTCPTest(cothority.Suite)
	defer local.CloseAll()

	signer := darc.NewSignerEd25519(nil, nil)
	_, roster, _ := local.GenTree(3, true)

	genesisMsg, err := byzcoin.DefaultGenesisMsg(byzcoin.CurrentVersion, roster,
		[]string{"spawn:value", "spawn:darc", "spawn:secure_darc"}, signer.Identity())
	require.Nil(t, err)
	gDarc := &genesisMsg.GenesisDarc
	genesisMsg.BlockInterval = time.Second
	cl, _, err := byzcoin.NewLedger(genesisMsg, false)
	require.Nil(t, err)

	restrictedSigner := darc.NewSignerEd25519(nil, nil)
	unrestrictedSigner := darc.NewSignerEd25519(nil, nil)

	log.Info("spawn a new secure darc with spawn:darc - fail")
	secDarc := gDarc.Copy()
	secDarcBuf, err := secDarc.ToProto()
	require.NoError(t, err)
	ctx := byzcoin.ClientTransaction{
		Instructions: []byzcoin.Instruction{{
			InstanceID: byzcoin.NewInstanceID(gDarc.GetBaseID()),
			Spawn: &byzcoin.Spawn{
				ContractID: ContractSecureDarcID,
				Args: []byzcoin.Argument{{
					Name:  "darc",
					Value: secDarcBuf,
				}},
			},
			SignerCounter: []uint64{1},
		}},
	}
	require.Nil(t, ctx.SignWith(signer))
	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.Error(t, err)

	log.Info("do the same but without spawn:darc - pass")
	require.NoError(t, secDarc.Rules.DeleteRules("spawn:darc"))
	require.NoError(t, secDarc.Rules.AddRule("invoke:secure_darc."+cmdDarcEvolve, []byte(restrictedSigner.Identity().String())))
	require.NoError(t, secDarc.Rules.AddRule("invoke:secure_darc."+cmdDarcEvolveUnrestriction, []byte(unrestrictedSigner.Identity().String())))
	secDarcBuf, err = secDarc.ToProto()
	require.NoError(t, err)
	ctx = byzcoin.ClientTransaction{
		Instructions: []byzcoin.Instruction{{
			InstanceID: byzcoin.NewInstanceID(gDarc.GetBaseID()),
			Spawn: &byzcoin.Spawn{
				ContractID: ContractSecureDarcID,
				Args: []byzcoin.Argument{{
					Name:  "darc",
					Value: secDarcBuf,
				}},
			},
			SignerCounter: []uint64{1},
		}},
	}
	require.Nil(t, ctx.SignWith(signer))
	_, err = cl.AddTransactionAndWait(ctx, 10)
	require.NoError(t, err)

	log.Info("evolve to add rules - fail")
	{
		secDarc2 := secDarc.Copy()
		require.NoError(t, secDarc2.EvolveFrom(secDarc))
		secDarc2.Rules.AddRule("spawn:coin", secDarc.Rules.Get("invoke:secure_darc."+cmdDarcEvolveUnrestriction))
		secDarc2Buf, err := secDarc2.ToProto()
		ctx2 := byzcoin.ClientTransaction{
			Instructions: []byzcoin.Instruction{{
				InstanceID: byzcoin.NewInstanceID(secDarc.GetBaseID()),
				Invoke: &byzcoin.Invoke{
					ContractID: ContractSecureDarcID,
					Command:    cmdDarcEvolve,
					Args: []byzcoin.Argument{{
						Name:  "darc",
						Value: secDarc2Buf,
					}},
				},
				SignerCounter: []uint64{1},
			}},
		}
		require.Nil(t, ctx2.SignWith(restrictedSigner))
		_, err = cl.AddTransactionAndWait(ctx2, 10)
		require.Error(t, err)
	}

	log.Info("evolve to modify existing rules - pass")
	{
		secDarc2 := secDarc.Copy()
		require.NoError(t, secDarc2.EvolveFrom(secDarc))
		secDarc2Buf, err := secDarc2.ToProto()
		ctx2 := byzcoin.ClientTransaction{
			Instructions: []byzcoin.Instruction{{
				InstanceID: byzcoin.NewInstanceID(secDarc.GetBaseID()),
				Invoke: &byzcoin.Invoke{
					ContractID: ContractSecureDarcID,
					Command:    cmdDarcEvolve,
					Args: []byzcoin.Argument{{
						Name:  "darc",
						Value: secDarc2Buf,
					}},
				},
				SignerCounter: []uint64{1},
			}},
		}
		require.Nil(t, ctx2.SignWith(restrictedSigner))
		_, err = cl.AddTransactionAndWait(ctx2, 10)
		require.NoError(t, err)
	}

	// get the latest darc
	resp, err := cl.GetProof(secDarc.GetBaseID())
	require.NoError(t, err)
	myDarc := darc.Darc{}
	require.NoError(t, resp.Proof.VerifyAndDecode(cothority.Suite, ContractSecureDarcID, &myDarc))
	// secDarc is copied from genesis DARC, after one evolution the version
	// should increase by one
	require.Equal(t, myDarc.Version, gDarc.Version+1)

	log.Info("evolve_unrestricted fails with the wrong signer")
	{
		myDarc2 := myDarc.Copy()
		require.NoError(t, myDarc2.EvolveFrom(&myDarc))
		myDarc2.Rules.AddRule("spawn:coin", myDarc.Rules.Get("invoke:secure_darc."+cmdDarcEvolveUnrestriction))
		myDarc2Buf, err := myDarc2.ToProto()
		ctx2 := byzcoin.ClientTransaction{
			Instructions: []byzcoin.Instruction{{
				InstanceID: byzcoin.NewInstanceID(myDarc.GetBaseID()),
				Invoke: &byzcoin.Invoke{
					ContractID: ContractSecureDarcID,
					Command:    cmdDarcEvolveUnrestriction,
					Args: []byzcoin.Argument{{
						Name:  "darc",
						Value: myDarc2Buf,
					}},
				},
				SignerCounter: []uint64{1},
			}},
		}
		require.Nil(t, ctx2.SignWith(restrictedSigner)) // here we use the wrong signer
		_, err = cl.AddTransactionAndWait(ctx2, 10)
		require.Error(t, err)
	}

	log.Info("evolve_unrestricted to add rules - pass")
	{
		myDarc2 := myDarc.Copy()
		require.NoError(t, myDarc2.EvolveFrom(&myDarc))
		myDarc2.Rules.AddRule("spawn:coin", myDarc2.Rules.Get("invoke:secure_darc."+cmdDarcEvolveUnrestriction))
		myDarc2Buf, err := myDarc2.ToProto()
		ctx2 := byzcoin.ClientTransaction{
			Instructions: []byzcoin.Instruction{{
				InstanceID: byzcoin.NewInstanceID(myDarc.GetBaseID()),
				Invoke: &byzcoin.Invoke{
					ContractID: ContractSecureDarcID,
					Command:    cmdDarcEvolveUnrestriction,
					Args: []byzcoin.Argument{{
						Name:  "darc",
						Value: myDarc2Buf,
					}},
				},
				SignerCounter: []uint64{1},
			}},
		}
		require.Nil(t, ctx2.SignWith(unrestrictedSigner)) // here we use the correct signer
		_, err = cl.AddTransactionAndWait(ctx2, 10)
		require.NoError(t, err)
	}

	// try to get the DARC again and it should have the "spawn:coin" rule
	{
		resp, err := cl.GetProof(secDarc.GetBaseID())
		require.NoError(t, err)
		myDarc := darc.Darc{}
		require.NoError(t, resp.Proof.VerifyAndDecode(cothority.Suite, ContractSecureDarcID, &myDarc))
		require.Equal(t, myDarc.Rules.Get("spawn:coin"), myDarc.Rules.Get("invoke:secure_darc."+cmdDarcEvolveUnrestriction))
	}

	local.WaitDone(genesisMsg.BlockInterval)
}
