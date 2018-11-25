package omniledger

import (
	"encoding/binary"
	"errors"
	"github.com/dedis/onet/log"
	//"fmt"
	"github.com/dedis/cothority"
	bc "github.com/dedis/cothority/byzcoin"
	"github.com/dedis/cothority/darc"
	lib "github.com/dedis/cothority/omniledger/lib"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/onet"
	"github.com/dedis/protobuf"
)

// ServiceName is used for registration on the onet.
const ServiceName = "OmniLedger"

// Client is a structure to communicate with the OmniLedger
// service.
type Client struct {
	*onet.Client
	ID     skipchain.SkipBlockID
	Roster onet.Roster
}

// NewClient returns a new client connected to the service
func NewClient(ID skipchain.SkipBlockID, Roster onet.Roster) *Client {
	return &Client{
		Client: onet.NewClient(cothority.Suite, ServiceName),
		ID:     ID,
		Roster: Roster,
	}
}

func NewOmniLedger(req *CreateOmniLedger) (*Client, *CreateOmniLedgerResponse,
	error) {
	// Create client
	c := NewClient(nil, req.Roster)

	// Fill request's missing fields
	owner := darc.NewSignerEd25519(nil, nil)

	ibMsg, err := bc.DefaultGenesisMsg(req.Version,
		&req.Roster, []string{"spawn:darc", "spawn:omniledgerepoch", "invoke:request_new_epoch"}, owner.Identity())
	if err != nil {
		return nil, nil, err
	}

	d := ibMsg.GenesisDarc

	darcBuf, err := protobuf.Encode(&d)
	if err != nil {
		return nil, nil, err
	}

	scBuff := make([]byte, 8)
	binary.PutVarint(scBuff, int64(req.ShardCount))

	esBuff := lib.EncodeDuration(req.EpochSize)

	rosterBuf, err := protobuf.Encode(&(req.Roster))
	if err != nil {
		return nil, nil, err
	}

	tsBuf := make([]byte, 8)
	binary.BigEndian.PutUint64(tsBuf, uint64(req.Timestamp.Unix()))

	/*signerCtrs, err := c.GetSignerCounters(owner.Identity().String())
	if err != nil {
		return nil, nil, err
	}
	if len(signerCtrs.Counters) != 1 {
		return nil, nil, errors.New("incorrect signer counter length")
	}*/

	instr := bc.Instruction{
		InstanceID: bc.NewInstanceID(d.GetBaseID()),
		Spawn: &bc.Spawn{
			ContractID: ContractOmniledgerEpochID,
			Args: []bc.Argument{
				bc.Argument{Name: "darc", Value: darcBuf},
				bc.Argument{Name: "roster", Value: rosterBuf},
				bc.Argument{Name: "shardCount", Value: scBuff},
				bc.Argument{Name: "epochSize", Value: esBuff},
				bc.Argument{Name: "timestamp", Value: tsBuf},
			},
		},
		SignerCounter: []uint64{1},
	}

	spawnTx := &bc.ClientTransaction{
		Instructions: bc.Instructions{instr},
	}
	spawnTx.SignWith(owner)
	//spawnTx.InstructionsHash = spawnTx.Instructions.Hash()

	// Add genesismsg and instr
	req.IBGenesisMsg = ibMsg
	req.SpawnTx = spawnTx
	req.OwnerID = owner.Identity()

	// Create reply struct
	//req.Version = bc.CurrentVersion
	reply := &CreateOmniLedgerResponse{}
	reply.Owner = owner
	err = c.SendProtobuf(req.Roster.List[0], req, reply)
	if err != nil {
		return nil, nil, err
	}

	c.ID = reply.IDSkipBlock.CalculateHash()

	return c, reply, nil
}

