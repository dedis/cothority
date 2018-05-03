package service

import (
	"sync"
	"testing"
	"time"

	bolt "github.com/coreos/bbolt"
	"github.com/dedis/cothority"
	"github.com/dedis/cothority/ocs/darc"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/kyber/suites"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"github.com/dedis/protobuf"
	"github.com/stretchr/testify/require"
)

var tSuite = suites.MustFind("Ed25519")

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestService_proof(t *testing.T) {
	if testing.Short() {
		t.Skip("flaky test on travis - skip")
	}
	o := createOCS(t)
	defer o.local.CloseAll()

	// Creating a write request
	encKey := []byte{1, 2, 3}
	write := NewWrite(cothority.Suite, o.sc.OCS.Hash, o.sc.X, o.readers, encKey)
	write.Data = []byte{}
	sigPath := darc.NewSignaturePath([]*darc.Darc{o.readers}, *o.writerI, darc.User)
	sig, err := darc.NewDarcSignature(write.Reader.GetID(), sigPath, o.writer)
	require.Nil(t, err)
	wr, err := o.service.WriteRequest(&WriteRequest{
		OCS:       o.sc.OCS.Hash,
		Write:     *write,
		Signature: *sig,
		Readers:   o.readers,
	})
	require.Nil(t, err)

	// Making a read request
	sigRead, err := darc.NewDarcSignature(wr.SB.Hash, sigPath, o.writer)
	require.Nil(t, err)
	read := Read{
		DataID:    wr.SB.Hash,
		Signature: *sigRead,
	}
	rr, err := o.service.ReadRequest(&ReadRequest{
		OCS:  o.sc.OCS.Hash,
		Read: read,
	})
	require.Nil(t, err)

	// Decoding the file
	symEnc, err := o.service.DecryptKeyRequest(&DecryptKeyRequest{
		Read: rr.SB.Hash,
	})
	require.Nil(t, err)
	priv, err := o.writer.GetPrivate()
	require.Nil(t, err)
	sym, err2 := DecodeKey(cothority.Suite, o.sc.X, write.Cs, symEnc.XhatEnc, priv)
	require.Nil(t, err2)
	require.Equal(t, encKey, sym)

	// Create a wrong Decryption request by abusing skipchain's database and
	// writing a wrong reader public key to the OCS-data.
	ocsd := NewOCS(rr.SB.Data)
	ocsd.Read.Signature.SignaturePath.Signer.Ed25519.Point = cothority.Suite.Point()
	rr.SB.Data, err = protobuf.Encode(ocsd)
	require.Nil(t, err)
	val, err := network.Marshal(rr.SB)
	require.Nil(t, err)
	bucket := skipchain.ServiceName + "_skipblocks"
	for _, s := range o.services {
		db := s.(*Service).db()
		require.Nil(t, db.Update(func(tx *bolt.Tx) error {
			return tx.Bucket([]byte(bucket)).Put(rr.SB.Hash, val)
		}))
	}
	symEnc, err = o.service.DecryptKeyRequest(&DecryptKeyRequest{
		Read: rr.SB.Hash,
	})
	require.NotNil(t, err)

	// GetReadRequests
	requests, err := o.service.GetReadRequests(&GetReadRequests{
		Start: wr.SB.Hash,
		Count: 0,
	})
	require.Nil(t, err)
	require.Equal(t, 1, len(requests.Documents))
}

func TestService_GetDarcPath(t *testing.T) {
	o := createOCS(t)
	defer o.local.CloseAll()

	w := &darc.Darc{}
	wDarcID := darc.NewIdentityDarc(w.GetID())

	newReader := o.readers.Copy()
	newReader.AddUser(wDarcID)
	path := darc.NewSignaturePath([]*darc.Darc{o.readers}, *o.writerI, darc.Owner)
	err := newReader.SetEvolution(o.readers, path, o.writer)
	require.Nil(t, err)

	_, err = o.service.UpdateDarc(&UpdateDarc{
		OCS:  o.sc.OCS.SkipChainID(),
		Darc: *w,
	})
	require.Nil(t, err)
	_, err = o.service.UpdateDarc(&UpdateDarc{
		OCS:  o.sc.OCS.SkipChainID(),
		Darc: *newReader,
	})
	require.Nil(t, err)

	log.Lvl1("Searching for wrong role")
	request := &GetDarcPath{
		OCS:        o.sc.OCS.SkipChainID(),
		BaseDarcID: o.readers.GetID(),
		Identity:   *wDarcID,
		Role:       int(darc.Owner),
	}
	reply, err := o.service.GetDarcPath(request)
	require.NotNil(t, err)

	log.Lvl1("Searching for correct role")
	request.Role = int(darc.User)
	reply, err = o.service.GetDarcPath(request)
	require.Nil(t, err)
	require.NotNil(t, reply.Path)
	require.NotEqual(t, 0, len(*reply.Path))
}

