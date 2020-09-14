package darc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/mr-tron/base58"
	"go.dedis.ch/onet/v3/log"
	"golang.org/x/xerrors"
)

// DIDResolver resolves a given DID to a DID Document
type DIDResolver interface {
	Resolve(string) (*DIDDoc, error)
}

// IndyCLIDIDResolver resolves a DID using `indy-cli`
// https://github.com/hyperledger/indy-sdk/tree/master/cli
type IndyCLIDIDResolver struct {
	Path            string
	GenesisFilePath string
}

// DIDVerificationKeyTypes represents the valid key types
// that may be stored in a DID document
// Refer to https://w3c-ccg.github.io/ld-cryptosuite-registry/
type DIDVerificationKeyTypes int

const (
	// Ed25519VerificationKey2018 represents key type for Ed25519 keys
	Ed25519VerificationKey2018 DIDVerificationKeyTypes = iota
	// RsaVerificationKey2018 represents key type for RSA Keys
	RsaVerificationKey2018
	// EcdsaSecp256k1VerificationKey2019 represents key type for Secp256k1 keys
	EcdsaSecp256k1VerificationKey2019
)

func (t DIDVerificationKeyTypes) String() string {
	return [...]string{
		"Ed25519VerificationKey2018",
		"RsaVerificationKey2018",
		"EcdsaSecp256k1VerificationKey2019",
	}[t]
}

func (r *IndyCLIDIDResolver) generatePoolName(n int) string {
	rand.Seed(time.Now().UnixNano())
	alphabet := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, n)
	for i := range b {
		b[i] = alphabet[rand.Intn(len(alphabet))]
	}
	return string(b)
}

// This would initiate a connection to the pool for every request we receive.
// An optimal way would be to reuse existing connections but that can be
// done by another resolver
func (r *IndyCLIDIDResolver) executeCli(id string) (string, error) {
	log.Infof("executing cli")
	poolName := r.generatePoolName(6)
	commands := fmt.Sprintf(`
pool create %s gen_txn_file="%s"
pool connect %s
ledger get-nym did=%s
ledger get-attrib did=%s raw=endpoint
pool disconnect
pool delete %s`, poolName, r.GenesisFilePath, poolName, id, id, poolName)

	cmd := exec.Command(r.Path)
	cmd.Stdin = strings.NewReader(commands)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("error executing indy-cli: %s", err)
	}
	return out.String(), nil
}

// parseOutput parses the output from indy-cli. It assumes only one verkey and service
// in the output for now
func (r *IndyCLIDIDResolver) parseOutput(id, output string) (time.Time, []PublicKey, []DIDService, error) {
	verkey := ""
	endpoint := ""
	createdAt := ""
	lines := strings.Split(output, "\n")

	for i, line := range lines {
		if createdAt == "" && strings.Contains(line, "| Transaction time") && len(lines) > i+2 {
			components := strings.Split(lines[i+2], "|")
			createdAt = strings.TrimSpace(components[len(components)-2])
		}
		if strings.Contains(line, "| Verkey") && len(lines) > i+2 {
			components := strings.Split(lines[i+2], "|")
			verkey = strings.TrimSpace(components[len(components)-3])
		} else if strings.Contains(line, "| Data") && len(lines) > i+2 {
			components := strings.Split(lines[i+2], "|")
			endpoint = strings.TrimSpace(components[1])
			break
		}
	}

	/*
		_createdAt, err := time.Parse("2006-01-02 22:04:05", createdAt)
		_createdAt, err := time.Parse("2006-01-02 15:04:05", createdAt)
		if err != nil {
		/return time.Time{}, nil, nil, fmt.Errorf("error parsing time: %s", err)
		}
	*/

	idBuf, err := base58.Decode(id)
	if err != nil {
		return time.Time{}, nil, nil, fmt.Errorf("error base58 decoding did: %s", err)
	}

	// Some txns have verykey beginning with a ~ sign
	var verkeyBuf []byte
	if len(verkey) > 0 && verkey[0] == '~' {
		verkeyBuf, err = base58.Decode(verkey[1:])
	} else {
		verkeyBuf, err = base58.Decode(verkey)
	}

	var pkBuf []byte
	if len(verkey) > 0 && verkey[0] == '~' {
		pkBuf = append(idBuf, verkeyBuf...)
	} else {
		pkBuf = verkeyBuf
	}

	pk := PublicKey{
		ID:         fmt.Sprintf("%s-keys#1", id),
		Type:       Ed25519VerificationKey2018.String(),
		Controller: id,
		Value:      pkBuf,
	}

	var svcs []DIDService
	if endpoint != "" {
		// Adopted from ACA-Py
		svcs = append(svcs, DIDService{
			ID:              "indy",
			Type:            "IndyAgent",
			Priority:        0,
			RecipientKeys:   []string{base58.Encode(pkBuf)},
			ServiceEndpoint: endpoint,
		})
	}
	return time.Time{}, []PublicKey{pk}, svcs, nil
}

