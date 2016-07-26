// This is the Guard Service that is going to be integrated with the C code in the Openldap
//This code will be broken down into two functions, checkfunc and hashfunc, both of which will be identical. The only
//difficult thing will be integrating all of the information needed within the database, as Openldap will not have
//things like IV's and salts

package main

import (
	"os"

	"errors"

	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"

	"github.com/dedis/cothority/app/lib/config"
	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/sda"
	"github.com/dedis/cothority/services/guard"
	"github.com/dedis/crypto/abstract"

	"io/ioutil"

	s "github.com/SSSaaS/sssa-golang"
	"gopkg.in/codegangsta/cli.v1"

	"bytes"

	"C"

	"github.com/dedis/cothority/network"
)

// User is a representation of the Users data in the code, and is used to store all relevant information.
type User struct {
	// Name or UserID
	Name []byte
	// Salt used for the password-hash
	Salt []byte
	// Xored keys with hash
	Keys [][]byte
	// Data AEAD-encrypted with key
	Data []byte
	Iv   []byte
}

// Database is a structure that stores Cothority(the list of guard servers), and a list of all users within the database.
type Database struct {
	Cothority *sda.Roster
	Users     []User
}

var db *Database

// readGroup takes a toml file name and reads the file, returning the entities within.
func readGroup(tomlFileName string) (*sda.Roster, error) {
	log.Lvl2("Reading From File")
	f, err := os.Open(tomlFileName)
	if err != nil {
		return nil, err
	}
	el, err := config.ReadGroupToml(f)
	if err != nil {
		return nil, err
	}
	if len(el.List) <= 0 {
		return nil, errors.New("Empty or invalid group file:" +
			tomlFileName)
	}
	log.Lvl3(el)
	return el, err
}

func set(uid []byte, epoch []byte, password string, userdata []byte) {
	mastersalt := make([]byte, 12)
	rand.Read(mastersalt)
	k := make([]byte, 32)
	rand.Read(k)
	// secretkeys is the Shamir Secret share of the keys.
	secretkeys := s.Create(2, len(db.Cothority.List), string(k))
	blind := make([]byte, 12)
	rand.Read(blind)
	blinds := saltgen(blind, len(db.Cothority.List))
	iv := make([]byte, 16)
	rand.Read(iv)
	// pwhash is the password hash that will be sent to the guard servers with Gu and bi.
	pwhash := abstract.Sum(network.Suite, []byte(password), mastersalt)
	GuHash := abstract.Sum(network.Suite, uid, epoch)
	// creating stream for Scalar.Pick from the hash.
	blocky, err := aes.NewCipher(iv)
	log.ErrFatal(err)
	log.Lvl2("Blocky: ", blocky)
	log.Lvl2("Iv: ", iv)
	log.Lvl2("SetHash: ", pwhash)
	GuStream := cipher.NewCTR(blocky, iv)
	gupoint := network.Suite.Point()
	Gu, _ := gupoint.Pick(GuHash, GuStream)
	// This is a test to see whether Gu is working
	gudat, _ := Gu.MarshalBinary()
	log.Lvl2("SetGu: ", gudat)

	responses := make([][]byte, len(db.Cothority.List))
	keys := make([][]byte, len(db.Cothority.List))
	for i, si := range db.Cothority.List {
		cl := guard.NewClient()
		// blankpoints needed to call the functions
		blankpoint := network.Suite.Point()
		blankscalar := network.Suite.Scalar()
		// Initializing the variables pwbytes and blindbytes, which are scalars with the values of pwhash and blinds[i].
		pwbytes := network.Suite.Scalar()
		pwbytes.SetBytes(pwhash)
		a, _ := blankscalar.MarshalBinary()
		log.Lvl2("BlankScalar: ", a)
		blindbytes := network.Suite.Scalar()
		blindbytes.SetBytes(blinds[i])
		b, _ := blankscalar.MarshalBinary()
		log.Lvl2("BlankScalar: ", b)
		log.Lvl2("pwbytes: ", pwbytes)
		log.Lvl2("blindbytes: ", blindbytes)
		log.Lvl2("Pwhash: ", pwhash)
		log.Lvl2("blinds: ", blinds[i])
		// this next part performs all necessary computations to create Xi, here called sendy.
		ad := blankscalar.Add(pwbytes, blindbytes).Bytes()
		log.Lvl2("Addition: ", ad)
		log.Lvl2("BlankScalar: ", blankscalar.Bytes())
		sendy := blankpoint.Mul(Gu, blankscalar.Mul(pwbytes, blindbytes))
		rep, err := cl.GetGuard(si, uid, epoch, sendy)
		log.ErrFatal(err)
		log.Lvl2("SetRep: ", rep.Msg)
		reply := blankpoint.Mul(rep.Msg, blankscalar.Inv(blindbytes))
		responses[i], err = reply.MarshalBinary()
		log.ErrFatal(err)

		block, err := aes.NewCipher(responses[i])
		if err != nil {
			panic(err)
		}
		stream := cipher.NewCTR(block, iv)
		msg := make([]byte, 88)
		stream.XORKeyStream(msg, []byte(secretkeys[i]))
		keys[i] = msg

	}
	log.Lvl2("key: ", k)
	// This is the code that seals the user data using the master key and saves it to the db.
	block, _ := aes.NewCipher(k)
	aesgcm, _ := cipher.NewGCM(block)
	ciphertext := aesgcm.Seal(nil, mastersalt, userdata, nil)
	db.Users = append(db.Users, User{uid, mastersalt, keys, ciphertext, iv})
}