func TestService_UpdateDarcOffline(t *testing.T) {
	o := createOCS(t)
	defer o.local.CloseAll()

	latestReader := o.readers.Copy()
	var newSigner *darc.Signer
	for i := 0; i < 10; i++ {
		log.Lvl1("Adding darc", i)
		w := darc.NewSignerEd25519(nil, nil)
		newReader := latestReader.Copy()
		newReader.AddUser(w.Identity())
		if newSigner != nil {
			newReader.RemoveUser(newSigner.Identity())
		}
		err := newReader.SetEvolution(latestReader, nil, o.writer)
		require.Nil(t, err)

		_, err = o.service.UpdateDarc(&UpdateDarc{
			OCS:  o.sc.OCS.SkipChainID(),
			Darc: *newReader,
		})
		require.Nil(t, err)

		_, err = o.service.GetDarcPath(&GetDarcPath{
			OCS:        o.sc.OCS.SkipChainID(),
			BaseDarcID: o.readers.GetID(),
			Identity:   *w.Identity(),
			Role:       int(darc.User),
		})
		require.Nil(t, err)

		latestReader = newReader
		newSigner = w
	}
}

func TestService_UpdateDarcOnline(t *testing.T) {
	if testing.Short() {
		t.Skip("adding 100 darcs takes a lot of time")
	}
	o := createOCS(t)
	defer o.local.CloseAll()

	latestReader := o.readers.Copy()
	var newSigner *darc.Signer
	for i := 0; i < 100; i++ {
		log.Lvl1("Adding darc", i)
		w := darc.NewSignerEd25519(nil, nil)
		newReader := latestReader.Copy()
		newReader.AddUser(w.Identity())
		if newSigner != nil {
			newReader.RemoveUser(newSigner.Identity())
		}
		err := newReader.SetEvolutionOnline(latestReader, o.writer)
		require.Nil(t, err)

		_, err = o.service.UpdateDarc(&UpdateDarc{
			OCS:  o.sc.OCS.SkipChainID(),
			Darc: *newReader,
		})
		require.Nil(t, err)

		buf, err := network.Marshal(newReader)
		require.Nil(t, err)
		log.Lvl2("Size of darc:", len(buf))

		latestReader = newReader
		newSigner = w
	}
}

func TestStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Not stress-testing on travis")
	}

	nbrThreads := 20
	nbrLoops := 20

	o := createOCS(t)
	defer o.local.CloseAll()

	wg := &sync.WaitGroup{}
	wg.Add(nbrThreads)
	addDarc := sync.Mutex{}
	latestReader := o.readers.Copy()
	var newSigner *darc.Signer
	doneChan := make(chan bool)
	errChan := make(chan error)
	done := make([]bool, nbrThreads)
	mutex := sync.Mutex{}

	// Bits: 0 - update darc; 1 - write; 2 - read; 3 - re-encrypt;
	// 4 - getDarcPath
	actions := 0x1f
	count := 0
	for thread := 0; thread < nbrThreads; thread++ {
		go func(n int) {
			for loop := 0; loop < nbrLoops; loop++ {
				start := time.Now()
				mutex.Lock()
				log.Print("Run:", count)
				count++
				mutex.Unlock()
				if actions&0x1 != 0 {
					// Adding a new darc
					for {
						addDarc.Lock()
						log.Lvlf1("Loop %d in thread %d: PrepareDarc", loop, n)
						w := darc.NewSignerEd25519(nil, nil)
						newReader := latestReader.Copy()
						newReader.AddUser(w.Identity())
						if newSigner != nil {
							newReader.RemoveUser(newSigner.Identity())
						}
						err := newReader.SetEvolutionOnline(latestReader, o.writer)
						if err != nil {
							errChan <- err
						}

						log.Lvlf1("Loop %d in thread %d: UpdateDarc", loop, n)
						_, err = o.service.UpdateDarc(&UpdateDarc{
							OCS:  o.sc.OCS.SkipChainID(),
							Darc: *newReader,
						})
						if err != nil {
							log.Lvlf2("Couldn't store darc: %s - trying again", err.Error())
						} else {
							latestReader = newReader
							newSigner = w
							addDarc.Unlock()
							break
						}
						addDarc.Unlock()
					}
				}

				if actions&0x10 != 0 {
					addDarc.Lock()
					latestID := (*latestReader.Users)[len(*latestReader.Users)-1]
					addDarc.Unlock()
					log.Lvlf1("Loop %d in thread %d: GetDarcPath", loop, n)
					_, err := o.service.GetDarcPath(&GetDarcPath{
						OCS:        o.sc.OCS.SkipChainID(),
						BaseDarcID: o.readers.GetID(),
						Identity:   *latestID,
						Role:       int(darc.User),
					})
					if err != nil {
						errChan <- err
					}
				}

				if actions&0x2 != 0 {
					// Creating a write request
					log.Lvlf1("Loop %d in thread %d: Write", loop, n)
					encKey := []byte{1, 2, 3}
					write := NewWrite(cothority.Suite, o.sc.OCS.Hash, o.sc.X, o.readers, encKey)
					write.Data = []byte{}
					sigPath := darc.NewSignaturePath([]*darc.Darc{o.readers}, *o.writerI, darc.User)
					sig, err := darc.NewDarcSignature(write.Reader.GetID(), sigPath, o.writer)
					if err != nil {
						errChan <- err
					}
					wr, err := o.service.WriteRequest(&WriteRequest{
						OCS:       o.sc.OCS.Hash,
						Write:     *write,
						Signature: *sig,
						Readers:   o.readers,
					})
					if err != nil {
						errChan <- err
					}
					require.NotNil(t, wr)

					if actions&0x4 != 0 {
						// Making a read request
						log.Lvlf1("Loop %d in thread %d: Read", loop, n)
						sigRead, err := darc.NewDarcSignature(wr.SB.Hash, sigPath, o.writer)
						if err != nil {
							errChan <- err
						}
						read := Read{
							DataID:    wr.SB.Hash,
							Signature: *sigRead,
						}
						rr, err := o.service.ReadRequest(&ReadRequest{
							OCS:  o.sc.OCS.Hash,
							Read: read,
						})
						if err != nil {
							errChan <- err
						}

						if actions&0x8 != 0 {
							// Decoding the file
							log.Lvlf1("Loop %d in thread %d: DecryptKey", loop, n)
							symEnc, err := o.service.DecryptKeyRequest(&DecryptKeyRequest{
								Read: rr.SB.Hash,
							})
							if err != nil {
								errChan <- err
							}
							priv, err := o.writer.GetPrivate()
							if err != nil {
								errChan <- err
							}
							sym, err2 := DecodeKey(cothority.Suite, o.sc.X, write.Cs, symEnc.XhatEnc, priv)
							if err2 != nil {
								errChan <- err2
							}
							require.Equal(t, encKey, sym)
						}
					}
				}
				log.LLvl5("Timing TestRun [ms]:", time.Since(start).Nanoseconds()/1e6/int64(nbrThreads))
			}
			wg.Done()
			mutex.Lock()
			done[n] = true
			mutex.Unlock()
		}(thread)
	}
	go func() {
		wg.Wait()
		doneChan <- true
	}()
	for {
		select {
		case <-doneChan:
			log.Lvl1("Stress-test done")
			return
		case err := <-errChan:
			log.Fatal("Error in stress-test:", err)
		case <-time.After(10 * time.Second):
			log.Lvl1("** Thread-list waiting:")
			mutex.Lock()
			for i := range done {
				if !done[i] {
					log.Lvl1("Thread", i, "still working")
				}
			}
			mutex.Unlock()
		}
	}
}

type ocsStruct struct {
	local    *onet.LocalTest
	services []onet.Service
	service  *Service
	writer   *darc.Signer
	writerI  *darc.Identity
	readers  *darc.Darc
	sc       *CreateSkipchainsReply
}

func createOCS(t *testing.T) *ocsStruct {
	o := &ocsStruct{
		local: onet.NewTCPTest(tSuite),
	}
	// generate 5 hosts, they don't connect, they process messages, and they
	// don't register the tree or entitylist
	hosts, roster, _ := o.local.GenTree(5, true)

	o.services = o.local.GetServices(hosts, templateID)
	o.service = o.services[0].(*Service)

	// Initializing onchain-secrets skipchain
	o.writer = darc.NewSignerEd25519(nil, nil)
	o.writerI = o.writer.Identity()
	o.readers = darc.NewDarc(nil, nil, nil)
	o.readers.AddOwner(o.writerI)
	o.readers.AddUser(o.writerI)
	var err error
	o.sc, err = o.service.CreateSkipchains(&CreateSkipchainsRequest{
		Roster:  *roster,
		Writers: *o.readers,
	})
	require.Nil(t, err)
	return o
}
