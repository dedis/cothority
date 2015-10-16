package cliutils

import (
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
	"github.com/dedis/crypto/random"
	"io/ioutil"
	"os"
	"strings"
)

// This file manage every operations related to keys
// KeyPair will generate a keypair (private + public key) from a given suite
func KeyPair(s abstract.Suite) config.KeyPair {
	kp := config.KeyPair{}
	kp.Gen(s, random.Stream)
	return kp
}

// WritePrivKey will write the private key into the filename given
// It takes a suite in order to adequatly write the secret
// Returns an error if anything went wrong during file handling or writing key
func WritePrivKey(priv abstract.Secret, suite abstract.Suite, fileName string) error {
	// Opening file
	privFile, err := os.OpenFile(fileName, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0744)
	if err != nil {
		return err
	}
	defer privFile.Close()

	// Writing down !

	err = suite.Write(privFile, priv)
	if err != nil {
		return err
	}
	return nil
}

// WritePubKey will write the public key into the filename using the suite
// 'prepend' is if you want to write something before the actual key like in ssh
// format hostname KEY_in_base_64
// if before contains a space it will throw an error
// Returns an error if anything went wrong during file handling or writing key
func WritePubKey(pub abstract.Point, suite abstract.Suite, fileName string, prepend string) error {

	pubFile, err := os.OpenFile(fileName, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0744)
	if err != nil {
		return err
	}
	defer pubFile.Close()

	if strings.Contains(prepend, " ") {
		return errors.New("The string to insert before public key contains some space. Invalid !")
	}
	pubFile.WriteString(prepend + " ")

	encoder := base64.NewEncoder(base64.StdEncoding, pubFile)
	err = suite.Write(encoder, pub)
	if err != nil {
		return err
	}
	encoder.Close()

	return nil
}

// ReadPrivKey will read the file and decrypt the private key inside
// It takes a suite to decrypt and a filename to know where to read
// Returns the secret and an error if anything wrong occured
func ReadPrivKey(suite abstract.Suite, fileName string) (abstract.Secret, error) {
	secret := suite.Secret()
	// Opening files
	privFile, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer privFile.Close()

	// Read the keys
	err = suite.Read(privFile, &secret)
	if err != nil {
		return nil, err
	}
	return secret, nil
}

// ReadPubKey will read the file and decrypt the public key inside
// It takes a suite to decrypt and a file name
// Returns the public key, whatever text is in front and an error if anything went wrong
func ReadPubKey(suite abstract.Suite, fileName string) (abstract.Point, string, error) {

	public := suite.Point()
	// Opening files
	pubFile, err := os.Open(fileName)
	if err != nil {
		return nil, "", err
	}
	defer pubFile.Close()

	// read the string before
	by, err := ioutil.ReadAll(pubFile)
	if err != nil {
		return nil, "", errors.New(fmt.Sprintf("Error reading the whole file  %s", err))
	}
	splits := strings.Split(string(by), " ")
	if len(splits) != 2 {
		return nil, "", errors.New(fmt.Sprintf("Error reading pub key file format is not correct (val space val)"))
	}

	before := splits[0]
	key := strings.NewReader(splits[1])

	// Some readings
	dec := base64.NewDecoder(base64.StdEncoding, key)
	err = suite.Read(dec, &public)
	if err != nil {
		return nil, "", errors.New(fmt.Sprintf("Error reading the public key itself : %s", err))
	}

	return public, before, nil

}
