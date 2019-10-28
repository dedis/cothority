package cosuite

import (
	"hash"
	"io"

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

// Raw returns the cipher data container of the public key.
func (pk *Bn256PublicKey) Raw() *ciphersuite.RawPublicKey {
	buf, _ := pk.point.MarshalBinary()
	return &ciphersuite.RawPublicKey{
		CipherData: &ciphersuite.CipherData{
			CipherName: pk.cipher,
			Data:       buf,
		},
	}
}

// Name returns the name of the cipher for this public key.
func (pk *Bn256PublicKey) Name() ciphersuite.Name {
	return pk.cipher
}

func (pk *Bn256PublicKey) Equal(other ciphersuite.PublicKey) bool {
	if iother, ok := other.(*Bn256PublicKey); ok {
		return iother.point.Equal(pk.point)
	}

	return false
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

// Raw returns the cipher data container of the secret key.
func (sk *Bn256SecretKey) Raw() *ciphersuite.RawSecretKey {
	buf, _ := sk.scalar.MarshalBinary()
	return &ciphersuite.RawSecretKey{
		CipherData: &ciphersuite.CipherData{
			CipherName: sk.cipher,
			Data:       buf,
		},
	}
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

func newBn256Signature() *Bn256Signature {
	return &Bn256Signature{
		point: bn256Suite.G1().Point(),
		mask:  []byte{},
	}
}

// Raw returns the cipher data container for this signature.
func (s *Bn256Signature) Raw() *ciphersuite.RawSignature {
	buf, _ := s.point.MarshalBinary()
	return &ciphersuite.RawSignature{
		CipherData: &ciphersuite.CipherData{
			CipherName: s.cipher,
			Data:       append(buf, s.mask...),
		},
	}
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
type BlsCipherSuite struct {
	cipher ciphersuite.Name
}

// NewBlsSuite makes a BLS cipher suite and returns it.
func NewBlsSuite() *BlsCipherSuite {
	return &BlsCipherSuite{
		cipher: BlsName,
	}
}

// Name returns the name of the cipher suite.
func (s *BlsCipherSuite) Name() ciphersuite.Name {
	return s.cipher
}

// PublicKey returns an empty implementation of the public key.
func (s *BlsCipherSuite) PublicKey(raw *ciphersuite.RawPublicKey) (ciphersuite.PublicKey, error) {
	pk := &Bn256PublicKey{
		cipher: s.Name(),
		point:  bn256Suite.G2().Point(),
	}

	return pk, pk.point.UnmarshalBinary(raw.Data)
}

// SecretKey returns an empty implementation of the secret key.
func (s *BlsCipherSuite) SecretKey(raw *ciphersuite.RawSecretKey) (ciphersuite.SecretKey, error) {
	sk := &Bn256SecretKey{
		cipher: s.Name(),
		scalar: bn256Suite.Scalar(),
	}

	return sk, sk.scalar.UnmarshalBinary(raw.Data)
}

// Signature returns an empty implementation of the signature.
func (s *BlsCipherSuite) Signature(raw *ciphersuite.RawSignature) (ciphersuite.Signature, error) {
	sig := &Bn256Signature{
		cipher: s.Name(),
		point:  bn256Suite.G1().Point(),
	}
	sig.mask = raw.Data[sig.point.MarshalSize():]

	return sig, sig.point.UnmarshalBinary(raw.Data)
}

// GenerateKeyPair makes an random key pair and returns the secret abd public key.
func (s *BlsCipherSuite) GenerateKeyPair(reader io.Reader) (ciphersuite.PublicKey, ciphersuite.SecretKey, error) {
	kp := key.NewKeyPair(bn256Suite)
	return &Bn256PublicKey{point: kp.Public, cipher: s.Name()}, &Bn256SecretKey{scalar: kp.Private, cipher: s.Name()}, nil
}

// Sign signs the message using the secret key and returns the signature that can be
// verified with the public key associated. The signature is not initialized with a mask.
func (s *BlsCipherSuite) Sign(sk ciphersuite.SecretKey, msg []byte) (ciphersuite.Signature, error) {
	secretKey, err := s.unpackSecretKey(sk)
	if err != nil {
		return nil, xerrors.Errorf("unpacking secret key: %v", err)
	}

	buf, err := bls.Sign(bn256Suite, secretKey.scalar, msg)
	if err != nil {
		return nil, err
	}

	sig := newBn256Signature()
	sig.cipher = s.Name()

	return sig, sig.point.UnmarshalBinary(buf)
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
	publicKey, err := s.unpackPublicKey(pk)
	if err != nil {
		return xerrors.Errorf("unpacking public key: %v", err)
	}

	sigdata := sig.Raw().Data

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

	signature, err := s.unpackSignature(sig)
	if err != nil {
		return nil, xerrors.Errorf("unpacking signature: %v", err)
	}

	err = mask.SetMask(signature.mask)
	if err != nil {
		return nil, err
	}

	agg := bls.AggregatePublicKeys(bn256Suite, mask.Participants()...)

	pk := &Bn256PublicKey{
		point:  agg,
		cipher: s.Name(),
	}

	return pk, nil
}

// AggregateSignatures aggregates the signature so that the result will be
// verifiable by the corresponding public key.
func (s *BlsCipherSuite) AggregateSignatures(sigs []ciphersuite.Signature, pubs []ciphersuite.PublicKey) (ciphersuite.Signature, error) {
	signature := newBn256Signature()
	signature.cipher = s.Name()

	mask, err := s.Mask(pubs)
	if err != nil {
		return nil, err
	}

	if len(sigs) == 0 {
		signature.mask = mask.Mask()
		return signature, nil
	}

	sigbuf := make([][]byte, len(sigs))
	for i, sig := range sigs {
		signature, err := s.unpackSignature(sig)
		if err != nil {
			return nil, xerrors.Errorf("unpacking signature: %v", err)
		}

		buf, err := signature.point.MarshalBinary()
		if err != nil {
			return nil, err
		}

		sigbuf[i] = buf

		err = mask.Merge(signature.mask)
		if err != nil {
			return nil, err
		}
	}

	buf, err := bls.AggregateSignatures(bn256Suite, sigbuf...)
	if err != nil {
		return nil, err
	}

	signature.mask = mask.Mask()
	err = signature.point.UnmarshalBinary(buf)
	if err != nil {
		return nil, xerrors.Errorf("unmarshaling point: %v", err)
	}

	return signature, nil
}

// Hash returns a context for the cipher suite.
func (s *BlsCipherSuite) Hash() hash.Hash {
	return bn256Suite.Hash()
}

// Mask returns a mask compatible with the list of public keys.
func (s *BlsCipherSuite) Mask(publicKeys []ciphersuite.PublicKey) (*sign.Mask, error) {
	publics := make([]kyber.Point, len(publicKeys))
	for i, pk := range publicKeys {
		pubkey, err := s.unpackPublicKey(pk)
		if err != nil {
			return nil, xerrors.Errorf("unpacking public key: %v", err)
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
	signature, err := s.unpackSignature(sig)
	if err != nil {
		return 0
	}

	return signature.Count()
}

func (s *BlsCipherSuite) unpackPublicKey(pubkey ciphersuite.PublicKey) (*Bn256PublicKey, error) {
	if data, ok := pubkey.(*ciphersuite.RawPublicKey); ok {
		var err error
		pubkey, err = s.PublicKey(data)
		if err != nil {
			return nil, xerrors.Errorf("decoding: %v", err)
		}
	}

	if pk, ok := pubkey.(*Bn256PublicKey); ok {
		return pk, nil
	}

	return nil, xerrors.New("invalid public key type")
}

func (s *BlsCipherSuite) unpackSecretKey(secret ciphersuite.SecretKey) (*Bn256SecretKey, error) {
	if data, ok := secret.(*ciphersuite.RawSecretKey); ok {
		var err error
		secret, err = s.SecretKey(data)
		if err != nil {
			return nil, xerrors.Errorf("decoding: %v", err)
		}
	}

	if sk, ok := secret.(*Bn256SecretKey); ok {
		return sk, nil
	}

	return nil, xerrors.New("invalid secret key type")
}

func (s *BlsCipherSuite) unpackSignature(signature ciphersuite.Signature) (*Bn256Signature, error) {
	if data, ok := signature.(*ciphersuite.RawSignature); ok {
		var err error
		signature, err = s.Signature(data)
		if err != nil {
			return nil, xerrors.Errorf("decoding: %v", err)
		}
	}

	if sig, ok := signature.(*Bn256Signature); ok {
		return sig, nil
	}

	return nil, xerrors.New("invalid signature type")
}
