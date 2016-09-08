package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"strings"

	"os"

	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/log"
	"golang.org/x/crypto/openpgp"
)

func main() {
	//const text = `ea55486dff1a62088a4be47c92c65fe2d802d8a87493e1c6aaed91c7bc04b025`
	var developers openpgp.EntityList
	var text, pfile string
	type privToml struct {
		Entities []string
	}
	var p privToml

	pfile = os.Args[1]
	text = os.Args[2]

	if _, err := toml.DecodeFile(pfile, &p); err != nil {
		log.Error("Could not read private ring file", err)
	}

	developers = make(openpgp.EntityList, 0)
	for _, pubkey := range p.Entities {
		keybuf, err := openpgp.ReadArmoredKeyRing(strings.NewReader(pubkey))
		if err != nil {
			log.Error("Could not decode armored public key", err)
		}
		for _, entity := range keybuf {
			developers = append(developers, entity)
		}
	}

	w := new(bytes.Buffer)
	for _, entity := range developers {
		openpgp.ArmoredDetachSign(w, entity, strings.NewReader(text), nil)
		w.WriteByte(byte('\n'))
	}

	if err := ioutil.WriteFile("signatures.txt", w.Bytes(), 0660); err != nil {
		log.Error("Could not write to a file", err)
	}

	fmt.Println(w.String())
}
