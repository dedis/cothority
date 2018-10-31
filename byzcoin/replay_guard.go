package byzcoin

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/dedis/cothority/darc"
	"github.com/dedis/protobuf"
)

// getSignatureCounter returns 0 if the key is not set
func getSignatureCounter(st ReadOnlyStateTrie, id string) (uint64, error) {
	val, _, _, err := st.GetValues(publicVersionKey(id))
	if err == errKeyNotSet {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	ver := binary.LittleEndian.Uint64(val)
	return ver, nil
}

// setSignatureCounter is mostly for testing
func setSignatureCounter(sst *stagingStateTrie, id string, v uint64) error {
	key := publicVersionKey(id)
	verBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(verBuf, v)
	body := StateChangeBody{
		StateAction: Update,
		ContractID:  []byte{},
		Value:       verBuf,
		DarcID:      darc.ID([]byte{}),
	}
	buf, err := protobuf.Encode(&body)
	if err != nil {
		return err
	}
	return sst.Set(key, buf)
}

func incrementSignatureCounters(sst *stagingStateTrie, sigs []darc.Signature) (StateChanges, error) {
	var scs StateChanges
	for _, sig := range sigs {
		id := sig.Signer.String()
		ver, err := getSignatureCounter(sst, id)
		if err != nil {
			return scs, err
		}
		verBuf := make([]byte, 8)
		binary.LittleEndian.PutUint64(verBuf, ver+1)
		scs = append(scs, StateChange{
			StateAction: Update,
			InstanceID:  publicVersionKey(id),
			ContractID:  []byte{},
			Value:       verBuf,
			DarcID:      darc.ID([]byte{}),
		})
	}
	return scs, sst.StoreAll(scs)
}

func verifySignatureCounters(st ReadOnlyStateTrie, counters []uint64, sigs []darc.Signature) error {
	if len(counters) != len(sigs) {
		return errors.New("lengths of the counters and signatures are not the same")
	}
	for i, counter := range counters {
		if !sigs[i].Signer.PrimaryIdentity() {
			return errors.New("not a primary identity")
		}
		id := sigs[i].Signer.String()
		c, err := getSignatureCounter(st, id)
		if err != nil {
			return err
		}
		if counter != c+1 {
			return fmt.Errorf("for pk %s, got version %v, but need %v", id, counter, c+1)
		}
	}
	return nil
}

func publicVersionKey(id string) []byte {
	key := sha256.Sum256([]byte("version_" + id))
	return key[:]
}
