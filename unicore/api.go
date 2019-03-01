package unicore

import (
	"fmt"

	"go.dedis.ch/cothority"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/onet/v3"
)

// BcConfig is
type BcConfig struct {
	Roster    onet.Roster
	ByzCoinID skipchain.SkipBlockID
}

// Client is
type Client struct {
	ByzCoinClient *byzcoin.Client
	DarcID        darc.ID
	Signers       []darc.Signer
	Counters      []uint64
	Instance      byzcoin.InstanceID

	oc *onet.Client
}

// NewClient is
func NewClient(cfg *BcConfig) *Client {
	return &Client{
		ByzCoinClient: byzcoin.NewClient(cfg.ByzCoinID, cfg.Roster),
		Signers:       []darc.Signer{},
		Counters:      []uint64{},
		oc:            onet.NewClient(cothority.Suite, ServiceName),
	}
}

// Create spawns an instance with an executable as the binary
func (c *Client) Create(binary []byte) error {
	instr := byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(c.DarcID),
		Spawn: &byzcoin.Spawn{
			ContractID: contractName,
			Args: []byzcoin.Argument{
				byzcoin.Argument{
					Name:  "binary",
					Value: binary,
				},
			},
		},
		SignerCounter: []uint64(c.Counters),
	}

	ctx := byzcoin.ClientTransaction{Instructions: []byzcoin.Instruction{instr}}
	if err := ctx.FillSignersAndSignWith(c.Signers...); err != nil {
		return err
	}

	if _, err := c.ByzCoinClient.AddTransactionAndWait(ctx, 10); err != nil {
		return err
	}

	c.Instance = ctx.Instructions[0].DeriveID("")
	c.increaseCounters()

	fmt.Printf("Unicore instance created with ID: %s", c.Instance.String())

	return nil
}

// Exec invoke the given instance and tries to execute the binary stored inside it
func (c *Client) Exec(args []byzcoin.Argument) error {
	instr := byzcoin.Instruction{
		InstanceID: c.Instance,
		Invoke: &byzcoin.Invoke{
			ContractID: contractName,
			Command:    "exec",
			Args:       args,
		},
		SignerCounter: []uint64(c.Counters),
	}

	ctx := byzcoin.ClientTransaction{Instructions: []byzcoin.Instruction{instr}}
	if err := ctx.FillSignersAndSignWith(c.Signers...); err != nil {
		return err
	}

	if _, err := c.ByzCoinClient.AddTransactionAndWait(ctx, 10); err != nil {
		return err
	}

	c.increaseCounters()

	return nil
}

// GetState returns the state of the smart contract
func (c *Client) GetState(iid byzcoin.InstanceID) ([]byte, error) {
	req := &GetStateRequest{
		ByzCoinID:  c.ByzCoinClient.ID,
		InstanceID: iid.Slice(),
	}

	reply := &GetStateReply{}
	err := c.oc.SendProtobuf(c.ByzCoinClient.Roster.List[0], req, reply)
	if err != nil {
		return nil, err
	}

	return reply.Value, nil
}

// AddSigner assigns a new signer to the client
func (c *Client) AddSigner(s darc.Signer) error {
	c.Signers = append(c.Signers, s)

	ctr, err := c.ByzCoinClient.GetSignerCounters(s.Identity().String())
	if err != nil {
		return err
	}

	c.Counters = append(c.Counters, ctr.Counters[0]+1)
	return nil
}

func (c *Client) increaseCounters() {
	for i, count := range c.Counters {
		c.Counters[i] = count + 1
	}
}
