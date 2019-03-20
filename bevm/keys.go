package byzcoin

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/pborman/uuid"
	"go.dedis.ch/onet/v3/log"
)

//Key creation from Ethereum library
type Key struct {
	Id uuid.UUID // Version 4 "random" for unique id not derived from key data
	// to simplify lookups we also store the address
	Address common.Address
	// we only store privkey as pubkey/address can be derived from it
	// privkey in this struct is always in plaintext
	PrivateKey *ecdsa.PrivateKey
}

func GenerateKeys() (address string, privateKey string) {
	private, err := crypto.GenerateKey()
	if err != nil {
		log.Fatal(err)
	}
	addressB := crypto.PubkeyToAddress(private.PublicKey)
	address = addressB.Hex()
	log.Lvlf2("Public key : %x ",  elliptic.Marshal(crypto.S256(), private.PublicKey.X, private.PublicKey.Y))
	log.Lvl2("Address generated : ", address)
	privateKey = common.Bytes2Hex(crypto.FromECDSA(private))
	return
}

//NewKeyFromECDSA :
func NewKeyFromECDSA(privateKeyECDSA *ecdsa.PrivateKey) *Key {
	id := uuid.NewRandom()
	key := &Key{
	Id:         id,
	Address:    crypto.PubkeyToAddress(privateKeyECDSA.PublicKey),
	PrivateKey: privateKeyECDSA,
	}
return key
}