// This implements the simplest possible example of how to set up the
// onchain-secret service. It shall be used as a source for copy/pasting.
package main

import (
	"bytes"
	"io/ioutil"
	"os"

	"strings"

	"github.com/dedis/onchain-secrets"
	"gopkg.in/dedis/cothority.v1/skipchain"
	"gopkg.in/dedis/crypto.v0/cipher/sha3"
	"gopkg.in/dedis/crypto.v0/random"
	"gopkg.in/dedis/onet.v1/app"
	"gopkg.in/dedis/onet.v1/log"
)

// dummy document that will be stored on the skipchain.
var secretDoc = []byte("Top-secret file about Switzerlands plan to build an A-bomb in the 1940s")

// main creates a new chain, stores the invoiceData on it, and then retrieves
// the data.
func main() {
	// Don't show program-lines - set to 1 or higher for mroe debugging
	// messages
	if len(os.Args) < 2 {
		log.Fatal("Please give public.toml as argument")
	}
	publicToml, err := ioutil.ReadFile(os.Args[1])
	log.ErrFatal(err)
	log.SetDebugVisible(0)
	ocs, err := setupChains(string(publicToml))
	log.ErrFatal(err)

	// Marshalling the onchain-secret structure, so that it can be stored
	// in a file or as a value in a keyvalue storage.
	forStorage, err := ocs.Marshal()
	log.ErrFatal(err)
	log.Info("Having", len(forStorage), "bytes for onchain-secret storage")

	// Unmarshalling the onchain-secret structure to access it
	ocsLoaded, err := onchain_secrets.NewOnchainSecretsUnmarshal(forStorage)
	fileID, err := writeFile(ocsLoaded, secretDoc)
	log.ErrFatal(err)

	// Getting the file off the skipchain
	data, err := readFile(ocsLoaded, fileID, "chaincode")
	log.ErrFatal(err)
	if bytes.Compare(secretDoc, data) != 0 {
		log.Fatal("Original data and retrieved data are not the same")
	}
	log.Info("Retrieved data:", string(data))

	// Reading at most 4 read-requests from the start
	requests, err := ocsLoaded.GetReadRequests(nil, 4)
	log.ErrFatal(err)
	for _, req := range requests {
		log.Infof("User %s read document %x", req.Reader, req.FileID)
	}
}

// setupChains creates the skipchains needed and adds two users:
//  - admin - with the right to add/remove users
//  - client - with the right to write to the skipchain
//  - chaincode - with the right to read from the skipchain
func setupChains(public string) (ocs *onchain_secrets.OnchainSecrets, err error) {
	group, err := app.ReadGroupDescToml(strings.NewReader(public))
	if err != nil {
		return
	}

	// In the next step we create a new ocs-skipchain with an admin-user
	// called 'admin'.
	log.Info("Setting up skipchains")
	ocs, err = onchain_secrets.NewOnchainSecrets(group.Roster, "admin")
	if err != nil {
		return
	}

	// Now we add two users:
	// client - with write access
	// chaincode - with read access
	log.Info("Adding users")
	if err = ocs.AddUser("client", onchain_secrets.UserWriter); err != nil {
		return
	}
	if err = ocs.AddUser("chaincode", onchain_secrets.UserReader); err != nil {
		return
	}
	return
}

// writeFile stores the data on the skipchain and returns a fileID that can
// be used to retrieve that data. fileID is a unique identifier over all
// the skipchain.
func writeFile(ocs *onchain_secrets.OnchainSecrets, data []byte) (fileID skipchain.SkipBlockID, err error) {
	// The client stores a file on the skipchain.
	log.Info("Encrypting file and sending it to the skipchain")
	// 1. Create a random symmetric key
	key := random.Bytes(32, random.Stream)
	// 2. Encrypt our data using that key
	cipher := sha3.NewShakeCipher128(key)
	encData := cipher.Seal(nil, data)
	// 3. Encrypt the key using the secret share public key
	encKey, cerr := ocs.EncryptKey(key)
	if cerr != nil {
		err = cerr
		return
	}
	// 4. Write the encrypted data with the encrypted key to the skipchain.
	fileID, err = ocs.AddFile(encData, encKey, "client")
	return
}

// readFile requests the data from the skipchain under the reader's name. If
// reader is not registered to the skipchain, the function will return an error.
func readFile(ocs *onchain_secrets.OnchainSecrets, idFile skipchain.SkipBlockID, reader string) ([]byte, error) {
	// Now the chaincode requests access to the file.
	log.Info("Send file-request")
	readRequest, err := ocs.RequestFile(idFile, reader)
	if err != nil {
		return nil, err
	}

	// Finally fetch the file (supposing we don't have it yet)
	log.Info("Get re-encrypted key")
	encData, key, err := ocs.ReadFile(readRequest)
	if err != nil {
		return nil, err
	}

	// And decrypt it
	log.Info("Decrypt the data")
	cipher := sha3.NewShakeCipher128(key)
	data, err := cipher.Open(nil, encData)
	if err != nil {
		return nil, err
	}
	return data, nil
}
