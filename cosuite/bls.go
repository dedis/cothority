package cosuite

import (
	"hash"

	"go.dedis.ch/kyber/v4"
	"go.dedis.ch/kyber/v4/pairing"
	"go.dedis.ch/kyber/v4/sign"
	"go.dedis.ch/kyber/v4/sign/bls"
	"go.dedis.ch/kyber/v4/util/key"
	"go.dedis.ch/onet/v4/ciphersuite"
	"golang.org/x/xerrors"
)

var bn256Suite = pairing.NewSuiteBn256()

// BlsName is the name of BLS cipher suite.
var BlsName = "CIPHER_SUITE_BLS_BN256"

// Bn256PublicKey is the public key implementation for ciphers using the
// BN256 elliptic curve.
type Bn256PublicKey struct {
	point  kyber.Point
	cipher ciphersuite.Name
}

// Pack returns the cipher data container of the public key.
func (pk *Bn256PublicKey) Pack() *ciphersuite.CipherData {
	buf, _ := pk.point.MarshalBinary()
	return &ciphersuite.CipherData{
		Name: pk.cipher,
		Data: buf,
	}
}

// Unpack converts the cipher data into a public key.
func (pk *Bn256PublicKey) Unpack(data *ciphersuite.CipherData) error {
	pk.cipher = data.Name
	return pk.point.UnmarshalBinary(data.Data)
}

// Name returns the name of the cipher for this public key.
func (pk *Bn256PublicKey) Name() ciphersuite.Name {
	return pk.cipher
}

func (pk *Bn256PublicKey) String() string {
	return pk.point.String()
}

// Bn256SecretKey is the secret key implementation for ciphers using the
// BN256 elliptic curve.
type Bn256SecretKey struct {
	scalar kyber.Scalar
	cipher ciphersuite.Name
}

// Pack returns the cipher data container of the secret key.
func (sk *Bn256SecretKey) Pack() *ciphersuite.CipherData {
	buf, _ := sk.scalar.MarshalBinary()
	return &ciphersuite.CipherData{
		Name: sk.cipher,
		Data: buf,
	}
}

// Unpack converts a secret key back from cipher data.
func (sk *Bn256SecretKey) Unpack(data *ciphersuite.CipherData) error {
	sk.cipher = data.Name
	return sk.scalar.UnmarshalBinary(data.Data)
}

// Name returns the name of the cipher for this secret key.
func (sk *Bn256SecretKey) Name() ciphersuite.Name {
	return sk.cipher
}

func (sk *Bn256SecretKey) String() string {
	return sk.scalar.String()
}

// Bn256Signature is the signature implementation for ciphers using the
// BN256 elliptic curve.
type Bn256Signature struct {
	point  kyber.Point
	mask   []byte
	cipher ciphersuite.Name
}

// Pack returns the cipher data container for this signature.
func (s *Bn256Signature) Pack() *ciphersuite.CipherData {
	buf, _ := s.point.MarshalBinary()
	return &ciphersuite.CipherData{
		Name: s.cipher,
		Data: append(buf, s.mask...),
	}
}

// Unpack converts a signature back from cipher data.
func (s *Bn256Signature) Unpack(data *ciphersuite.CipherData) error {
	buf := data.Data[:s.point.MarshalSize()]
	s.mask = data.Data[s.point.MarshalSize():]
	s.cipher = data.Name
	return s.point.UnmarshalBinary(buf)
}

// Name returns the name of the cipher for this signature.
func (s *Bn256Signature) Name() ciphersuite.Name {
	return s.cipher
}

// Count returns the number of signature contained in the aggregation.
func (s *Bn256Signature) Count() int {
	sum := 0
	for _, b := range s.mask {
		for i := 0; i < 8; i++ {
			sum += (int(b) >> uint(i)) & 1
		}
	}

	return sum
}

func (s *Bn256Signature) String() string {
	return s.point.String()
}

// BlsCipherSuite is an implementation of the collective signature cipher suite
// that using the BLS signature algorithm.
type BlsCipherSuite struct{}

// NewBlsSuite makes a BLS cipher suite and returns it.
func NewBlsSuite() *BlsCipherSuite {
	return new(BlsCipherSuite)
}

// Name returns the name of the cipher suite.
func (s *BlsCipherSuite) Name() ciphersuite.Name {
	return BlsName
}

// PublicKey returns an empty implementation of the public key.
func (s *BlsCipherSuite) PublicKey() ciphersuite.PublicKey {
	return &Bn256PublicKey{
		cipher: BlsName,
		point:  bn256Suite.G2().Point(),
	}
}

// SecretKey returns an empty implementation of the secret key.
func (s *BlsCipherSuite) SecretKey() ciphersuite.SecretKey {
	return &Bn256SecretKey{
		cipher: BlsName,
		scalar: bn256Suite.Scalar(),
	}
}

// Signature returns an empty implementation of the signature.
func (s *BlsCipherSuite) Signature() ciphersuite.Signature {
	return &Bn256Signature{
		cipher: BlsName,
		point:  bn256Suite.G1().Point(),
		mask:   []byte{},
	}
}

