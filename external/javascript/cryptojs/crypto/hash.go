package crypto

import (
	"crypto/sha256"
	"crypto/sha512"
)

// Return the hex encoded sha256 hash of the provided string
func Sha256(bytes []byte) []byte {
	hash := sha256.New()
	hash.Write(bytes)

	return hash.Sum(nil)
}

// Return the hex encoded sha512 hash of the provided string
func Sha512(bytes []byte) []byte {
	hash := sha512.New()
	hash.Write(bytes)

	return hash.Sum(nil)
}
