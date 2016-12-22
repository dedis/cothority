package main

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"

	"github.com/BurntSushi/toml"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/anon"
	"github.com/dedis/crypto/ed25519"
	"github.com/dedis/crypto/random"
	"github.com/gopherjs/gopherjs/js"
)

func main() {
	js.Global.Set("sig", map[string]interface{}{
		"Sign": Sign,
		"Toy":  Toy,
	})
}

type container struct {
	Attendees []abstract.Point
	Nonce     string
	Context   string
}

func readContainer(suite abstract.Suite, containerB string) *container {
	var cont struct {
		Attendees [][]byte
		Nonce     string
		Context   string
	}
	if err := toml.Unmarshal([]byte(containerB), &cont); err != nil {
		panic(err)
	}
	var pubs = make([]abstract.Point, len(cont.Attendees))
	for i, buff := range cont.Attendees {
		point := suite.Point()
		if err := point.UnmarshalBinary(buff); err != nil {
			panic(err)
		}
		pubs[i] = point
	}
	return &container{
		Attendees: pubs,
		Nonce:     cont.Nonce,
		Context:   cont.Context,
	}
}

func Toy() string {
	return "hello world"
}

// Sign computes the ring signature over the nonce contained in the container
// and returns the slice of bytes that must be ported back to the server.
// XXX TODO later add hash verification
func Sign(private64, container string) (string, string) {
	suite := ed25519.NewAES128SHA256Ed25519(false)
	// XXX Signature verification still not impl.
	cont := readContainer(suite, container)
	set := anon.Set(cont.Attendees)
	// find our index in the set
	private, err := base64.StdEncoding.DecodeString(private64)
	if err != nil {
		return "", err.Error()
	}
	myPriv := suite.Scalar()
	myPriv.UnmarshalBinary(private)
	myPub := suite.Point().Mul(nil, myPriv)
	myIdx := -1
	println("sig.go: ranging over attendees ", len(cont.Attendees))
	for i, other := range cont.Attendees {
		if myPub.Equal(other) {
			myIdx = i
		}
	}

	if myIdx == -1 {
		return "", "Could not find our public key in the list"
	}

	println("sig.go: Nonce :", hex.EncodeToString([]byte(cont.Nonce)))
	println("sig.go: Context:", cont.Context)
	sig := anon.Sign(suite, random.Stream, []byte(cont.Nonce), set, []byte(cont.Context), myIdx, myPriv)
	var loginInfo struct {
		Nonce     []byte
		Signature []byte
	}
	println("sig.go: Signature (", len(sig), ") :", hex.EncodeToString(sig))
	loginInfo.Nonce = []byte(cont.Nonce)
	loginInfo.Signature = sig
	var b bytes.Buffer
	if err := toml.NewEncoder(&b).Encode(loginInfo); err != nil {
		return "", err.Error()
	}
	return b.String(), ""
}