// KeyPair makes an random key pair and returns the secret abd public key.
func (s *BlsCipherSuite) KeyPair() (ciphersuite.PublicKey, ciphersuite.SecretKey) {
	kp := key.NewKeyPair(bn256Suite)
	return &Bn256PublicKey{point: kp.Public, cipher: BlsName}, &Bn256SecretKey{scalar: kp.Private, cipher: BlsName}
}

// Sign signs the message using the secret key and returns the signature that can be
// verified with the public key associated. The signature is not initialized with a mask.
func (s *BlsCipherSuite) Sign(sk ciphersuite.SecretKey, msg []byte) (ciphersuite.Signature, error) {
	secretKey, ok := sk.(*Bn256SecretKey)
	if !ok {
		return nil, xerrors.New("mismatching secret key type")
	}

	buf, err := bls.Sign(bn256Suite, secretKey.scalar, msg)
	if err != nil {
		return nil, err
	}

	sig := s.Signature().(*Bn256Signature)
	err = sig.point.UnmarshalBinary(buf)

	return sig, err
}

// SignWithMask signs the message using the secret key and returns the signature
// that can be verified with the public key associated. The signature will contain
// the corresponding mask and it will be used for aggregating with other ones.
func (s *BlsCipherSuite) SignWithMask(sk ciphersuite.SecretKey, msg []byte, mask *sign.Mask) (ciphersuite.Signature, error) {
	if mask.CountEnabled() != 1 {
		return nil, xerrors.New("mask must have only one bit enabled")
	}

	sig, err := s.Sign(sk, msg)
	if err != nil {
		return nil, err
	}

	sig.(*Bn256Signature).mask = mask.Mask()
	return sig, nil
}

// Verify returns nil if the signature verifies the message. An error is returned
// otherwise with details about the reason.
func (s *BlsCipherSuite) Verify(pk ciphersuite.PublicKey, sig ciphersuite.Signature, msg []byte) error {
	publicKey, ok := pk.(*Bn256PublicKey)
	if !ok {
		return xerrors.New("mismatching public key type")
	}

	sigdata := sig.Pack().Data

	// TODO: threshold
	return bls.Verify(bn256Suite, publicKey.point, msg, sigdata)
}

// VerifyThreshold returns true if the aggregation has enough signatures.
func (s *BlsCipherSuite) VerifyThreshold(sig ciphersuite.Signature, threshold int) bool {
	return s.Count(sig) >= threshold
}

// AggregatePublicKeys produces a single public key that can verify the signature
// passed in parameter.
func (s *BlsCipherSuite) AggregatePublicKeys(publicKeys []ciphersuite.PublicKey, sig ciphersuite.Signature) (ciphersuite.PublicKey, error) {
	mask, err := s.Mask(publicKeys)
	if err != nil {
		return nil, err
	}

	err = mask.SetMask(sig.(*Bn256Signature).mask)
	if err != nil {
		return nil, err
	}

	agg := bls.AggregatePublicKeys(bn256Suite, mask.Participants()...)

	pk := s.PublicKey()
	pk.(*Bn256PublicKey).point = agg

	return pk, nil
}

// AggregateSignatures aggregates the signature so that the result will be
// verifiable by the corresponding public key.
func (s *BlsCipherSuite) AggregateSignatures(sigs []ciphersuite.Signature, pubs []ciphersuite.PublicKey) (ciphersuite.Signature, error) {
	if len(sigs) == 0 {
		return nil, xerrors.New("expect at least one signature")
	}

	sigbuf := make([][]byte, len(sigs))
	mask, err := s.Mask(pubs)
	if err != nil {
		return nil, err
	}
	for i, sig := range sigs {
		buf, err := sig.(*Bn256Signature).point.MarshalBinary()
		if err != nil {
			return nil, err
		}

		sigbuf[i] = buf

		err = mask.Merge(sig.(*Bn256Signature).mask)
		if err != nil {
			return nil, err
		}
	}

	buf, err := bls.AggregateSignatures(bn256Suite, sigbuf...)
	if err != nil {
		return nil, err
	}

	sig := s.Signature().(*Bn256Signature)
	sig.mask = mask.Mask()
	err = sig.point.UnmarshalBinary(buf)

	return sig, err
}

// Hash returns a context for the cipher suite.
func (s *BlsCipherSuite) Hash() hash.Hash {
	return bn256Suite.Hash()
}

// Mask returns a mask compatible with the list of public keys.
func (s *BlsCipherSuite) Mask(publicKeys []ciphersuite.PublicKey) (*sign.Mask, error) {
	publics := make([]kyber.Point, len(publicKeys))
	for i, pk := range publicKeys {
		pubkey, ok := pk.(*Bn256PublicKey)
		if !ok {
			return nil, xerrors.New("wrong type of public key for this suite")
		}

		publics[i] = pubkey.point
	}

	m, err := sign.NewMask(bn256Suite, publics, nil)
	if err != nil {
		return nil, err
	}

	return m, nil
}

// Count returns the number of signatures contained in the aggregation.
func (s *BlsCipherSuite) Count(sig ciphersuite.Signature) int {
	return sig.(*Bn256Signature).Count()
}
