package swupdate

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"time"

	"errors"

	"crypto"

	"github.com/dedis/cothority/log"
	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/armor"
	"golang.org/x/crypto/openpgp/packet"
)

/*
* PGP - wrappers
 */

// Yay - security!
const PGPBits = 512

type PGP struct {
	Public  *packet.PublicKey
	Private *packet.PrivateKey
}

func NewPGP() *PGP {
	if PGPBits < 2048 {
		log.Warn("Please adjust PGPBits to 2048 bits or more.")
	}
	key, err := rsa.GenerateKey(rand.Reader, PGPBits)
	log.ErrFatal(err)
	return &PGP{
		Public:  packet.NewRSAPublicKey(time.Now(), &key.PublicKey),
		Private: packet.NewRSAPrivateKey(time.Now(), key),
	}
}

func NewPGPPublic(public string) *PGP {
	return &PGP{Public: DecodePublic(public)}
}

func (p *PGP) Sign(data []byte) (string, error) {
	if p.Private == nil {
		return "", errors.New("No private key defined.")
	}
	in := bytes.NewBuffer(data)
	out := &bytes.Buffer{}
	err := openpgp.ArmoredDetachSign(out, p.Entity(), in, nil)

	return out.String(), err
}

func (p *PGP) Verify(data []byte, sigStr string) error {
	// open ascii armored public key
	in := bytes.NewBufferString(sigStr)

	block, err := armor.Decode(in)
	log.ErrFatal(err)

	if block.Type != openpgp.SignatureType {
		log.Fatal("Invalid signature file")
	}

	reader := packet.NewReader(block.Body)
	pkt, err := reader.Next()
	log.ErrFatal(err)

	sig, ok := pkt.(*packet.Signature)
	if !ok {
		log.Fatal("Invalid signature")
	}

	hash := sig.Hash.New()
	hash.Write(data)

	return p.Public.VerifySignature(hash, sig)
}

func (p *PGP) ArmorPrivate() string {
	priv := &bytes.Buffer{}
	wPriv, err := armor.Encode(priv, openpgp.PrivateKeyType, make(map[string]string))
	log.ErrFatal(err)
	log.ErrFatal(p.Private.Serialize(wPriv))
	log.ErrFatal(wPriv.Close())
	return priv.String()
}

func (p *PGP) ArmorPublic() string {
	pub := &bytes.Buffer{}
	wPub, err := armor.Encode(pub, openpgp.PublicKeyType, make(map[string]string))
	log.ErrFatal(err)

	log.ErrFatal(p.Public.Serialize(wPub))
	log.ErrFatal(wPub.Close())
	return pub.String()
}

func (p *PGP) Entity() *openpgp.Entity {
	config := packet.Config{
		DefaultHash:            crypto.SHA256,
		DefaultCipher:          packet.CipherAES256,
		DefaultCompressionAlgo: packet.CompressionZLIB,
		CompressionConfig: &packet.CompressionConfig{
			Level: 9,
		},
		RSABits: PGPBits,
	}
	currentTime := config.Now()
	uid := packet.NewUserId("", "", "")

	e := openpgp.Entity{
		PrimaryKey: p.Public,
		PrivateKey: p.Private,
		Identities: make(map[string]*openpgp.Identity),
	}
	isPrimaryId := false

	e.Identities[uid.Id] = &openpgp.Identity{
		Name:   uid.Name,
		UserId: uid,
		SelfSignature: &packet.Signature{
			CreationTime: currentTime,
			SigType:      packet.SigTypePositiveCert,
			PubKeyAlgo:   packet.PubKeyAlgoRSA,
			Hash:         config.Hash(),
			IsPrimaryId:  &isPrimaryId,
			FlagsValid:   true,
			FlagSign:     true,
			FlagCertify:  true,
			IssuerKeyId:  &e.PrimaryKey.KeyId,
		},
	}

	keyLifetimeSecs := uint32(86400 * 365)

	e.Subkeys = make([]openpgp.Subkey, 1)
	e.Subkeys[0] = openpgp.Subkey{
		PublicKey:  p.Public,
		PrivateKey: p.Private,
		Sig: &packet.Signature{
			CreationTime:              currentTime,
			SigType:                   packet.SigTypeSubkeyBinding,
			PubKeyAlgo:                packet.PubKeyAlgoRSA,
			Hash:                      config.Hash(),
			PreferredHash:             []uint8{8}, // SHA-256
			FlagsValid:                true,
			FlagEncryptStorage:        true,
			FlagEncryptCommunications: true,
			IssuerKeyId:               &e.PrimaryKey.KeyId,
			KeyLifetimeSecs:           &keyLifetimeSecs,
		},
	}
	return &e
}

func DecodePrivate(priv string) *packet.PrivateKey {
	// open ascii armored private key
	in := bytes.NewBufferString(priv)
	block, err := armor.Decode(in)
	log.ErrFatal(err)

	if block.Type != openpgp.PrivateKeyType {
		log.Fatal("Invalid private key file")
	}

	reader := packet.NewReader(block.Body)
	pkt, err := reader.Next()
	log.ErrFatal(err)

	key, ok := pkt.(*packet.PrivateKey)
	if !ok {
		log.Fatal("Invalid private key")
	}
	return key
}

func DecodePublic(pub string) *packet.PublicKey {
	in := bytes.NewBufferString(pub)
	block, err := armor.Decode(in)
	log.ErrFatal(err)

	if block.Type != openpgp.PublicKeyType {
		log.Fatal("Invalid private key file")
	}

	reader := packet.NewReader(block.Body)
	pkt, err := reader.Next()
	log.ErrFatal(err)

	key, ok := pkt.(*packet.PublicKey)
	if !ok {
		log.Fatal("Invalid public key")
	}
	return key
}