func (c *Client) NewEpoch(req *NewEpoch) (*NewEpochResponse, error) {
	// Connect to IB via client
	ibClient := bc.NewClient(req.IBID, req.IBRoster)

	// Fetch old roster
	gpr, err := ibClient.GetProof(req.OLInstanceID.Slice())
	oldCC := &lib.ChainConfig{}
	err = gpr.Proof.VerifyAndDecode(cothority.Suite, ContractOmniledgerEpochID, oldCC)
	oldRosters := oldCC.ShardRosters

	// Get IB signer counter
	signerCtrs, err := ibClient.GetSignerCounters(req.Owner.Identity().String())
	if err != nil {
		return nil, err
	}
	if len(signerCtrs.Counters) != 1 {
		return nil, errors.New("incorrect signer counter length")
	}

	// Prepare and send request_new_epoch instruction to IB
	tsBuf := make([]byte, 8)
	binary.BigEndian.PutUint64(tsBuf, uint64(req.Timestamp.Unix()))

	reqNewEpoch := bc.Instruction{
		InstanceID: req.OLInstanceID,
		Invoke: &bc.Invoke{
			Command: "request_new_epoch",
			Args: []bc.Argument{
				bc.Argument{Name: "timestamp", Value: tsBuf},
			},
		},
		SignerCounter: []uint64{signerCtrs.Counters[0] + 1},
	}
	tx := bc.ClientTransaction{
		Instructions: []bc.Instruction{reqNewEpoch},
	}
	tx.SignWith(req.Owner)

	req.ReqNewEpochTx = &tx

	// Send tx via service, get response
	reply := &NewEpochResponse{}
	olClient := NewClient(req.IBID, req.IBRoster)
	err = olClient.SendProtobuf(req.IBRoster.List[0], req, reply)
	if err != nil {
		return nil, err
	}

	// Get latest/New chain config from service response
	latestCC := &lib.ChainConfig{}
	err = reply.ReqNewEpochProof.VerifyAndDecode(cothority.Suite, ContractOmniledgerEpochID, latestCC)
	if err != nil {
		return nil, err
	}

	proofBuf, err := protobuf.Encode(&gpr.Proof)
	if err != nil {
		return nil, err
	}

	for i := 0; i < len(latestCC.ShardRosters); i++ {
		//log.Print("SENDING TO SHARD", i)
		oldRoster := oldRosters[i]
		newRoster := latestCC.ShardRosters[i]
		changesCount := getRosterChangesCount(oldRoster, newRoster)
		log.Print("OLD ROSTER:", oldRoster.List)
		log.Print("NEW ROSTER:", newRoster.List)
		log.Print("#CHANGES:", changesCount)

		log.Print("SHARD ID", req.ShardIDs[i])
		shardClient := bc.NewClient(req.ShardIDs[i], newRoster)

		shardIndBuff := make([]byte, 8)
		binary.PutVarint(shardIndBuff, int64(i))

		newEpoch := bc.Instruction{
			InstanceID: bc.NewInstanceID(nil),
			Invoke: &bc.Invoke{
				Command: "new_epoch",
				Args: []bc.Argument{
					bc.Argument{Name: "epoch", Value: proofBuf},
					bc.Argument{Name: "shard-index", Value: shardIndBuff},
					bc.Argument{Name: "ib-ID", Value: req.IBID},
				},
			},
		}

		// Fetch counter of each shard
		shardSignerCounter, err := shardClient.GetSignerCounters(req.Owner.Identity().String())
		if err != nil {
			return nil, err
		}

		//newEpoch.SignBy(req.ShardDarcIDs[i].GetBaseID(), req.Owner)
		//tx.Instructions[0] = newEpoch
		//tx.SignWith(req.Owner)

		for j := 0; j < changesCount; j++ {
			log.Print("ACTUALLY SENDING IT", j)
			// TODO:
			// Just update the signer counter
			// And resign the tx
			// Increment the counter for every repeated instruction

			//newEpoch.SignerCounter = []uint64{uint64(j + 1)}

			newEpoch.SignerCounter = []uint64{shardSignerCounter.Counters[0] + uint64(j+1)}
			tx.Instructions[0] = newEpoch
			tx.SignWith(req.Owner)
			shardClient.AddTransactionAndWait(tx, 5)
		}
	}

	clientReply := &NewEpochResponse{
		IBRoster: *latestCC.Roster,
	}

	return clientReply, nil
}

func getRosterChangesCount(oldRoster, newRoster onet.Roster) int {
	smallRoster := oldRoster.List
	largeRoster := newRoster.List

	if len(smallRoster) > len(largeRoster) {
		temp := smallRoster
		smallRoster = largeRoster
		largeRoster = temp
	}

	changesCount := 0

	smallSet := make(map[string]bool)
	for _, node := range smallRoster {
		id := node.ID.String()
		smallSet[id] = true
	}

	largeSet := make(map[string]bool)
	for _, node := range largeRoster {
		id := node.ID.String()
		largeSet[id] = true

		if _, ok := smallSet[id]; !ok {
			changesCount++
		}
	}

	for _, node := range smallRoster {
		id := node.ID.String()
		if _, ok := largeSet[id]; !ok {
			changesCount++
		}
	}

	return changesCount
}
