package omniledger

import (
	"encoding/binary"
	"github.com/dedis/cothority"
	bc "github.com/dedis/cothority/byzcoin"
	"github.com/dedis/cothority/byzcoin/darc"
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

	esBuff := make([]byte, 8)
	binary.PutVarint(esBuff, int64(req.EpochSize))

	rosterBuf, err := protobuf.Encode(&(req.Roster))
	if err != nil {
		return nil, nil, err
	}

	tsBuf := make([]byte, 8)
	binary.BigEndian.PutUint64(tsBuf, uint64(req.Timestamp.Unix()))

	instr := &bc.Instruction{
		InstanceID: bc.NewInstanceID(d.GetBaseID()),
		Nonce:      bc.GenNonce(),
		Index:      0,
		Length:     1,
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
	}
	err = instr.SignBy(d.GetBaseID(), owner)
	if err != nil {
		return nil, nil, err
	}

	olInstID := instr.DeriveID("")

	// Add genesismsg and instr
	req.IBGenesisMsg = ibMsg
	req.SpawnInstruction = instr

	// Create reply struct
	req.Version = bc.CurrentVersion
	reply := &CreateOmniLedgerResponse{}
	err = c.SendProtobuf(req.Roster.List[0], req, reply)
	if err != nil {
		return nil, nil, err
	}
	reply.OmniledgerInstanceID = olInstID

	c.ID = reply.IDSkipBlock.CalculateHash()

	return c, reply, nil
}

func (c *Client) NewEpoch(req *NewEpoch) (*NewEpochResponse, error) {
	// Connect to IB via client
	ibClient := bc.NewClient(req.IBID, req.IBRoster)

	// Prepare and send request_new_epoch instruction to IB
	reqNewEpoch := bc.Instruction{
		Nonce:  bc.GenNonce(),
		Index:  0,
		Length: 1,
		Invoke: &bc.Invoke{
			Command: "request_new_epoch",
		},
	}
	reqNewEpoch.SignBy(req.IBDarcID.GetBaseID(), req.Owner)

	tx := bc.ClientTransaction{
		Instructions: []bc.Instruction{reqNewEpoch},
	}

	_, err := ibClient.AddTransactionAndWait(tx, 10)
	if err != nil {
		return nil, err
	}

	// Get proof from request_new_epoch instr, prepare the new_epoch instrutions and send them to the shards
	gpr, err := ibClient.GetProof(reqNewEpoch.DeriveID("").Slice())
	if err != nil {
		return nil, err
	}

	cc := &lib.ChainConfig{}
	err = gpr.Proof.ContractValue(cothority.Suite, ContractOmniledgerEpochID, cc)
	if err != nil {
		return nil, err
	}

	proofBuf, err := protobuf.Encode(gpr.Proof)
	if err != nil {
		return nil, err
	}

	for i := 0; i < len(cc.ShardRosters); i++ {
		shardIndBuff := make([]byte, 8)
		binary.PutVarint(shardIndBuff, int64(i))
		newEpoch := bc.Instruction{
			Nonce:  bc.GenNonce(),
			Index:  0,
			Length: 1,
			Invoke: &bc.Invoke{
				Command: "new_epoch",
				Args: []bc.Argument{
					bc.Argument{Name: "epoch", Value: proofBuf},
					bc.Argument{Name: "shard-index", Value: shardIndBuff},
					bc.Argument{Name: "ib-ID", Value: req.IBID},
				},
			},
		}
		newEpoch.SignBy(req.ShardDarcIDs[i].GetBaseID(), req.Owner)
		tx.Instructions[0] = newEpoch

		newRoster := cc.ShardRosters[i]
		oldRoster := req.ShardRosters[i]
		changesCount := getRosterChangesCount(oldRoster, newRoster)

		shardClient := bc.NewClient(req.ShardIDs[i], newRoster)
		for j := 0; j < changesCount; j++ {
			shardClient.AddTransactionAndWait(tx, 2)
		}
	}

	// TODO: Fill the reply w/ relevant changes
	reply := &NewEpochResponse{
		IBRoster:     *cc.Roster,
		ShardRosters: cc.ShardRosters,
	}

	return reply, nil
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
