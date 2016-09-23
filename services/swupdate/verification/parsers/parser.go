package parsers

import (
	"bufio"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/log"
)

// Scanner for a file containing signatures
func SigScanner(filename string) ([]string, error) {
	var blocks []string
	head := "-----BEGIN PGP SIGNATURE-----"
	log.Lvl2("Reading file", filename)

	file, err := os.Open(filename)
	defer file.Close()
	if err != nil {
		log.Errorf("Couldn't open file", file, err)
		return nil, err
	}

	scanner := bufio.NewScanner(file)
	var block []string
	for scanner.Scan() {
		text := scanner.Text()
		log.Lvl4("Decoding", text)
		// end of the first part
		if text == head {
			log.Lvl4("Found header")
			if len(block) > 0 {
				blocks = append(blocks, strings.Join(block, "\n"))
				block = make([]string, 0)
			}
		}
		block = append(block, text)
	}
	blocks = append(blocks, strings.Join(block, "\n"))
	return blocks, nil
}

// Scanner for a file containing policy
func PolicyScanner(filename string) (int, []string, error) {
	type policyToml struct {
		Threshold  int
		PublicKeys []string
	}
	var p policyToml

	log.Lvl3("Reading file", filename)
	meta, err := toml.DecodeFile(filename, &p)
	if err != nil {
		log.Fatal(err)
	}

	log.Lvlf4("Fields of the policy are %+v", meta.Keys())
	return p.Threshold, p.PublicKeys, err
}

// Scanner for a file containing commit id
func ReleaseScanner(filename string) (string, string, string, error) {
	type releaseToml struct {
		Hashr    string
		GitPath  string
		Commitid string
	}
	var release releaseToml

	log.Lvl3("Reading file", filename)
	_, err := toml.DecodeFile(filename, &release)
	if err != nil {
		log.Lvlf1("Could not decode a toml release file", err)
	}

	return release.Hashr, release.GitPath, release.Commitid, err
}
