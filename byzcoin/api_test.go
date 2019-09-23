package byzcoin

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
	"go.dedis.ch/protobuf"
)

func init() {
	// register a service for the test that will do nothing but reply with a chosen response
	onet.RegisterNewServiceWithSuite(testServiceName, pairingSuite, newTestService)
}

func TestClient_NewLedgerCorrupted(t *testing.T) {
	l := onet.NewTCPTest(cothority.Suite)
	servers, roster, _ := l.GenTree(3, true)
	defer l.CloseAll()

	service := servers[0].Service(testServiceName).(*corruptedService)
	signer := darc.NewSignerEd25519(nil, nil)
	msg, err := DefaultGenesisMsg(CurrentVersion, roster, []string{"spawn:dummy"}, signer.Identity())
	require.Nil(t, err)
	c := &Client{
		Client: onet.NewClient(cothority.Suite, testServiceName),
		Roster: *roster,
	}

	sb := skipchain.NewSkipBlock()
	service.CreateGenesisBlockResponse = &CreateGenesisBlockResponse{Skipblock: sb}

	sb.Roster = &onet.Roster{ID: onet.RosterID{}}
	sb.Hash = sb.CalculateHash()
	_, err = newLedgerWithClient(msg, c)
	require.Error(t, err)
	require.Equal(t, "wrong roster in genesis block", err.Error())

	sb.Roster = roster
	sb.Payload = []byte{1, 2, 3}
	sb.Hash = sb.CalculateHash()
	_, err = newLedgerWithClient(msg, c)
	require.Error(t, err)
	require.Contains(t, err.Error(), "fail to decode data:")

	sb.Payload = []byte{}
	sb.Hash = sb.CalculateHash()
	_, err = newLedgerWithClient(msg, c)
	require.Error(t, err)
	require.Equal(t, "genesis block should only have one transaction", err.Error())

	data := &DataBody{
		TxResults: []TxResult{
			TxResult{ClientTransaction: ClientTransaction{Instructions: []Instruction{Instruction{}}}},
		},
	}
	sb.Payload, err = protobuf.Encode(data)
	sb.Hash = sb.CalculateHash()
	require.NoError(t, err)
	_, err = newLedgerWithClient(msg, c)
	require.Error(t, err)
	require.Equal(t, "didn't get a spawn instruction", err.Error())

	data.TxResults[0].ClientTransaction.Instructions[0].Spawn = &Spawn{
		Args: []Argument{
			Argument{
				Name:  "darc",
				Value: []byte{1, 2, 3},
			},
		},
	}
	sb.Payload, err = protobuf.Encode(data)
	sb.Hash = sb.CalculateHash()
	require.NoError(t, err)
	_, err = newLedgerWithClient(msg, c)
	require.Error(t, err)
	require.Contains(t, err.Error(), "fail to decode the darc:")

	darcBytes, _ := protobuf.Encode(&darc.Darc{})
	data.TxResults[0].ClientTransaction.Instructions[0].Spawn = &Spawn{
		Args: []Argument{
			Argument{
				Name:  "darc",
				Value: darcBytes,
			},
		},
	}
	sb.Payload, err = protobuf.Encode(data)
	sb.Hash = sb.CalculateHash()
	require.NoError(t, err)
	_, err = newLedgerWithClient(msg, c)
	require.Error(t, err)
	require.Equal(t, "wrong darc spawned", err.Error())
}

func TestClient_CreateTransaction(t *testing.T) {
	c := Client{}

	header := DataHeader{Version: 5}
	bHeader, err := protobuf.Encode(&header)
	require.NoError(t, err)

	latest := skipchain.NewSkipBlock()
	latest.Data = bHeader
	c.Latest = latest

	instr := Instruction{}
	ctx, err := c.CreateTransaction(instr)
	require.NoError(t, err)
	require.Equal(t, Version(5), ctx.Instructions[0].version)
}

