package main

import (
	"bytes"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"github.com/dedis/cothority/log"
	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/armor"
)

func main() {
	const text = `975ae735dc4617410681a02e4fe9a9a492417ef8`
	const amount = 5
	const poname = "policy.toml"
	const privfile = "privatering.txt"

	var developers openpgp.EntityList

	//cfg := &packet.Config{
	//	DefaultHash:   crypto.SHA256,
	//	DefaultCipher: packet.CipherAES256,
	//	RSABits:       2048,
	//}

	for i := 0; i < amount; i++ {
		entity, err := openpgp.NewEntity(strconv.Itoa(i), "", "", nil)
		developers = append(developers, entity)
		if err != nil {
			log.Errorf("PGP entity %+v has not been created %+v", i, err)
		}
	}

	// Writing threshold to a policy file
	pubwr := new(bytes.Buffer)
	_, err := pubwr.WriteString("threshold = ")
	_, err = pubwr.WriteString(strconv.Itoa(amount))
	_, err = pubwr.WriteString("\n\npublicKeys = [\n")
	if err := ioutil.WriteFile(poname, pubwr.Bytes(), 0660); err != nil {
		log.Errorf("Could not write thresold value to the file:", err)
	}
	pubwr.Reset()

	fpub, _ := os.OpenFile(poname, os.O_APPEND|os.O_WRONLY, 0660)
	defer fpub.Close()
	fpriv, _ := os.OpenFile(privfile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0660)
	defer fpriv.Close()
	fpriv.WriteString("Entities = [\n")

	for i, entity := range developers {
		// Saving private and public keys
		privwr := new(bytes.Buffer)
		privarmor, _ := armor.Encode(privwr, openpgp.PrivateKeyType, nil)
		pubarmor, _ := armor.Encode(pubwr, openpgp.PublicKeyType, nil)
		if err := entity.SerializePrivate(privarmor, nil); err != nil {
			log.Errorf("Problem with serializing private key:", err)
		}
		if err := entity.Serialize(pubarmor); err != nil {
			log.Error("Problem with serializing public key:", err)
		}
		privarmor.Close()
		pubarmor.Close()

		fpriv.WriteString("\"\"\"")
		fpub.WriteString("\"\"\"")
		if i != len(developers)-1 {
			pubwr.Write([]byte("\n\"\"\",\n"))
			privwr.Write([]byte("\n\"\"\",\n"))
		} else {
			pubwr.Write([]byte("\n\"\"\"]"))
			privwr.Write([]byte("\n\"\"\"]"))
		}

		if _, err := fpriv.Write(privwr.Bytes()); err != nil {
			log.Error("Could not write privates key to a file:", err)
		}
		if _, err := fpub.Write(pubwr.Bytes()); err != nil {
			log.Error("Could not write a public key to policy file:", err)
		}
		privwr.Reset()
		pubwr.Reset()
	}

	for _, entity := range developers {
		openpgp.ArmoredDetachSign(pubwr, entity, strings.NewReader(text), nil)
		pubwr.WriteByte(byte('\n'))
	}

	err = ioutil.WriteFile("signatures.txt", pubwr.Bytes(), 0660)
	if err != nil {
		log.Error("Could not write to a signatures file", err)
	}

	//fmt.Println(w.String())
}
