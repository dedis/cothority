package crypto

import (
	"crypto/sha256"
	"encoding/binary"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/cosi"
	"github.com/dedis/crypto/ed25519"
	"github.com/gopherjs/gopherjs/js"
)

// Given a SkipBlock object, return the hash
func HashSkipBlock(block *js.Object) []byte {

	hash := sha256.New()

	for _, i := range []*js.Object{block.Get("Index"), block.Get("Height"), block.Get("MaximumHeight"),
		block.Get("BaseHeight")} {

		binary.Write(hash, binary.LittleEndian, i.Int())
	}

	if block.Get("BackLinkIDs") != js.Undefined {
		for _, i := range block.Get("BackLinkIDs").Interface().([]interface{}) {
			hash.Write(i.([]byte))
		}
	}

	if block.Get("VerifierIDs") != js.Undefined {
		for _, i := range block.Get("VerifierIDs").Interface().([]interface{}) {
			hash.Write(i.([]byte))
		}
	}

	if block.Get("GenesisID") != js.Undefined {
		hash.Write(block.Get("GenesisID").Interface().([]byte))
	}

	if block.Get("Data") != js.Undefined {
		hash.Write(block.Get("Data").Interface().([]byte))
	}

	// Add the public key of the roster in the hash
	if block.Get("Roster") != js.Undefined && block.Get("Roster").Get("list") != js.Undefined {

		for _, server := range block.Get("Roster").Get("list").Interface().([]interface{}) {
			hash.Write(server.(map[string]interface{})["public"].([]byte))
		}
	}

	return hash.Sum(nil)
}

// Given an object with the proper fields, it will check the signature and return true if everything's fine
func VerifyForwardLink(obj *js.Object) bool {
	suite := ed25519.NewAES128SHA256Ed25519(false)

	var keys []abstract.Point
	if obj.Get("publicKeys") != js.Undefined {

		keys = make([]abstract.Point, obj.Get("publicKeys").Length())
		for i, k := range obj.Get("publicKeys").Interface().([]interface{}) {
			p := suite.Point()
			p.UnmarshalBinary(k.([]byte))
			keys[i] = p
		}

	}

	var hash []byte
	if obj.Get("hash") != js.Undefined {
		hash = obj.Get("hash").Interface().([]byte)
	}

	var signature []byte
	if obj.Get("signature") != js.Undefined {
		signature = obj.Get("signature").Interface().([]byte)
	}

	// Every field must be provided
	if keys == nil || hash == nil || signature == nil {
		return false
	}

	return cosi.VerifySignature(suite, keys, hash, signature) == nil
}
