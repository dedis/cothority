package cosuite

import (
	"go.dedis.ch/kyber/v4/sign"
	"go.dedis.ch/kyber/v4/sign/bdn"
	"go.dedis.ch/onet/v4/ciphersuite"
	"golang.org/x/xerrors"
)

// BdnName is the name of the cipher suite.
var BdnName = "CIPHER_SUITE_BDN_BN256"

// BdnCipherSuite is a cipher suite that is using the BN256 elliptic curve and the BLS
// signature algorithm upgraded with coefficients to protect against rogue public key
// attacks.
type BdnCipherSuite struct {
	*BlsCipherSuite
}

// NewBdnSuite returns an instance of the cipher suite.
func NewBdnSuite() *BdnCipherSuite {
	return &BdnCipherSuite{
		BlsCipherSuite: NewBlsSuite(),
	}
}

// Name returns the name of the cipher suite.
func (s *BdnCipherSuite) Name() ciphersuite.Name {
	return BdnName
}

// PublicKey returns an empty implementation of the public key.
func (s *BdnCipherSuite) PublicKey() ciphersuite.PublicKey {
	pk := s.BlsCipherSuite.PublicKey().(*Bn256PublicKey)
	pk.cipher = BdnName
	return pk
}

// SecretKey returns an empty implementation of the secret key.
func (s *BdnCipherSuite) SecretKey() ciphersuite.SecretKey {
	sk := s.BlsCipherSuite.SecretKey().(*Bn256SecretKey)
	sk.cipher = BdnName
	return sk
}

// Signature returns an empty implementation of the signature.
func (s *BdnCipherSuite) Signature() ciphersuite.Signature {
	sig := s.BlsCipherSuite.Signature().(*Bn256Signature)
	sig.cipher = BdnName
	return sig
}

// KeyPair makes an random key pair and returns the secret abd public key.
func (s *BdnCipherSuite) KeyPair() (ciphersuite.PublicKey, ciphersuite.SecretKey) {
	pk, sk := s.BlsCipherSuite.KeyPair()
	pk.(*Bn256PublicKey).cipher = BdnName
	sk.(*Bn256SecretKey).cipher = BdnName
	return pk, sk
}

// Sign signs the message using the secret key and returns the signature that can be
// verified with the public key associated. The signature is not initialized with a mask.
func (s *BdnCipherSuite) Sign(sk ciphersuite.SecretKey, msg []byte) (ciphersuite.Signature, error) {
	sig, err := s.BlsCipherSuite.Sign(sk, msg)
	if err != nil {
		return nil, err
	}

	sig.(*Bn256Signature).cipher = BdnName
	return sig, nil
}

// SignWithMask signs the message using the secret key and returns the signature
// that can be verified with the public key associated. The signature will contain
// the corresponding mask and it will be used for aggregating with other ones.
func (s *BdnCipherSuite) SignWithMask(sk ciphersuite.SecretKey, msg []byte, mask *sign.Mask) (ciphersuite.Signature, error) {
	if mask.CountEnabled() != 1 {
		return nil, xerrors.New("expect only one bit enabled")
	}

	secretKey, ok := sk.(*Bn256SecretKey)
	if !ok {
		return nil, xerrors.New("mismatching secret key type")
	}

	sig, err := bdn.Sign(bn256Suite, secretKey.scalar, msg)
	if err != nil {
		return nil, err
	}

	point, err := bdn.AggregateSignatures(bn256Suite, [][]byte{sig}, mask)
	if err != nil {
		return nil, err
	}

	signature := s.Signature().(*Bn256Signature)
	signature.point = point
	signature.mask = mask.Mask()
	return signature, nil
}

// AggregatePublicKeys produces a single public key that can verify the signature
// passed in parameter.
func (s *BdnCipherSuite) AggregatePublicKeys(publicKeys []ciphersuite.PublicKey, signature ciphersuite.Signature) (ciphersuite.PublicKey, error) {
	mask, err := s.Mask(publicKeys)
	if err != nil {
		return nil, err
	}

	sig, ok := signature.(*Bn256Signature)
	if !ok || sig.cipher != BdnName {
		return nil, xerrors.New("wrong type of signature")
	}

	err = mask.SetMask(sig.mask)
	if err != nil {
		return nil, err
	}

	point, err := bdn.AggregatePublicKeys(bn256Suite, mask)
	if err != nil {
		return nil, err
	}

	pk := s.PublicKey().(*Bn256PublicKey)
	pk.point = point

	return pk, nil
}

// AggregateSignatures aggregates the signature so that the result will be
// verifiable by the corresponding public key.
func (s *BdnCipherSuite) AggregateSignatures(sigs []ciphersuite.Signature, pubs []ciphersuite.PublicKey) (ciphersuite.Signature, error) {
	sig, err := s.BlsCipherSuite.AggregateSignatures(sigs, pubs)
	if err != nil {
		return nil, err
	}

	sig.(*Bn256Signature).cipher = BdnName

	return sig, nil
}