func TestClient_GetProof(t *testing.T) {
	l := onet.NewTCPTest(cothority.Suite)
	servers, roster, _ := l.GenTree(3, true)
	registerDummy(servers)
	defer l.CloseAll()

	// Initialise the genesis message and send it to the service.
	signer := darc.NewSignerEd25519(nil, nil)
	msg, err := DefaultGenesisMsg(CurrentVersion, roster, []string{"spawn:dummy"}, signer.Identity())
	msg.BlockInterval = 100 * time.Millisecond
	require.Nil(t, err)

	// The darc inside it should be valid.
	d := msg.GenesisDarc
	require.Nil(t, d.Verify(true))

	c, csr, err := NewLedger(msg, false)
	require.Nil(t, err)
	require.NoError(t, c.UseNode(0))

	gac, err := c.GetAllByzCoinIDs(roster.List[1])
	require.NoError(t, err)
	require.Equal(t, 1, len(gac.IDs))

	// Create a new transaction.
	value := []byte{5, 6, 7, 8}
	kind := "dummy"
	tx, err := createOneClientTx(d.GetBaseID(), kind, value, signer)
	require.Nil(t, err)
	_, err = c.AddTransactionAndWait(tx, 10)
	require.Nil(t, err)

	// We should have a proof of our transaction in the skipchain.
	newID := tx.Instructions[0].Hash()
	p, err := c.GetProof(newID)
	require.NoError(t, err)
	require.Nil(t, p.Proof.Verify(csr.Skipblock.SkipChainID()))
	require.Equal(t, 2, len(p.Proof.Links))
	k, v0, _, _, err := p.Proof.KeyValue()
	require.Nil(t, err)
	require.Equal(t, k, newID)
	require.Equal(t, value, v0)

	// The proof should now be smaller as we learnt about the block
	p, err = c.GetProofFromLatest(newID)
	require.NoError(t, err)
	require.Equal(t, 1, len(p.Proof.Links))
}

