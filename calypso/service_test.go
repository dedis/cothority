package calypso

import (
	"fmt"
	"go.dedis.ch/cothority/v3/darc/expression"
	"sync"
	"testing"
	"time"

	"golang.org/x/xerrors"

	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/share"
	"go.dedis.ch/kyber/v3/util/key"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/protobuf"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}

// TestService_CreateLTS runs the DKG protocol on the service and check that we
// get back valid results.
func TestService_CreateLTS(t *testing.T) {
	for _, nodes := range []int{4, 7} {
		func(nodes int) {
			if nodes > 5 && testing.Short() {
				log.Info("skipping, dkg might take too long for", nodes)
				return
			}
			s := newTS(t, nodes)
			defer s.closeAll(t)
			require.NotNil(t, s.ltsReply.ByzCoinID)
			require.NotNil(t, s.ltsReply.InstanceID)
			require.NotNil(t, s.ltsReply.X)
		}(nodes)
	}
}

// Try to change the roster to a new roster that is disjoint, which
// should result in an error.
func TestService_ReshareLTS_Different(t *testing.T) {
	nodes := 4
	s := newTSWithExtras(t, nodes, nodes)
	defer s.closeAll(t)
	require.NotNil(t, s.ltsReply.ByzCoinID)
	require.NotNil(t, s.ltsReply.InstanceID)
	require.NotNil(t, s.ltsReply.X)

	// The current DKG is on List[0:nodes], and this new roster will
	// be on List[nodes:], thus entirely disjoint.
	otherRoster := onet.NewRoster(s.allRoster.List[nodes:])
	ltsInstInfoBuf, err := protobuf.Encode(&LtsInstanceInfo{*otherRoster})
	require.NoError(t, err)

	ctx := byzcoin.NewClientTransaction(byzcoin.CurrentVersion,
		byzcoin.Instruction{
			InstanceID: s.ltsReply.InstanceID,
			Invoke: &byzcoin.Invoke{
				ContractID: ContractLongTermSecretID,
				Command:    "reshare",
				Args: []byzcoin.Argument{
					{
						Name:  "lts_instance_info",
						Value: ltsInstInfoBuf,
					},
				},
			},
			SignerCounter: []uint64{2},
		},
	)
	require.Nil(t, ctx.FillSignersAndSignWith(s.signer))
	_, err = s.cl.AddTransactionAndWait(ctx, 4)
	require.Error(t, err)
}

func TestService_ReshareLTS_Same(t *testing.T) {
	for _, nodes := range []int{4, 7} {
		func(nodes int) {
			if nodes > 5 && testing.Short() {
				log.Info("skipping, dkg might take too long for", nodes)
				return
			}
			s := newTS(t, nodes)
			defer s.closeAll(t)
			require.NotNil(t, s.ltsReply.ByzCoinID)
			require.NotNil(t, s.ltsReply.InstanceID)
			require.NotNil(t, s.ltsReply.X)
			sec1 := s.reconstructKey(t)

			ltsInstInfoBuf, err := protobuf.Encode(&LtsInstanceInfo{*s.ltsRoster})
			require.NoError(t, err)

			ctx, err := s.cl.CreateTransaction(byzcoin.Instruction{
				InstanceID: s.ltsReply.InstanceID,
				Invoke: &byzcoin.Invoke{
					ContractID: ContractLongTermSecretID,
					Command:    "reshare",
					Args: []byzcoin.Argument{
						{
							Name:  "lts_instance_info",
							Value: ltsInstInfoBuf,
						},
					},
				},
				SignerCounter: []uint64{2},
			})
			require.NoError(t, err)
			require.NoError(t, ctx.FillSignersAndSignWith(s.signer))
			_, err = s.cl.AddTransactionAndWait(ctx, 4)
			require.NoError(t, err)

			// Get the proof and start resharing
			proof, err := s.cl.GetProof(s.ltsReply.InstanceID.Slice())
			require.NoError(t, err)

			log.Lvl1("first reshare")
			var wg sync.WaitGroup
			wg.Add(len(s.ltsRoster.List))
			s.afterReshare(func() { wg.Done() })
			_, err = s.services[0].ReshareLTS(&ReshareLTS{
				Proof: proof.Proof,
			})
			require.NoError(t, err)
			wg.Wait()
			r := s.reconstructKey(t)
			require.True(t, r.Equal(sec1))

			// Try to do resharing again
			wg.Add(len(s.ltsRoster.List))
			log.Lvl1("second reshare")
			_, err = s.services[0].ReshareLTS(&ReshareLTS{
				Proof: proof.Proof,
			})
			require.NoError(t, err)
			wg.Wait()
			require.True(t, s.reconstructKey(t).Equal(sec1))
		}(nodes)
	}
}

func TestService_ReshareLTS_OneMore(t *testing.T) {
	for _, nodes := range []int{4, 7} {
		func(nodes int) {
			if nodes > 5 && testing.Short() {
				log.Info("skipping, dkg might take too long for", nodes)
				return
			}
			s := newTSWithExtras(t, nodes, 1)
			defer s.closeAll(t)
			require.NotNil(t, s.ltsReply.ByzCoinID)
			require.NotNil(t, s.ltsReply.InstanceID)
			require.NotNil(t, s.ltsReply.X)
			sec1 := s.reconstructKey(t)

			// Create a new roster that has one more node than
			// before
			s.ltsRoster = onet.NewRoster(s.allRoster.List[:nodes+1])
			ltsInstInfoBuf, err := protobuf.Encode(&LtsInstanceInfo{*s.ltsRoster})
			require.NoError(t, err)

			ctx, err := s.cl.CreateTransaction(byzcoin.Instruction{
				InstanceID: s.ltsReply.InstanceID,
				Invoke: &byzcoin.Invoke{
					ContractID: ContractLongTermSecretID,
					Command:    "reshare",
					Args: []byzcoin.Argument{
						{
							Name:  "lts_instance_info",
							Value: ltsInstInfoBuf,
						},
					},
				},
				SignerCounter: []uint64{2},
			})
			require.NoError(t, err)
			require.NoError(t, ctx.FillSignersAndSignWith(s.signer))
			atr, err := s.cl.AddTransactionAndWait(ctx, 4)
			require.NoError(t, err)

			// Get the proof and start resharing
			proof, err := s.cl.GetProofAfter(s.ltsReply.InstanceID.Slice(), true, &atr.Proof.Latest)
			require.NoError(t, err)

			log.Lvl1("first reshare")
			var wg sync.WaitGroup
			wg.Add(len(s.ltsRoster.List))
			s.afterReshare(func() { wg.Done() })
			_, err = s.services[0].ReshareLTS(&ReshareLTS{
				Proof: proof.Proof,
			})
			require.NoError(t, err)
			wg.Wait()
			require.True(t, s.reconstructKey(t).Equal(sec1))

			// Try to do resharing again
			log.Lvl1("second reshare")
			wg.Add(len(s.ltsRoster.List))
			_, err = s.services[0].ReshareLTS(&ReshareLTS{
				Proof: proof.Proof,
			})
			require.NoError(t, err)
			wg.Wait()
			require.True(t, s.reconstructKey(t).Equal(sec1))
		}(nodes)
	}
}

// TestContract_Write creates a write request and check that it gets stored.
func TestContract_Write(t *testing.T) {
	s := newTS(t, 5)
	defer s.closeAll(t)

	pr := s.addWriteAndWait(t, []byte("secret key"))
	require.Nil(t, pr.Verify(s.gbReply.Skipblock.Hash))
}

// TestContract_Write_Benchmark makes many write requests transactions and logs
// the transaction per second.
func TestContract_Write_Benchmark(t *testing.T) {
	log.SetDebugVisible(1)
	if testing.Short() {
		t.Skip("running benchmark takes too long and it's extremely CPU intensive (100% CPU usage)")
	}

	//s := newTS(t, 5)
	s := newTS(t, 10)
	defer s.closeAll(t)
	require.NoError(t, s.cl.UseNode(0))

	//totalTrans := 50
	totalTrans := 256
	var times []time.Duration

	var ctr uint64 = 2
	for i := 0; i < 1; i++ {
		//for i := 0; i < 1; i++ {
		log.Lvl1("Creating transaction", i)
		iids := make([]byzcoin.InstanceID, totalTrans)
		start := time.Now()
		for i := 0; i < totalTrans; i++ {
			iids[i] = s.addWrite(t, []byte("secret key"), ctr)
			ctr++
		}
		timeSend := time.Since(start)
		fmt.Println("DURATION:" + timeSend.String())
		log.Lvlf1("Time to send %d writes to the ledger: %s", totalTrans, timeSend)
		start = time.Now()
		for i := 0; i < totalTrans; i++ {
			s.waitInstID(t, iids[i])
		}
		timeWait := time.Since(start)
		log.Lvlf1("Time to wait for %d writes in the ledger: %s", totalTrans, timeWait)
		times = append(times, timeSend+timeWait)
		for _, ti := range times {
			log.Lvlf1("Total time: %s - tps: %f", ti,
				float64(totalTrans)/ti.Seconds())
		}
	}
}

func setupDarcs(s ts, count int) ([]darc.Signer, []*darc.Darc) {
	writers := make([]darc.Signer, count)
	wDarcs := make([]*darc.Darc, count)

	for i := 0; i < count; i++ {
		writer := darc.NewSignerEd25519(nil, nil)
		wDarc := darc.NewDarc(darc.InitRules([]darc.Identity{writer.Identity()}, []darc.Identity{writer.Identity()}), []byte(fmt.Sprintf("%s%d", "Writer", i)))
		wDarc.Rules.AddRule(darc.Action("spawn:"+ContractWriteID),
			expression.InitOrExpr(writer.Identity().String()))
		ctr := uint64(2)
		darcBuf, _ := wDarc.ToProto()
		ctx := byzcoin.NewClientTransaction(byzcoin.CurrentVersion,
			byzcoin.Instruction{
				InstanceID: byzcoin.NewInstanceID(s.gDarc.GetBaseID()),
				Spawn: &byzcoin.Spawn{
					ContractID: byzcoin.ContractDarcID,
					Args: []byzcoin.Argument{{
						Name:  "darc",
						Value: darcBuf,
					}},
				},
				SignerCounter: []uint64{ctr + uint64(i)},
			},
		)
		_ = ctx.FillSignersAndSignWith(s.signer)
		if i == count-1 {
			_, _ = s.cl.AddTransactionAndWait(ctx, 10)
		} else {
			_, _ = s.cl.AddTransaction(ctx)
		}
		writers[i] = writer
		wDarcs[i] = wDarc
	}

	return writers, wDarcs
}

func (s *ts) addWriteBurst(t *testing.T, writer darc.Signer, wDarc darc.Darc, key []byte, ctr uint64) byzcoin.InstanceID {
	write := NewWrite(cothority.Suite, s.ltsReply.InstanceID, wDarc.GetBaseID(), s.ltsReply.X, key)
	writeBuf, err := protobuf.Encode(write)
	require.NoError(t, err)

	ctx := byzcoin.NewClientTransaction(byzcoin.CurrentVersion,
		byzcoin.Instruction{
			InstanceID: byzcoin.NewInstanceID(wDarc.GetBaseID()),
			Spawn: &byzcoin.Spawn{
				ContractID: ContractWriteID,
				Args:       byzcoin.Arguments{{Name: "write", Value: writeBuf}},
			},
			SignerCounter: []uint64{ctr},
		},
	)
	require.Nil(t, ctx.FillSignersAndSignWith(writer))
	_, err = s.cl.AddTransaction(ctx)
	require.NoError(t, err)
	return ctx.Instructions[0].DeriveID("")
}

// TestContract_Write_Benchmark makes many write requests transactions and logs
// the transaction per second.
func TestContract_WriteBurst_Benchmark(t *testing.T) {
	log.SetDebugVisible(1)
	var wg sync.WaitGroup
	if testing.Short() {
		t.Skip("running benchmark takes too long and it's extremely CPU intensive (100% CPU usage)")
	}

	s := newTS(t, 10)
	defer s.closeAll(t)
	require.NoError(t, s.cl.UseNode(0))

	totalTrans := 4096
	times := make([]time.Duration, totalTrans)

	writers, wDarcs := setupDarcs(s, totalTrans)

	for i := 0; i < 1; i++ {
		log.Lvl1("Creating transaction", i)
		iids := make([]byzcoin.InstanceID, totalTrans)
		wg.Add(totalTrans)
		for j := 0; j < totalTrans; j++ {
			go func(idx int) {
				defer wg.Done()
				start := time.Now()
				iids[idx] = s.addWriteBurst(t, writers[idx], *wDarcs[idx],
					[]byte("secret key"), 1)
				s.waitInstID(t, iids[idx])
				timeWait := time.Since(start)
				times[idx] = timeWait
			}(j)
		}
		wg.Wait()

		sum := 0.0
		for _, ti := range times {
			sum += ti.Seconds()
			//log.Lvlf1("Total time: %s - tps: %f", ti,
			//	float64(totalTrans)/ti.Seconds())
		}
		log.Lvlf1("Avg: %f", sum/float64(totalTrans))
		log.Lvlf1("%d", len(times))
	}
}

// TestContract_Read makes a write requests and a corresponding read request
// which should be created from the write instance.
func TestContract_Read(t *testing.T) {
	s := newTS(t, 5)
	defer s.closeAll(t)

	prWrite := s.addWriteAndWait(t, []byte("secret key"))
	pr := s.addReadAndWait(t, prWrite, s.signer.Ed25519.Point)
	require.Nil(t, pr.Verify(s.gbReply.Skipblock.Hash))
}

// TestService_DecryptKey is an end-to-end test that logs two write and read
// requests and make sure that we can decrypt the secret afterwards.
func TestService_DecryptKey(t *testing.T) {
	s := newTS(t, 5)
	defer s.closeAll(t)

	key1 := []byte("secret key 1")
	prWr1 := s.addWriteAndWait(t, key1)
	prRe1 := s.addReadAndWait(t, prWr1, s.signer.Ed25519.Point)
	key2 := []byte("secret key 2")
	prWr2 := s.addWriteAndWait(t, key2)
	prRe2 := s.addReadAndWait(t, prWr2, s.signer.Ed25519.Point)

	_, err := s.services[0].DecryptKey(&DecryptKey{Read: *prRe1, Write: *prWr2})
	require.NotNil(t, err)
	_, err = s.services[0].DecryptKey(&DecryptKey{Read: *prRe2, Write: *prWr1})
	require.NotNil(t, err)

	dk1, err := s.services[0].DecryptKey(&DecryptKey{Read: *prRe1, Write: *prWr1})
	require.NoError(t, err)
	require.True(t, dk1.X.Equal(s.ltsReply.X))
	keyCopy1, err := dk1.RecoverKey(s.signer.Ed25519.Secret)
	require.NoError(t, err)
	require.Equal(t, key1, keyCopy1)

	dk2, err := s.services[0].DecryptKey(&DecryptKey{Read: *prRe2, Write: *prWr2})
	require.NoError(t, err)
	require.True(t, dk2.X.Equal(s.ltsReply.X))
	keyCopy2, err := dk2.RecoverKey(s.signer.Ed25519.Secret)
	require.NoError(t, err)
	require.Equal(t, key2, keyCopy2)
}

// TestService_DecryptEphemeralKey requests a read to a different key than the
// readers.
func TestService_DecryptEphemeralKey(t *testing.T) {
	s := newTS(t, 5)
	defer s.closeAll(t)

	ephemeral := key.NewKeyPair(cothority.Suite)

	key1 := []byte("secret key 1")
	prWr1 := s.addWriteAndWait(t, key1)
	prRe1 := s.addReadAndWait(t, prWr1, ephemeral.Public)

	dk1, err := s.services[0].DecryptKey(&DecryptKey{Read: *prRe1, Write: *prWr1})
	require.NoError(t, err)
	require.True(t, dk1.X.Equal(s.ltsReply.X))

	keyCopy1, err := dk1.RecoverKey(ephemeral.Private)
	require.NoError(t, err)
	require.Equal(t, key1, keyCopy1)
}

type ts struct {
	local      *onet.LocalTest
	servers    []*onet.Server
	services   []*Service
	allRoster  *onet.Roster
	ltsRoster  *onet.Roster
	byzRoster  *onet.Roster
	ltsReply   *CreateLTSReply
	signer     darc.Signer
	cl         *byzcoin.Client
	gbReply    *byzcoin.CreateGenesisBlockResponse
	genesisMsg *byzcoin.CreateGenesisBlock
	gDarc      *darc.Darc
}

func (s *ts) addRead(t *testing.T, write *byzcoin.Proof, Xc kyber.Point, ctr uint64) byzcoin.InstanceID {
	var readBuf []byte
	read := &Read{
		Write: byzcoin.NewInstanceID(write.InclusionProof.Key()),
		Xc:    Xc,
	}
	var err error
	readBuf, err = protobuf.Encode(read)
	require.NoError(t, err)
	ctx := byzcoin.NewClientTransaction(byzcoin.CurrentVersion,
		byzcoin.Instruction{
			InstanceID: byzcoin.NewInstanceID(write.InclusionProof.Key()),
			Spawn: &byzcoin.Spawn{
				ContractID: ContractReadID,
				Args:       byzcoin.Arguments{{Name: "read", Value: readBuf}},
			},
			SignerCounter: []uint64{ctr},
		},
	)
	require.Nil(t, ctx.FillSignersAndSignWith(s.signer))
	_, err = s.cl.AddTransaction(ctx)
	require.NoError(t, err)
	return ctx.Instructions[0].DeriveID("")
}

func (s *ts) addReadAndWait(t *testing.T, write *byzcoin.Proof, Xc kyber.Point) *byzcoin.Proof {
	ctr, err := s.cl.GetSignerCounters(s.signer.Identity().String())
	require.NoError(t, err)
	instID := s.addRead(t, write, Xc, ctr.Counters[0]+1)
	return s.waitInstID(t, instID)
}

func newTS(t *testing.T, nodes int) ts {
	return newTSWithExtras(t, nodes, 0)
}

// newTSWithExtras initially the byzRoster and ltsRoster are the same, the extras are
// there so that we can change the ltsRoster later to be something different.
func newTSWithExtras(t *testing.T, nodes int, extras int) ts {
	allowInsecureAdmin = true
	s := ts{}
	s.local = onet.NewLocalTestT(cothority.Suite, t)

	// Create the service
	s.servers, s.allRoster, _ = s.local.GenTree(nodes+extras, true)
	services := s.local.GetServices(s.servers, calypsoID)
	for _, ser := range services {
		s.services = append(s.services, ser.(*Service))
	}
	s.byzRoster = onet.NewRoster(s.allRoster.List[:nodes])
	s.ltsRoster = onet.NewRoster(s.allRoster.List[:nodes])

	// Create the skipchain
	s.signer = darc.NewSignerEd25519(nil, nil)
	s.createGenesis(t)

	// Create LTS instance
	ltsInstInfoBuf, err := protobuf.Encode(&LtsInstanceInfo{*s.ltsRoster})
	require.NoError(t, err)
	inst := byzcoin.Instruction{
		InstanceID: byzcoin.NewInstanceID(s.gDarc.GetBaseID()),
		Spawn: &byzcoin.Spawn{
			ContractID: ContractLongTermSecretID,
			Args: []byzcoin.Argument{
				{
					Name:  "lts_instance_info",
					Value: ltsInstInfoBuf,
				},
			},
		},
		SignerCounter: []uint64{1},
	}
	tx, err := s.cl.CreateTransaction(inst)
	require.NoError(t, err)
	require.NoError(t, tx.FillSignersAndSignWith(s.signer))
	_, err = s.cl.AddTransactionAndWait(tx, 4)
	require.NoError(t, err)

	// Get the proof
	proof, err := s.cl.WaitProof(tx.Instructions[0].DeriveID(""), s.genesisMsg.BlockInterval, nil)
	require.NoError(t, err)

	// Start DKG
	s.ltsReply, err = s.services[0].CreateLTS(&CreateLTS{
		Proof: *proof,
	})
	require.NoError(t, err)

	reply2 := CreateLTSReply{
		ByzCoinID:  s.gbReply.Skipblock.SkipChainID(),
		InstanceID: tx.Instructions[0].DeriveID(""),
		X:          s.ltsReply.X,
	}

	require.True(t, s.ltsReply.InstanceID.Equal(reply2.InstanceID))
	return s
}

func (s *ts) createGenesis(t *testing.T) {
	var err error
	s.genesisMsg, err = byzcoin.DefaultGenesisMsg(byzcoin.CurrentVersion,
		s.byzRoster,
		[]string{"spawn:" + ContractLongTermSecretID}, s.signer.Identity())
	//s.genesisMsg, err = byzcoin.DefaultGenesisMsg(byzcoin.CurrentVersion, s.byzRoster,
	//	[]string{"spawn:" + ContractWriteID,
	//		"spawn:" + ContractReadID,
	//		"spawn:" + ContractLongTermSecretID,
	//		"invoke:" + ContractLongTermSecretID + ".reshare"},
	//	s.signer.Identity())
	require.NoError(t, err)
	s.gDarc = &s.genesisMsg.GenesisDarc
	//s.genesisMsg.BlockInterval = time.Second
	s.genesisMsg.BlockInterval = 10 * time.Second

	s.cl, s.gbReply, err = byzcoin.NewLedger(s.genesisMsg, false)
	require.NoError(t, err)

	for _, svc := range s.services {
		req := &Authorize{ByzCoinID: s.cl.ID}
		_, err = svc.Authorize(req)
		require.NoError(t, err)
	}
}

func (s *ts) afterReshare(f func()) {
	for _, sv := range s.services {
		sv.afterReshare = f
	}
}

func (s *ts) waitInstID(t *testing.T, instID byzcoin.InstanceID) *byzcoin.Proof {
	var err error
	var pr *byzcoin.Proof
	for i := 0; i < 10; i++ {
		pr, err = s.cl.WaitProof(instID, s.genesisMsg.BlockInterval, nil)
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil {
		require.Fail(t, "didn't find proof")
	}
	return pr
}

func (s *ts) addWriteAndWait(t *testing.T, key []byte) *byzcoin.Proof {
	ctr, err := s.cl.GetSignerCounters(s.signer.Identity().String())
	require.NoError(t, err)

	instID := s.addWrite(t, key, ctr.Counters[0]+1)
	return s.waitInstID(t, instID)
}

func (s *ts) addWrite(t *testing.T, key []byte, ctr uint64) byzcoin.InstanceID {
	write := NewWrite(cothority.Suite, s.ltsReply.InstanceID, s.gDarc.GetBaseID(), s.ltsReply.X, key)
	writeBuf, err := protobuf.Encode(write)
	require.NoError(t, err)

	ctx := byzcoin.NewClientTransaction(byzcoin.CurrentVersion,
		byzcoin.Instruction{
			InstanceID: byzcoin.NewInstanceID(s.gDarc.GetBaseID()),
			Spawn: &byzcoin.Spawn{
				ContractID: ContractWriteID,
				Args:       byzcoin.Arguments{{Name: "write", Value: writeBuf}},
			},
			SignerCounter: []uint64{ctr},
		},
	)
	require.Nil(t, ctx.FillSignersAndSignWith(s.signer))
	_, err = s.cl.AddTransaction(ctx)
	require.NoError(t, err)
	return ctx.Instructions[0].DeriveID("")
}

func (s *ts) closeAll(t *testing.T) {
	require.Nil(t, s.cl.Close())
	s.local.CloseAll()
}

// The key might still be reconstructed at some nodes, so try one second later,
// if the first one fails.
func (s *ts) reconstructKey(t *testing.T) kyber.Scalar {
	key, err := s.reconstructKeyFunc()
	if err == nil {
		return key
	}
	time.Sleep(time.Second)
	key, err = s.reconstructKeyFunc()
	require.NoError(t, err)
	return key
}

func (s *ts) reconstructKeyFunc() (kyber.Scalar, error) {
	id := s.ltsReply.InstanceID
	var sshares []*share.PriShare
	for i := range s.services {
		for j := range s.ltsRoster.List {
			if s.services[i].ServerIdentity().Equal(s.ltsRoster.List[j]) {
				s.services[i].storage.Lock()
				if s.services[i].storage.DKS[id] != nil {
					sshares = append(sshares, s.services[i].storage.DKS[id].PriShare())
				}
				s.services[i].storage.Unlock()
			}
		}
	}
	n := len(s.ltsRoster.List)
	th := n - (n-1)/3
	if n != len(sshares) {
		return nil, xerrors.New("not correct amount of shares")
	}
	sec, err := share.RecoverSecret(cothority.Suite, sshares, th, n)
	if err != nil {
		return nil, xerrors.Errorf("while recovering secret: %v", err)
	}
	return sec, nil
}
