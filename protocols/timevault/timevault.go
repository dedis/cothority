// Package timevault provides a mechanism to encrypt a message with a shared
// secret key using ElGamal and provide the decrypted message on request after
// an initially specified timer has expired.
package timevault

import (
	"fmt"
	"sync"
	"time"

	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"github.com/dedis/crypto/poly"
)

func init() {
	sda.ProtocolRegisterName("TimeVault", NewTimeVault)
}

// SID is the type for shared secret identifiers
type SID string

// Base identifier for TimeVault shared secrets.
const (
	TVSS SID = "TVSS"
)

// TimeVault is the main protocol struct and implements the
// sda.ProtocolInstance interface.
type TimeVault struct {
	*sda.TreeNodeInstance
	keyPair          *config.KeyPair
	pubKeys          []abstract.Point
	info             poly.Threshold
	secrets          map[SID]*Secret
	recoveredSecrets map[SID]*RecoveredSecret
	secretsDone      chan bool
}

// TODO: generalise the shared secret struct from JVSS and introduce timed secret

// Secret contains all information on shared secrets.
type Secret struct {
	secret   *poly.SharedSecret // Shared secret
	receiver *poly.Receiver     // Receiver to aggregate deals
	deals    map[int]*poly.Deal // Buffer for deals
	numConfs int                // Number of collected confirmations that shared secrets are ready
	mtx      sync.Mutex         // Mutex to sync access to numConfs
	duration time.Duration      // Duration after which the timer expires
	expired  bool               // Indicator if timer has expired
}

// RecoveredSecret contains all information necessary to reconstruct a shared secret.
type RecoveredSecret struct {
	priShares   *poly.PriShares      // The secret shares
	numShares   int                  // Number of secret shares
	mtx         sync.Mutex           // Mutex to sync access to numShares
	secretsChan chan abstract.Secret // Channel to communicate reocvered shared secret
}

