package personhood

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/cothority/v3/personhood/contracts"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/kyber/v3/sign/anon"
	"go.dedis.ch/kyber/v3/util/key"
	"go.dedis.ch/onet/v3/network"
	"go.dedis.ch/protobuf"
)

// PopPartySpawn returns the instanceID of the newly created pop-party, or an error if it
// wasn't successful.
func PopPartySpawn(cl *byzcoin.Client, desc contracts.PopDesc, dID darc.ID,
	reward uint64, signers ...darc.Signer) (popIID byzcoin.InstanceID, err error) {
	var sigStrs []string
	for _, sig := range signers {
		sigStrs = append(sigStrs, sig.Identity().String())
	}
	signerCtrs, err := cl.GetSignerCounters(sigStrs...)
	if err != nil {
		return
	}

	descBuf, err := protobuf.Encode(&desc)
	if err != nil {
		return
	}
	mr := make([]byte, 8)
	binary.LittleEndian.PutUint64(mr, reward)
	ctx := byzcoin.NewClientTransaction(byzcoin.CurrentVersion,
		byzcoin.Instruction{
			InstanceID: byzcoin.NewInstanceID(dID),
			Spawn: &byzcoin.Spawn{
				ContractID: contracts.ContractPopPartyID,
				Args: byzcoin.Arguments{{
					Name:  "description",
					Value: descBuf,
				}, {
					Name:  "darcID",
					Value: dID,
				}, {
					Name:  "miningReward",
					Value: mr,
				}},
			},
			SignerCounter: []uint64{signerCtrs.Counters[0] + 1},
		},
	)
	err = ctx.FillSignersAndSignWith(signers...)
	if err != nil {
		return
	}
	_, err = cl.AddTransactionAndWait(ctx, 5)
	if err != nil {
		return
	}
	return ctx.Instructions[0].DeriveID(""), nil
}

// PopPartyBarrier activates the barrier in the pop-party.
func PopPartyBarrier(cl *byzcoin.Client, popIID byzcoin.InstanceID, signers ...darc.Signer) error {
	var sigStrs []string
	for _, sig := range signers {
		sigStrs = append(sigStrs, sig.Identity().String())
	}
	signerCtrs, err := cl.GetSignerCounters(sigStrs...)
	if err != nil {
		return err
	}

	ctx, err := cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: popIID,
		Invoke: &byzcoin.Invoke{
			ContractID: contracts.ContractPopPartyID,
			Command:    "barrier",
		},
		SignerCounter: []uint64{signerCtrs.Counters[0] + 1},
	})
	if err != nil {
		return err
	}
	err = ctx.FillSignersAndSignWith(signers...)
	if err != nil {
		return err
	}
	_, err = cl.AddTransactionAndWait(ctx, 5)
	return err
}

// PopPartyFinalized sends the list of attendees to the party for finalization.
func PopPartyFinalized(
	cl *byzcoin.Client,
	popIID byzcoin.InstanceID,
	atts contracts.Attendees,
	signers ...darc.Signer,
) error {
	_, err := PopPartyFinalizeDetailed(cl, popIID, atts, signers...)
	return err
}

// PopPartyFinalizeDetailed sends the list of attendees to the party for finalization.
func PopPartyFinalizeDetailed(
	cl *byzcoin.Client,
	popIID byzcoin.InstanceID,
	atts contracts.Attendees,
	signers ...darc.Signer,
) (*byzcoin.AddTxResponse, error) {
	var sigStrs []string
	for _, sig := range signers {
		sigStrs = append(sigStrs, sig.Identity().String())
	}
	signerCtrs, err := cl.GetSignerCounters(sigStrs...)
	if err != nil {
		return nil, err
	}

	attBuff, err := protobuf.Encode(&atts)
	if err != nil {
		return nil, err
	}
	ctx, err := cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: popIID,
		Invoke: &byzcoin.Invoke{
			ContractID: contracts.ContractPopPartyID,
			Command:    "finalize",
			Args: byzcoin.Arguments{
				{
					Name:  "attendees",
					Value: attBuff,
				},
			},
		},
		SignerCounter: []uint64{signerCtrs.Counters[0] + 1},
	})
	if err != nil {
		return nil, err
	}
	err = ctx.FillSignersAndSignWith(signers...)
	if err != nil {
		return nil, err
	}
	return cl.AddTransactionAndWait(ctx, 5)
}