// saltgen is a function that generates all the keys and salts given a length and an initial salt.
func saltgen(salt []byte, count int) [][]byte {
	salts := make([][]byte, count)
	for i := 0; i < count; i++ {
		salts[i] = salt
		salt = abstract.Sum(network.Suite, salt)
	}
	return salts
}

// setup is called when you setup the password database.
func setup(c *cli.Context) error {

	groupToml := c.Args().First()
	log.Lvl2("Setup is working")
	var err error
	t, err := readGroup(groupToml)
	db = &Database{
		Cothority: t,
	}
	log.ErrFatal(err)
	b, err := network.MarshalRegisteredType(db)
	log.ErrFatal(err)
	log.Lvl2("Setup is working")
	err = ioutil.WriteFile("config.bin", b, 0660)
	log.ErrFatal(err)
	return nil
}

// getuser returns the user that the UID matches
func getuser(UID []byte) *User {
	for _, u := range db.Users {
		if bytes.Equal(u.Name, UID) {
			return &u
		}
	}
	return nil
}

// getpass contacts the guard servers, then gets the passwords and reconstructs the secret keys
func getpass(uid []byte, epoch []byte, pass string) int {
	if getuser(uid) == nil {
		log.Fatal("Wrong username")
	}

	keys := make([]string, len(db.Cothority.List))

	blind := make([]byte, 12)
	rand.Read(blind)
	blinds := saltgen(blind, len(db.Cothority.List))
	iv := getuser(uid).Iv
	// pwhash is the password hash that will be sent to the guard servers with Gu and bi
	pwhash := abstract.Sum(network.Suite, []byte(pass), getuser(uid).Salt)
	GuHash := abstract.Sum(network.Suite, uid, epoch)
	// creating stream for Scalar.Pick from the hash
	// creating stream for Scalar.Pick from the hash
	blocky, err := aes.NewCipher(iv)
	log.ErrFatal(err)
	log.Lvl2("Blocky: ", blocky)
	log.Lvl2("Iv: ", iv)
	log.Lvl2("GetHash: ", pwhash)
	GuStream := cipher.NewCTR(blocky, iv)
	gupoint := network.Suite.Point()
	Gu, _ := gupoint.Pick(GuHash, GuStream)
	// printing gudat is just a test
	Gudat, _ := Gu.MarshalBinary()
	log.Lvl2("GetGu: ", Gudat)

	for i, si := range db.Cothority.List {
		cl := guard.NewClient()
		// blankpoints needed for computations
		blankpoint := network.Suite.Point()
		blankscalar := network.Suite.Scalar()
		// pwbytes and blindbytes are actually scalars that are initialized to the values of the bytes
		pwbytes := network.Suite.Scalar()
		pwbytes.SetBytes(pwhash)
		a, _ := blankscalar.MarshalBinary()
		log.Lvl2("BlankScalar: ", a)
		blindbytes := network.Suite.Scalar()
		blindbytes.SetBytes(blinds[i])
		b, _ := blankscalar.MarshalBinary()
		// The following sections of the code perform the computations to Create Xi, here called sendy.
		log.Lvl2("BlankScalar: ", b)
		log.Lvl2("getpwhash: ", pwhash)
		log.Lvl2("getblind: ", blinds[i])
		ad, _ := blankscalar.Add(pwbytes, blindbytes).MarshalBinary()
		log.Lvl2("GetAddition: ", ad)
		mul, err := blankpoint.Mul(Gu, blankscalar.Mul(pwbytes, blindbytes)).MarshalBinary()
		log.Lvl2("GetMul: ", mul)
		sendy := blankpoint.Mul(Gu, blankscalar.Mul(pwbytes, blindbytes))
		rep, err := cl.GetGuard(si, uid, epoch, sendy)
		log.Lvl2("GetRep: ", rep.Msg)
		log.ErrFatal(err)
		// This section of the program removes the blinding factor from the Zi for storage
		reply, err := blankpoint.Mul(rep.Msg, blankscalar.Inv(blindbytes)).MarshalBinary()
		log.ErrFatal(err)
		log.Lvl2("Reply: ", reply)
		// This section Xors the data with the response
		block, err := aes.NewCipher(reply)
		stream := cipher.NewCTR(block, getuser(uid).Iv)
		msg := make([]byte, len([]byte(getuser(uid).Keys[i])))
		stream.XORKeyStream(msg, []byte(getuser(uid).Keys[i]))
		keys[i] = string(msg)
	}
	k := s.Combine(keys)
	if len(k) == 0 {
		log.Fatal("You entered the wrong password")
	}
	log.Lvl2("key: ", []byte(k))
	block, err := aes.NewCipher([]byte(k))
	log.ErrFatal(err)
	aesgcm, err := cipher.NewGCM(block)
	log.ErrFatal(err)
	_, err = aesgcm.Open(nil, getuser(uid).Salt, getuser(uid).Data, nil)
	outp := 0
	if err == nil {
		outp = 1
	}
	return outp
}

//export setpass
// setpass
func setpass(UID []byte, Pass []byte, userdata []byte) {
	Epoch := []byte{'E', 'P', 'O', 'C', 'H'}
	set(UID, Epoch, string(Pass), userdata)
	b, err := network.MarshalRegisteredType(db)
	log.ErrFatal(err)
	err = ioutil.WriteFile("config.bin", b, 0660)
}

//export get
// get
func get(UID []byte, Pass []byte, userdata []byte) error {
	epoch := []byte{'E', 'P', 'O', 'C', 'H'}
	i := getpass(UID, epoch, string(Pass))
	return nil
}