// NewTimeVault creates a new TimeVault protocol and returns it.
func NewTimeVault(node *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {

	kp := &config.KeyPair{Suite: node.Suite(), Public: node.Public(), Secret: node.Private()}
	n := len(node.List())
	pk := make([]abstract.Point, n)
	for i, tn := range node.List() {
		pk[i] = tn.Entity.Public
	}

	// NOTE: T <= R <= N (for simplicity we use T = R = N; might change later)
	info := poly.Threshold{T: n, R: n, N: n}

	tv := &TimeVault{
		TreeNodeInstance: node,
		keyPair:          kp,
		pubKeys:          pk,
		info:             info,
		secrets:          make(map[SID]*Secret),
		secretsDone:      make(chan bool, 1),
	}

	// Setup message handlers
	h := []interface{}{
		tv.handleSecInit,
		tv.handleSecConf,
		tv.handleRevInit,
		tv.handleRevShare,
	}
	err := tv.RegisterHandlers(h...)

	return tv, err
}

// Start initiates the TimeVault protocol.
func (tv *TimeVault) Start() error {
	return nil
}

// Seal generates a shared secret and starts a timer indicating when the shared
// secret can be released.
func (tv *TimeVault) Seal(duration time.Duration) (SID, abstract.Point, error) {

	// Generate shared secret
	sid := SID(fmt.Sprintf("%s%d", TVSS, tv.Index()))
	if err := tv.initSecret(sid, duration); err != nil {
		return "", nil, err
	}
	<-tv.secretsDone

	// TODO: create a co-signature of the public key using the timevault peers

	return sid, tv.secrets[sid].secret.Pub.SecretCommit(), nil
}

// Open checks if the timer of a shared secret has already expired and if so
// recovers and returns the secret. If the timer has not yet expired, the
// function returns an error.
func (tv *TimeVault) Open(sid SID) (abstract.Secret, error) {

	secret, ok := tv.secrets[sid]
	if !ok {
		return nil, fmt.Errorf("Error, shared secret does not exist")
	}

	secret.mtx.Lock()
	defer secret.mtx.Unlock()
	if !secret.expired {
		return nil, fmt.Errorf("Error, secret has not yet expired")
	}

	// Setup list of recovered secrets if necessary
	if tv.recoveredSecrets == nil {
		tv.recoveredSecrets = make(map[SID]*RecoveredSecret)
	}

	rs := &RecoveredSecret{
		priShares:   &poly.PriShares{},
		numShares:   0,
		secretsChan: make(chan abstract.Secret, 1),
	}
	rs.priShares.Empty(tv.keyPair.Suite, tv.info.T, tv.info.N)
	rs.priShares.SetShare(tv.secrets[sid].secret.Index, *tv.secrets[sid].secret.Share)
	rs.numShares++
	tv.recoveredSecrets[sid] = rs

	// Start process to reveal shares
	if err := tv.Broadcast(&RevInitMsg{Src: tv.TreeNodeInstance.Index(), SID: sid}); err != nil {
		return nil, err
	}
	x := <-rs.secretsChan

	return x, nil
}

//func (tv *TimeVault) newSID(base SID) (SID, error) {
//	buf := new(bytes.Buffer)
//	timestamp, err := time.Now().MarshalBinary()
//	if err != nil {
//		return "", err
//	}
//	if err := binary.Write(buf, binary.LittleEndian, []byte(base)); err != nil {
//		return "", err
//	}
//	if err := binary.Write(buf, binary.LittleEndian, uint32(tv.TreeNodeInstance.Index())); err != nil {
//		return "", err
//	}
//	if err := binary.Write(buf, binary.LittleEndian, timestamp); err != nil {
//		return "", err
//	}
//	return SID(hex.EncodeToString(abstract.Sum(tv.keyPair.Suite, buf.Bytes()))), nil
//}

func (tv *TimeVault) initSecret(sid SID, duration time.Duration) error {

	// Initialise shared secret of given type if necessary
	if _, ok := tv.secrets[sid]; !ok {
		dbg.Lvl2(fmt.Sprintf("Node %d: Initialising %s shared secret", tv.TreeNodeInstance.Index(), sid))
		sec := &Secret{
			receiver: poly.NewReceiver(tv.keyPair.Suite, tv.info, tv.keyPair),
			deals:    make(map[int]*poly.Deal),
			numConfs: 0,
			duration: duration,
			expired:  false,
		}
		tv.secrets[sid] = sec
	}

	secret := tv.secrets[sid]

	// Initialise and broadcast our deal if necessary
	if len(secret.deals) == 0 {
		kp := config.NewKeyPair(tv.keyPair.Suite)
		deal := new(poly.Deal).ConstructDeal(kp, tv.keyPair, tv.info.T, tv.info.R, tv.pubKeys)
		dbg.Lvl2(fmt.Sprintf("Node %d: Initialising %v deal", tv.TreeNodeInstance.Index(), sid))
		secret.deals[tv.TreeNodeInstance.Index()] = deal
		db, _ := deal.MarshalBinary()
		msg := &SecInitMsg{
			Src:      tv.TreeNodeInstance.Index(),
			SID:      sid,
			Deal:     db,
			Duration: duration,
		}
		if err := tv.Broadcast(msg); err != nil {
			dbg.Warn("Broadcast failed", err)
		}
	}
	return nil
}

func (tv *TimeVault) finaliseSecret(sid SID) error {
	secret, ok := tv.secrets[sid]
	if !ok {
		return fmt.Errorf("Error, shared secret does not exist")
	}

	dbg.Lvl2(fmt.Sprintf("Node %d: %s deals %d/%d", tv.TreeNodeInstance.Index(), sid, len(secret.deals), len(tv.TreeNodeInstance.List())))

	if len(secret.deals) == tv.info.T {

		for _, deal := range secret.deals {
			if _, err := secret.receiver.AddDeal(tv.TreeNodeInstance.Index(), deal); err != nil {
				return err
			}
		}

		sec, err := secret.receiver.ProduceSharedSecret()
		if err != nil {
			return err
		}
		secret.secret = sec
		secret.mtx.Lock()
		secret.numConfs++
		secret.mtx.Unlock()
		dbg.Lvl2(fmt.Sprintf("Node %d: %v created", tv.TreeNodeInstance.Index(), sid))

		// Broadcast that we have finished setting up our shared secret
		msg := &SecConfMsg{
			Src: tv.TreeNodeInstance.Index(),
			SID: sid,
		}
		if err := tv.Broadcast(msg); err != nil {
			dbg.Warn("Broadcast failed", err)
		}

		// Start timer for revealing secret
		timer := time.NewTimer(secret.duration)
		go func() {
			<-timer.C
			secret.mtx.Lock()
			secret.expired = true
			secret.mtx.Unlock()
		}()
	}
	return nil
}
