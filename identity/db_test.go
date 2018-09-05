package identity

import (
	"testing"

	"github.com/dedis/cothority"
	"github.com/dedis/cothority/skipchain"
	"github.com/dedis/kyber/util/key"
	"github.com/dedis/onet/network"
	"github.com/stretchr/testify/require"
)

func init() {
	network.RegisterMessages(&storage0a{}, &storage0b{})
}

func TestLoadVersion(t *testing.T) {
	ml := myLoader{
		data: make(map[string][]byte),
	}

	// Test that storage0a is correctly recognized
	s0a := &storage0a{
		Identities: map[string]*idBlock0{"abc": &idBlock0{
			Latest: &Data{
				Threshold: 10,
			},
			LatestSkipblock: &skipchain.SkipBlock{
				SkipBlockFix: &skipchain.SkipBlockFix{
					Height: 3,
				},
			},
		}},
		OldSkipchainKey:  cothority.Suite.Scalar().One(),
		SkipchainKeyPair: key.NewKeyPair(cothority.Suite),
		Auth:             &authData1{},
	}
	s0aBuf, err := network.Marshal(s0a)
	require.Nil(t, err)
	ml.data[string(storageKey)] = s0aBuf

	storage, err := updateFrom0(ml, 1)
	require.Nil(t, err)
	require.True(t, s0a.SkipchainKeyPair.Public.Equal(storage.SkipchainKeyPair.Public))
	require.Equal(t, s0a.Identities["abc"].Latest.Threshold,
		storage.Identities["abc"].Latest.Threshold)
	require.Equal(t, s0a.Identities["abc"].LatestSkipblock.Height,
		storage.Identities["abc"].LatestSkipblock.Height)

	// Test that storage0b is correctly recognized
	s0b := &storage0b{
		Identities: map[string]*IDBlock{"abc": &IDBlock{
			Latest: &Data{
				Threshold: 10,
			},
			LatestSkipblock: &skipchain.SkipBlock{
				SkipBlockFix: &skipchain.SkipBlockFix{
					Height: 3,
				},
			},
		}},
		OldSkipchainKey:  cothority.Suite.Scalar().One(),
		SkipchainKeyPair: key.NewKeyPair(cothority.Suite),
		Auth:             &authData1{},
	}
	s0bBuf, err := network.Marshal(s0b)
	require.Nil(t, err)
	ml.data[string(storageKey)] = s0bBuf
	ml.SaveVersion(0)

	storage, err = updateFrom0(ml, 1)
	require.Nil(t, err)
	require.True(t, s0b.SkipchainKeyPair.Public.Equal(storage.SkipchainKeyPair.Public))
	require.Equal(t, s0b.Identities["abc"].Latest.Threshold,
		storage.Identities["abc"].Latest.Threshold)
	require.Equal(t, s0b.Identities["abc"].LatestSkipblock.Height,
		storage.Identities["abc"].LatestSkipblock.Height)
}

type myLoader struct {
	data map[string][]byte
}

func (ml myLoader) Load(key []byte) (interface{}, error) {
	_, i, err := network.Unmarshal(ml.data[string(key)], cothority.Suite)
	return i, err
}

func (ml myLoader) LoadRaw(key []byte) ([]byte, error) {
	return ml.data[string(key)], nil
}

func (ml myLoader) LoadVersion() (int, error) {
	v, ok := ml.data["version"]
	if !ok {
		return 0, nil
	}
	return int(v[0]), nil
}

func (ml myLoader) SaveVersion(v int) error {
	ml.data["version"] = []byte{byte(v)}
	return nil
}
