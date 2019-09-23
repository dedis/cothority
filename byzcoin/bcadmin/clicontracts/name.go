package clicontracts

import (
	"encoding/hex"
	"errors"

	"github.com/urfave/cli"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/byzcoin/bcadmin/lib"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
	"go.dedis.ch/protobuf"
)

// NameSpawn is used to spawn the name contract (can only be done once)
func NameSpawn(c *cli.Context) error {
	bcArg := c.String("bc")
	if bcArg == "" {
		return errors.New("--bc flag is required")
	}

	cfg, cl, err := lib.LoadConfig(bcArg)
	if err != nil {
		return err
	}

	gDarc, err := cl.GetGenDarc()
	if err != nil {
		return errors.New("failed to get genesis darc: " + err.Error())
	}

	var signer *darc.Signer

	sstr := c.String("sign")
	if sstr == "" {
		signer, err = lib.LoadKey(cfg.AdminIdentity)
	} else {
		signer, err = lib.LoadKeyFromString(sstr)
	}
	if err != nil {
		return err
	}

	counters, err := cl.GetSignerCounters(signer.Identity().String())

	ctx, err := cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(gDarc.GetBaseID()),
		Spawn: &byzcoin.Spawn{
			ContractID: byzcoin.ContractNamingID,
		},
		SignerCounter: []uint64{counters.Counters[0] + 1},
	})

	err = ctx.FillSignersAndSignWith(*signer)
	if err != nil {
		return errors.New("failed to fill signer: " + err.Error())
	}

	_, err = cl.AddTransactionAndWait(ctx, 10)
	if err != nil {
		return errors.New("failed to add transaction and wait: " + err.Error())
	}

	instID := ctx.Instructions[0].DeriveID("").Slice()
	log.Infof("Spawned a new namne contract. Its instance id is:\n%x", instID)

	return lib.WaitPropagation(c, cl)
}

// NameInvokeAdd is used to add a new name resolver
func NameInvokeAdd(c *cli.Context) error {
	bcArg := c.String("bc")
	if bcArg == "" {
		return errors.New("--bc flag is required")
	}

	cfg, cl, err := lib.LoadConfig(bcArg)
	if err != nil {
		return err
	}

	var signer *darc.Signer

	sstr := c.String("sign")
	if sstr == "" {
		signer, err = lib.LoadKey(cfg.AdminIdentity)
	} else {
		signer, err = lib.LoadKeyFromString(sstr)
	}
	if err != nil {
		return err
	}

	counters, err := cl.GetSignerCounters(signer.Identity().String())
	if err != nil {
		return errors.New("failed to get signer counters: " + err.Error())
	}

	name := c.String("name")
	if name == "" {
		return errors.New("--name flag is required")
	}

	instIDs := c.StringSlice("instid")

	if len(instIDs) == 0 {
		return errors.New("--instid flag is required")
	}

	append := c.Bool("append")

	multiple := false
	if len(instIDs) > 1 || append {
		multiple = true
	}
	names := make([]string, len(instIDs))

	instructions := make([]byzcoin.Instruction, len(instIDs))
	for i, instID := range instIDs {
		instIDBuf, err := hex.DecodeString(instID)
		if err != nil {
			return errors.New("failed to decode the instID string" + instID)
		}

		usedName := name
		if multiple {
			usedName = name + "-" + lib.RandString(16)
		}
		names[i] = usedName

		instructions[i] = byzcoin.Instruction{
			InstanceID: byzcoin.NamingInstanceID,
			Invoke: &byzcoin.Invoke{
				ContractID: byzcoin.ContractNamingID,
				Command:    "add",
				Args: byzcoin.Arguments{
					{
						Name:  "instanceID",
						Value: instIDBuf,
					},
					{
						Name:  "name",
						Value: []byte(usedName),
					},
				},
			},
			SignerCounter: []uint64{counters.Counters[0] + 1 + uint64(i)},
		}
	}

	ctx, err := cl.CreateTransaction(instructions...)
	if err != nil {
		return err
	}

	err = ctx.FillSignersAndSignWith(*signer)
	if err != nil {
		return err
	}

	_, err = cl.AddTransactionAndWait(ctx, 10)
	if err != nil {
		return err
	}

	for i, inst := range ctx.Instructions {
		instID := inst.DeriveID("").Slice()
		log.Infof("Added a new naming instance with name '%s'. "+
			"Its instance id is:\n%x", names[i], instID)
	}

	return lib.WaitPropagation(c, cl)
}

// NameInvokeRemove is used to remove a new name resolver
func NameInvokeRemove(c *cli.Context) error {
	bcArg := c.String("bc")
	if bcArg == "" {
		return errors.New("--bc flag is required")
	}

	cfg, cl, err := lib.LoadConfig(bcArg)
	if err != nil {
		return err
	}

	var signer *darc.Signer

	sstr := c.String("sign")
	if sstr == "" {
		signer, err = lib.LoadKey(cfg.AdminIdentity)
	} else {
		signer, err = lib.LoadKeyFromString(sstr)
	}
	if err != nil {
		return err
	}

	counters, err := cl.GetSignerCounters(signer.Identity().String())
	if err != nil {
		return errors.New("failed to get signer counters: " + err.Error())
	}

	name := c.String("name")
	if name == "" {
		return errors.New("--name flag is required")
	}

	instID := c.String("instid")
	if instID == "" {
		return errors.New("--instid flag required")
	}

	instIDBuf, err := hex.DecodeString(instID)
	if err != nil {
		return errors.New("failed to decode the instID string" + instID)
	}

	ctx, err := cl.CreateTransaction(byzcoin.Instruction{
		InstanceID: byzcoin.NamingInstanceID,
		Invoke: &byzcoin.Invoke{
			ContractID: byzcoin.ContractNamingID,
			Command:    "remove",
			Args: byzcoin.Arguments{
				{
					Name:  "instanceID",
					Value: instIDBuf,
				},
				{
					Name:  "name",
					Value: []byte(name),
				},
			},
		},
		SignerCounter: []uint64{counters.Counters[0] + 1},
	})
	if err != nil {
		return err
	}

	err = ctx.FillSignersAndSignWith(*signer)
	if err != nil {
		return err
	}

	_, err = cl.AddTransactionAndWait(ctx, 10)
	if err != nil {
		return err
	}

	newInstID := ctx.Instructions[0].DeriveID("").Slice()
	log.Infof("Name entry deleted! (instance ID is %x)", newInstID)

	return lib.WaitPropagation(c, cl)
}

// NameGet displays the name contract (which is a singleton). No need to provide
// the instance id since the name contract has a pre-determined one.
func NameGet(c *cli.Context) error {
	bcArg := c.String("bc")
	if bcArg == "" {
		return errors.New("--bc flag is required")
	}

	_, cl, err := lib.LoadConfig(bcArg)
	if err != nil {
		return err
	}

	// Get the latest name instance value
	pr, err := cl.GetProofFromLatest(byzcoin.NamingInstanceID.Slice())
	if err != nil {
		return errors.New("couldn't get proof for NamingInstanceID: " + err.Error())
	}
	proof := pr.Proof

	_, value, _, _, err := proof.KeyValue()
	if err != nil {
		return errors.New("couldn't get value out of proof: " + err.Error())
	}
	namingBody := byzcoin.ContractNamingBody{}
	err = protobuf.DecodeWithConstructors(value, &namingBody,
		network.DefaultConstructors(cothority.Suite))
	if err != nil {
		return errors.New("couldn't decode ContractNamingBody: " + err.Error())
	}

	log.Infof("Here is the naming data:\n%s", namingBody)

	return nil
}