// PopPartyMine is a method to be called by an outside client. It collects the reward for a given
// attendee of the party. For convenience, this can be called with some of the arguments being 'nil'.
//
//   - atts - the list of the public keys of the attendees. If it is nil, the party will be fetched from
//     byzcoin.
//   - coinIID - if set, 'd' must be nil. coinIID points to the coin InstanceID where the reward will be stored.
//   - d - if set, 'coinIID' must be nil. d is the darc that will be used to create a new coinInstance.
func PopPartyMine(
	cl *byzcoin.Client,
	popIID byzcoin.InstanceID,
	kp key.Pair,
	atts *contracts.Attendees,
	coinIID *byzcoin.InstanceID,
	d *darc.Darc,
) error {
	_, err := PopPartyMineDetailed(cl, popIID, kp, atts, coinIID, d, nil)
	return err
}

// PopPartyMineDetailed is a method to be called by an outside client. It collects the reward for a given
// attendee of the party. For convenience, this can be called with some of the arguments being 'nil'.
//
//   - atts - the list of the public keys of the attendees. If it is nil, the party will be fetched from
//     byzcoin.
//   - coinIID - if set, 'd' must be nil. coinIID points to the coin InstanceID where the reward will be stored.
//   - d - if set, 'coinIID' must be nil. d is the darc that will be used to create a new coinInstance.
func PopPartyMineDetailed(
	cl *byzcoin.Client,
	popIID byzcoin.InstanceID,
	kp key.Pair,
	atts *contracts.Attendees,
	coinIID *byzcoin.InstanceID,
	d *darc.Darc,
	barrier *skipchain.SkipBlock,
) (*byzcoin.AddTxResponse, error) {
	if (coinIID == nil && d == nil) ||
		(coinIID != nil && d != nil) {
		return nil, errors.New("either set coinIID or d, but not both")
	}
	if atts == nil {
		popProof, err := cl.GetProofAfter(popIID.Slice(), true, barrier)
		if err != nil {
			return nil, err
		}
		_, value, cID, _, err := popProof.Proof.KeyValue()
		if err != nil {
			return nil, err
		}
		if cID != contracts.ContractPopPartyID {
			return nil, errors.New(
				"given popIID is not of contract-type PopParty")
		}
		var pop contracts.PopPartyStruct
		err = protobuf.DecodeWithConstructors(value, &pop, network.DefaultConstructors(cothority.Suite))
		if err != nil {
			return nil, err
		}

		atts = &pop.Attendees
	}
	var mine = -1
	for i, p := range atts.Keys {
		if p.Equal(kp.Public) {
			mine = i
			break
		}
	}
	if mine == -1 {
		return nil, errors.New(
			"didn't find public key of keypair in attendees")
	}

	lrs := anon.Sign(&contracts.SuiteBlake2s{}, []byte("mine"), atts.Keys, popIID[:], mine, kp.Private)
	args := byzcoin.Arguments{{
		Name:  "lrs",
		Value: lrs,
	}}
	if coinIID == nil {
		darcBuf, err := d.ToProto()
		if err != nil {
			return nil, err
		}

		args = append(args, byzcoin.Argument{
			Name:  "newDarc",
			Value: darcBuf,
		})
	} else {
		args = append(args, byzcoin.Argument{
			Name:  "coinID",
			Value: coinIID.Slice(),
		})
	}

	ctx, err := cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: popIID,
		Invoke: &byzcoin.Invoke{
			ContractID: contracts.ContractPopPartyID,
			Command:    "mine",
			Args:       args,
		},
	})
	if err != nil {
		return nil, err
	}
	return cl.AddTransactionAndWait(ctx, 5)
}

// PopPartyMineDarcToCoin calculates the coin given a darc and returns the coin instance.
func PopPartyMineDarcToCoin(cl *byzcoin.Client, d *darc.Darc) (coinIID byzcoin.InstanceID, coin byzcoin.Coin, err error) {
	return PopPartyMineDarcToCoinAfter(cl, d, nil)
}

// PopPartyMineDarcToCoinAfter calculates the coin given a darc and returns
// the coin instance created/updated after the time barrier.
func PopPartyMineDarcToCoinAfter(cl *byzcoin.Client, d *darc.Darc, block *skipchain.SkipBlock) (coinIID byzcoin.InstanceID, coin byzcoin.Coin, err error) {
	h := sha256.New()
	h.Write([]byte("coin"))
	h.Write(d.GetBaseID())
	coinIID = byzcoin.NewInstanceID(h.Sum(nil))

	var proof *byzcoin.GetProofResponse
	proof, err = cl.GetProofAfter(coinIID.Slice(), false, block)
	if err != nil {
		return
	}
	var value []byte
	_, value, _, _, err = proof.Proof.KeyValue()
	if err != nil {
		return
	}
	err = protobuf.Decode(value, &coin)
	return
}
