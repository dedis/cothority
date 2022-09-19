package main

import (
	"github.com/BurntSushi/toml"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/calypso/pqots"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/cothority/v3/darc/expression"
	"go.dedis.ch/kyber/v3/share"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/simul/monitor"
	"golang.org/x/xerrors"
	"time"
)

func init() {
	onet.SimulationRegister("PQOTS", NewPQOTSService)
}

type SimulationService struct {
	onet.SimulationBFTree
	BlockInterval int
	BlockWait     int
}

type ByzcoinData struct {
	Signer darc.Signer
	Ctr    uint64
	Roster *onet.Roster
	Cl     *byzcoin.Client
	GMsg   *byzcoin.CreateGenesisBlock
	GDarc  darc.Darc
}

func NewPQOTSService(config string) (onet.Simulation, error) {
	es := &SimulationService{}
	_, err := toml.Decode(config, es)
	if err != nil {
		return nil, err
	}
	return es, nil
}
func (s *SimulationService) Setup(dir string,
	hosts []string) (*onet.SimulationConfig, error) {
	sc := &onet.SimulationConfig{}
	s.CreateRoster(sc, hosts, 2000)
	err := s.CreateTree(sc)
	if err != nil {
		return nil, err
	}
	return sc, nil
}

// Node can be used to initialize each node before it will be run
// by the server. Here we call the 'Node'-method of the
// SimulationBFTree structure which will load the roster- and the
// tree-structure to speed up the first round.
func (s *SimulationService) Node(config *onet.SimulationConfig) error {
	index, _ := config.Roster.Search(config.Server.ServerIdentity.ID)
	if index < 0 {
		log.Fatal("Didn't find this node in roster")
	}
	log.Lvl3("Initializing node-index", index)
	return s.SimulationBFTree.Node(config)
}

func setupByzcoin(r *onet.Roster, interval int) (data ByzcoinData, err error) {
	data.Signer = darc.NewSignerEd25519(nil, nil)
	data.Ctr = uint64(1)
	data.GMsg, err = byzcoin.DefaultGenesisMsg(byzcoin.CurrentVersion, r,
		nil, data.Signer.Identity())
	if err != nil {
		log.Errorf("cannot create genesis message: %v", err)
		return
	}
	data.GMsg.BlockInterval = time.Duration(interval) * time.Millisecond
	data.GDarc = data.GMsg.GenesisDarc
	err = data.GDarc.Verify(true)
	if err != nil {
		//log.Errorf("cannot verify genesis darc: %v", err)
		return
	}
	data.Cl, _, err = byzcoin.NewLedger(data.GMsg, false)
	if err != nil {
		//log.Errorf("cannot create new ledger: %v", err)
		return
	}
	return
}

func setupDarcs() (writer darc.Signer, reader darc.Signer, wDarc *darc.Darc,
	err error) {
	writer = darc.NewSignerEd25519(nil, nil)
	reader = darc.NewSignerEd25519(nil, nil)
	wDarc = darc.NewDarc(darc.InitRules([]darc.Identity{writer.Identity()},
		[]darc.Identity{writer.Identity()}), []byte("Writer"))
	err = wDarc.Rules.AddRule(darc.Action("spawn:"+pqots.ContractPQOTSWriteID),
		expression.InitOrExpr(writer.Identity().String()))
	if err != nil {
		//log.Errorf("cannot add rule: %v", err)
		return
	}
	err = wDarc.Rules.AddRule(darc.Action("spawn:"+pqots.ContractPQOTSReadID),
		expression.InitOrExpr(reader.Identity().String()))
	if err != nil {
		//log.Errorf("cannot add rule: %v", err)
		return
	}
	return
}

func (s *SimulationService) runSimulation(config *onet.SimulationConfig) error {
	n := len(config.Roster.List)
	f := (n - 1) / 3
	thr := 2*f + 1
	if 3*f+1 != n {
		return xerrors.New("error computing threshold")
	}
	for round := 0; round < s.Rounds; round++ {
		byz, err := setupByzcoin(config.Roster, s.BlockInterval)
		if err != nil {
			return err
		}
		cl := pqots.NewClient(byz.Cl)
		writer, reader, wDarc, err := setupDarcs()
		if err != nil {
			return err
		}
		_, err = cl.SpawnDarc(byz.Signer, byz.Ctr, byz.GDarc, *wDarc, 3)
		if err != nil {
			return err
		}
		byz.Ctr++

		prepWr := monitor.NewTimeMeasure("prepWr")
		poly := pqots.GenerateSSPoly(f + 1)
		shares, rands, commitments, err := pqots.GenerateCommitments(poly, n)
		if err != nil {
			return err
		}

		mesg := []byte("Hello world from pq-ots!")
		ctxt, ctxtHash, err := pqots.Encrypt(poly.Secret(), mesg)
		if err != nil {
			return err
		}

		wr := pqots.Write{
			Commitments: commitments,
			Publics:     config.Roster.Publics(),
			CtxtHash:    ctxtHash,
		}
		prepWr.Record()

		sigs := make(map[int][]byte)
		replies := cl.VerifyWriteAll(config.Roster, &wr, shares, rands)
		for i, r := range replies {
			sigs[i] = r.Sig
		}
		wReply, err := cl.AddWrite(&wr, sigs, thr, writer, 1, *wDarc, 10)
		if err != nil {
			return err
		}
		prWr, err := cl.WaitProof(wReply.InstanceID, time.Second, nil)
		if err != nil {
			return err
		}
		rReply, err := cl.AddRead(prWr, reader, 1, 10)
		if err != nil {
			return err
		}
		prRe, err := cl.WaitProof(rReply.InstanceID, time.Second, nil)

		dkReply, err := cl.DecryptKey(&pqots.DecryptKeyRequest{
			Roster: config.Roster,
			Read:   *prRe,
			Write:  *prWr,
		})
		if err != nil {
			return err
		}

		//recShares := make([]share.PriShare, len(roster.List))
		decShares := pqots.ElGamalDecrypt(cothority.Suite, reader.Ed25519.Secret,
			dkReply.Reencryptions)

		recSecret, err := share.RecoverSecret(cothority.Suite, decShares, thr, n)
		if err != nil {
			return err
		}
		ptxt, err := pqots.Decrypt(recSecret, ctxt)
		if err != nil {
			return err
		}
		log.Infof("Plaintext message is: %s", string(ptxt))
	}
	return nil
}

func (s *SimulationService) Run(config *onet.SimulationConfig) error {
	err := s.runSimulation(config)
	if err != nil {
		panic(err)
	}
	return nil
}
