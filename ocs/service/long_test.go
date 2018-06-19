// +build long_test

package service

import (
	"sync"
	"testing"
	"time"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/ocs/darc"
	"github.com/dedis/onet/log"
	"github.com/stretchr/testify/require"
)

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
