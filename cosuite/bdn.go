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
	bls := NewBlsSuite()
	bls.cipher = BdnName
	return &BdnCipherSuite{
		BlsCipherSuite: bls,
	}
}

// SignWithMask signs the message using the secret key and returns the signature
// that can be verified with the public key associated. The signature will contain
// the corresponding mask and it will be used for aggregating with other ones.
func (s *BdnCipherSuite) SignWithMask(sk ciphersuite.SecretKey, msg []byte, mask *sign.Mask) (ciphersuite.Signature, error) {
	if mask.CountEnabled() != 1 {
		return nil, xerrors.New("expect only one bit enabled")
	}

	secretKey, err := s.unpackSecretKey(sk)
	if err != nil {
		return nil, xerrors.Errorf("unpacking secret key: %v", err)
	}

	sig, err := bdn.Sign(bn256Suite, secretKey.scalar, msg)
	if err != nil {
		return nil, err
	}

	point, err := bdn.AggregateSignatures(bn256Suite, [][]byte{sig}, mask)
	if err != nil {
		return nil, err
	}

	signature := newBn256Signature()
	signature.cipher = s.Name()
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

	sig, err := s.unpackSignature(signature)
	if err != nil {
		return nil, xerrors.Errorf("unpacking signature: %v", err)
	}

	err = mask.SetMask(sig.mask)
	if err != nil {
		return nil, err
	}

	point, err := bdn.AggregatePublicKeys(bn256Suite, mask)
	if err != nil {
		return nil, err
	}

	pk := &Bn256PublicKey{
		cipher: s.Name(),
		point:  point,
	}

	return pk, nil
}