// Resolve resolves a did to a DID document using indy-cli.
func (r *IndyCLIDIDResolver) Resolve(id string) (*DIDDoc, error) {
	output, err := r.executeCli(id)
	if strings.Contains(output, "NYM not found") {
		return nil, errors.New("DID not found in the ledger")
	}
	if err != nil {
		return nil, err
	}
	_, pks, svcs, err := r.parseOutput(id, output)
	if err != nil {
		return nil, err
	}

	var auths []VerificationMethod
	for _, pk := range pks {
		auths = append(auths, VerificationMethod{PublicKey: pk})
	}

	return &DIDDoc{
		Context:        []string{""},
		ID:             id,
		PublicKey:      pks,
		Service:        svcs,
		Authentication: auths,
	}, nil
}

type IndyVDRResolver struct {
	URL string
}

type nymResponse struct {
	Data string `json:"data"`
}

type nymResponseData struct {
	Dest   string `json:"dest"`
	Verkey string `json:"verkey"`
}

func (r *IndyVDRResolver) Resolve(id string) (*DIDDoc, error) {
	response, err := http.Get(fmt.Sprintf("%s/nym/%s", r.URL, id))
	if err != nil {
		return nil, xerrors.Errorf("error getting nym txn: %s", err)
	}

	var nr nymResponse
	decoder := json.NewDecoder(response.Body)
	err = decoder.Decode(&nr)
	if err != nil {
		return nil, xerrors.Errorf("error unmarshaling nymResponse: %s", err)
	}

	if nr.Data == "" {
		return nil, xerrors.Errorf("verkey not found")
	}

	var nrdata nymResponseData
	err = json.Unmarshal([]byte(nr.Data), &nrdata)
	if err != nil {
		return nil, xerrors.Errorf("error unmarshaling nym transaction data ket: %s", err)
	}

	if len(nrdata.Verkey) == 0 {
		return nil, xerrors.Errorf("error resolving verkey")
	}

	pkBuf := []byte{}
	if nrdata.Verkey[0] == '~' {
		idBuf, err := base58.Decode(id)
		if err != nil {
			return nil, xerrors.Errorf("error decoding id: %s", err)
		}
		verkeyBuf, err := base58.Decode(nrdata.Verkey[1:])
		if err != nil {
			return nil, xerrors.Errorf("error decoding verkey: %s", err)
		}
		pkBuf = append(pkBuf, idBuf...)
		pkBuf = append(pkBuf, verkeyBuf...)
	} else {
		verkeyBuf, err := base58.Decode(nrdata.Verkey[1:])
		if err != nil {
			return nil, xerrors.Errorf("error decoding verkey: %s", err)
		}
		pkBuf = verkeyBuf
	}

	pk := PublicKey{
		ID:         fmt.Sprintf("%s-keys#1", id),
		Type:       Ed25519VerificationKey2018.String(),
		Controller: id,
		Value:      pkBuf,
	}
	svc := DIDService{}
	auth := VerificationMethod{PublicKey: pk}

	return &DIDDoc{
		Context:        []string{""},
		ID:             id,
		PublicKey:      []PublicKey{pk},
		Service:        []DIDService{svc},
		Authentication: []VerificationMethod{auth},
	}, nil
}