func TestClient_GetProofCorrupted(t *testing.T) {
	l := onet.NewTCPTest(cothority.Suite)
	servers, roster, _ := l.GenTree(3, true)
	defer l.CloseAll()

	service := servers[0].Service(testServiceName).(*corruptedService)

	c := &Client{
		Client: onet.NewClient(cothority.Suite, testServiceName),
		Roster: *roster,
	}
	gen := skipchain.NewSkipBlock()
	gen.Hash = gen.CalculateHash()
	c.ID = gen.Hash
	c.Genesis = gen
	// Fix on using only the leader
	require.NoError(t, c.UseNode(0))

	sb := skipchain.NewSkipBlock()
	sb.Data = []byte{1, 2, 3}
	service.GetProofResponse = &GetProofResponse{
		Proof: Proof{
			Latest: *sb,
			Links:  []skipchain.ForwardLink{skipchain.ForwardLink{To: c.ID}},
		},
	}

	_, err := c.GetProof([]byte{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "Error while decoding field")
}

// Create a streaming client and add blocks in the background. The client
// should receive valid blocks.
func TestClient_Streaming(t *testing.T) {
	l := onet.NewTCPTest(cothority.Suite)
	servers, roster, _ := l.GenTree(3, true)
	registerDummy(servers)
	defer l.CloseAll()

	// Initialise the genesis message and send it to the service.
	signer := darc.NewSignerEd25519(nil, nil)
	msg, err := DefaultGenesisMsg(CurrentVersion, roster, []string{"spawn:dummy"}, signer.Identity())
	msg.BlockInterval = time.Second
	require.Nil(t, err)

	// The darc inside it should be valid.
	d := msg.GenesisDarc
	require.Nil(t, d.Verify(true))

	c, csr, err := NewLedger(msg, false)
	require.Nil(t, err)

	n := 2
	go func() {
		time.Sleep(100 * time.Millisecond)
		for i := 0; i < n; i++ {
			value := []byte{5, 6, 7, 8}
			kind := "dummy"
			tx, err := createOneClientTxWithCounter(d.GetBaseID(), kind, value, signer, uint64(i)+1)
			// Need log.ErrFatal here, else it races with the rest of the code that
			// uses 't'.
			log.ErrFatal(err)
			_, err = c.AddTransaction(tx)
			log.ErrFatal(err)

			// sleep for a block interval so we create multiple blocks
			time.Sleep(msg.BlockInterval)
		}
	}()

	// Start collecting transactions
	c1 := NewClientKeep(csr.Skipblock.Hash, *roster)
	var xMut sync.Mutex
	var x int
	done := make(chan bool)
	cb := func(resp StreamingResponse, err error) {
		xMut.Lock()
		defer xMut.Unlock()
		if err != nil {
			// If we already closed the done channel, then it must
			// be after we've seen n blocks.
			require.True(t, x >= n)
			return
		}

		var body DataBody
		require.NotNil(t, resp.Block)
		err = protobuf.DecodeWithConstructors(resp.Block.Payload, &body, network.DefaultConstructors(cothority.Suite))
		require.NoError(t, err)
		for _, tx := range body.TxResults {
			for _, instr := range tx.ClientTransaction.Instructions {
				require.Equal(t, instr.Spawn.ContractID, "dummy")
			}
		}
		if x == n-1 {
			// We got n blocks, so we close the done channel.
			close(done)
		}
		x++
	}

	go func() {
		err = c1.StreamTransactions(cb)
		require.Nil(t, err)
	}()
	select {
	case <-done:
	case <-time.After(time.Duration(10) * msg.BlockInterval):
		require.Fail(t, "should have got n transactions")
	}
	require.NoError(t, c1.Close())
}

func TestClient_NoPhantomSkipchain(t *testing.T) {
	l := onet.NewTCPTest(cothority.Suite)
	servers, roster, _ := l.GenTree(3, true)
	registerDummy(servers)
	defer l.CloseAll()

	// Initialise the genesis message and send it to the service.
	signer := darc.NewSignerEd25519(nil, nil)
	msg, err := DefaultGenesisMsg(CurrentVersion, roster, []string{"spawn:dummy"}, signer.Identity())
	msg.BlockInterval = 100 * time.Millisecond
	require.NoError(t, err)

	d := msg.GenesisDarc

	c, _, err := NewLedger(msg, false)
	require.NoError(t, err)
	require.NoError(t, c.UseNode(0))

	gac, err := c.GetAllByzCoinIDs(roster.List[0])
	require.NoError(t, err)
	require.Equal(t, 1, len(gac.IDs))

	// Create a new transaction.
	tx, err := createOneClientTx(d.GetBaseID(), "dummy", []byte{}, signer)
	require.NoError(t, err)
	_, err = c.AddTransactionAndWait(tx, 10)
	require.NoError(t, err)

	gac, err = c.GetAllByzCoinIDs(roster.List[0])
	require.NoError(t, err)
	require.Equal(t, 1, len(gac.IDs))
}

// Insure that the decoder will return an error if the reply
// contains data from an old block.
func TestClient_SignerCounterDecoder(t *testing.T) {
	c := Client{}

	invalidReply := GetSignerCounters{}
	invalidBuff, err := protobuf.Encode(&invalidReply)
	require.NoError(t, err)
	// Invalid response type.
	require.Error(t, c.signerCounterDecoder(invalidBuff, &invalidReply))

	reply := GetSignerCountersResponse{
		Counters: []uint64{},
		Index:    0,
	}

	buf, err := protobuf.Encode(&reply)
	require.NoError(t, err)
	// Index is 0 so ignoring the check.
	require.NoError(t, c.signerCounterDecoder(buf, &reply))

	reply.Index = 1
	buf, err = protobuf.Encode(&reply)
	require.NoError(t, err)
	// Latest is nil.
	require.NoError(t, c.signerCounterDecoder(buf, &reply))

	c.Latest = &skipchain.SkipBlock{SkipBlockFix: &skipchain.SkipBlockFix{Index: 1}}
	// Correct scenario with Index = 1.
	require.NoError(t, c.signerCounterDecoder(buf, &reply))

	c.Latest.Index = 2
	// Incorrect scenario where the reply is older.
	require.Error(t, c.signerCounterDecoder(buf, &reply))
}

const testServiceName = "TestByzCoin"

type corruptedService struct {
	*Service

	// corrupted replies
	GetProofResponse           *GetProofResponse
	CreateGenesisBlockResponse *CreateGenesisBlockResponse
}

func newTestService(c *onet.Context) (onet.Service, error) {
	s := &Service{
		ServiceProcessor:       onet.NewServiceProcessor(c),
		contracts:              newContractRegistry(),
		txBuffer:               newTxBuffer(),
		storage:                &bcStorage{},
		darcToSc:               make(map[string]skipchain.SkipBlockID),
		stateChangeCache:       newStateChangeCache(),
		stateChangeStorage:     newStateChangeStorage(c),
		heartbeatsTimeout:      make(chan string, 1),
		closeLeaderMonitorChan: make(chan bool, 1),
		heartbeats:             newHeartbeats(),
		viewChangeMan:          newViewChangeManager(),
		streamingMan:           streamingManager{},
		closed:                 true,
	}

	cs := &corruptedService{Service: s}
	err := s.RegisterHandlers(cs.GetProof, cs.CreateGenesisBlock)

	return cs, err
}

func (cs *corruptedService) GetProof(req *GetProof) (resp *GetProofResponse, err error) {
	return cs.GetProofResponse, nil
}

func (cs *corruptedService) CreateGenesisBlock(req *CreateGenesisBlock) (*CreateGenesisBlockResponse, error) {
	return cs.CreateGenesisBlockResponse, nil
}
