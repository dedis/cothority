// Guard is a service that provides additional password protection by creating a
// series of guard servers that allow a client to further secure their passwords
// from direct database compromises. The service is hash based and the passwords
// never leave the main database, making the guard servers very lightweight. The
// guard servers are used in both setting and authenticating passwords.

package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io/ioutil"
	"os"

	s "github.com/SSSaaS/sssa-golang"
	"github.com/dedis/cothority"
	"github.com/dedis/cothority/guard/service"
	"github.com/dedis/kyber/util/hash"
	"github.com/dedis/onet"
	"github.com/dedis/onet/app"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"gopkg.in/urfave/cli.v1"
)

// User is a representation of the Users data in the code, and is used to store
// all relevant information.
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

// Database is a structure that stores Cothority(the list of guard servers), and
// a list of all users within the database.
type Database struct {
	Cothority *onet.Roster
	Users     []User
}

// EPOCH is a constantg
const EPOCH = "EPOCH"

var db *Database

func main() {
	network.RegisterMessage(&Database{})

	cliApp := cli.NewApp()
	cliApp.Name = "Guard"
	cliApp.Usage = "Get and print status of all servers of a file."

	cliApp.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "group, gd",
			Value: "group.toml",
			Usage: "Cothority group definition in `FILE.toml`",
		},
		cli.IntFlag{
			Name:  "debug, d",
			Value: 0,
			Usage: "debug-level: `integer`: 1 for terse, 5 for maximal",
		},
	}
	cliApp.Commands = []cli.Command{
		{
			Name:    "setpass",
			Aliases: []string{"s"},
			Usage:   "Setup the configuration for the server (interactive)",
			Action:  setpass,
		},
		{
			Name:      "setup",
			Aliases:   []string{"su"},
			Usage:     "Saves the cothority group-toml to the configuration",
			ArgsUsage: "Give group definition",
			Action:    setup,
		},
		{
			Name:    "recover",
			Aliases: []string{"r"},
			Usage:   "Gets the password back from the guards",
			Action:  get,
		},
	}
	cliApp.Before = func(c *cli.Context) error {
		b, err := ioutil.ReadFile("config.bin")
		if os.IsNotExist(err) {
			return nil
		}
		log.ErrFatal(err, "The config.bin file threw an error")
		_, msg, err := network.Unmarshal(b, cothority.Suite)
		log.ErrFatal(err, "UnmarshalRegistered messeed up")
		var ok bool
		db, ok = msg.(*Database)
		if !ok {
			log.Fatal("The message was improperly converted")
		}
		return nil
	}
	cliApp.Run(os.Args)
}

// readGroup takes a toml file name and reads the file, returning the entities within.
func readGroup(tomlFileName string) (*onet.Roster, error) {
	log.Lvl2("Reading From File")
	f, err := os.Open(tomlFileName)
	if err != nil {
		return nil, err
	}
	el, err := app.ReadGroupToml(f, cothority.Suite)
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

func set(c *cli.Context, uid []byte, epoch []byte, password string, userdata []byte) {
	suite := cothority.Suite
	mastersalt := make([]byte, 12)
	_, err := rand.Read(mastersalt)
	log.ErrFatal(err)
	k := make([]byte, 32)
	_, err = rand.Read(k)
	log.ErrFatal(err)
	// secretkeys is the Shamir Secret share of the keys.
	secretkeys, err := s.Create(2, len(db.Cothority.List), string(k))
	log.ErrFatal(err)
	blind := make([]byte, 12)
	_, err = rand.Read(blind)
	log.ErrFatal(err)
	blinds := saltgen(blind, len(db.Cothority.List))
	iv := make([]byte, 16)
	_, err = rand.Read(iv)
	log.ErrFatal(err)

	// pwhash is the password hash that will be sent to the guard servers
	// with Gu and bi.
	pwhash, err := hash.Bytes(suite.Hash(), []byte(password), mastersalt)
	log.ErrFatal(err)
	GuHash, err := hash.Bytes(suite.Hash(), uid, epoch)
	log.ErrFatal(err)

	// creating stream for Scalar.Pick from the hash.
	blocky, err := aes.NewCipher(iv)
	log.ErrFatal(err)
	GuStream := cipher.NewCTR(blocky, iv)
	gupoint := suite.Point()
	if len(GuHash) > gupoint.EmbedLen() {
		panic("too much data in GuHash")
	}
	Gu := gupoint.Embed(GuHash, GuStream)
	responses := make([][]byte, len(db.Cothority.List))
	keys := make([][]byte, len(db.Cothority.List))
	for i, si := range db.Cothority.List {
		cl := guard.NewClient()
		// blankpoints needed to call the functions.
		blankpoint := suite.Point()
		blankscalar := suite.Scalar()
		// Initializing the variables pwbytes and blindbytes, which are
		// scalars with the values of pwhash and blinds[i].
		pwbytes := suite.Scalar()
		pwbytes.SetBytes(pwhash)
		blindbytes := suite.Scalar()
		blindbytes.SetBytes(blinds[i])
		// this next part performs all necessary computations to create
		// Xi, here called sendy.
		blankscalar.Add(pwbytes, blindbytes).Bytes()
		sendy := blankpoint.Mul(blankscalar.Mul(blindbytes, pwbytes), Gu)
		rep, cerr := cl.SendToGuard(si, uid, epoch, sendy)
		log.ErrFatal(cerr)
		reply := blankpoint.Mul(blankscalar.Inv(blindbytes), rep.Msg)
		responses[i], err = reply.MarshalBinary()
		log.ErrFatal(err)
		block, err := aes.NewCipher(responses[i])
		log.ErrFatal(err)
		stream := cipher.NewCTR(block, iv)
		msg := make([]byte, 88)
		stream.XORKeyStream(msg, []byte(secretkeys[i]))
		keys[i] = msg

	}
	// This is the code that seals the user data using the master key
	// and saves it to the db.
	block, _ := aes.NewCipher(k)
	aesgcm, _ := cipher.NewGCM(block)
	ciphertext := aesgcm.Seal(nil, mastersalt, userdata, nil)
	db.Users = append(db.Users, User{uid, mastersalt, keys, ciphertext, iv})
}

// saltgen is a function that generates all the keys and salts given a length
// and an initial salt.
func saltgen(salt []byte, count int) [][]byte {
	salts := make([][]byte, count)
	for i := 0; i < count; i++ {
		salts[i] = salt
		salt, _ = hash.Bytes(cothority.Suite.Hash(), salt)
	}
	return salts
}

// setup is called when you setup the password database.
func setup(c *cli.Context) error {
	groupToml := c.Args().First()
	var err error
	t, err := readGroup(groupToml)
	db = &Database{
		Cothority: t,
	}
	log.ErrFatal(err)
	b, err := network.Marshal(db)
	log.ErrFatal(err)
	err = ioutil.WriteFile("config.bin", b, 0660)
	log.ErrFatal(err)
	return nil
}

// getuser returns the user that the UID matches.
func getuser(UID []byte) *User {
	for _, u := range db.Users {
		if bytes.Equal(u.Name, UID) {
			return &u
		}
	}
	return nil
}

// getpass contacts the guard servers, then gets the passwords and
// reconstructs the secret keys.
func getpass(c *cli.Context, uid []byte, epoch []byte, pass string) {
	suite := cothority.Suite
	if getuser(uid) == nil {
		log.Fatal("Wrong username")
	}

	keys := make([]string, len(db.Cothority.List))
	blind := make([]byte, 12)
	_, err := rand.Read(blind)
	log.ErrFatal(err)
	blinds := saltgen(blind, len(db.Cothority.List))
	iv := getuser(uid).Iv

	// pwhash is the password hash that will be sent to the guard servers
	// with Gu and bi.
	pwhash, err := hash.Bytes(suite.Hash(), []byte(pass), getuser(uid).Salt)
	log.ErrFatal(err)
	GuHash, err := hash.Bytes(suite.Hash(), uid, epoch)
	log.ErrFatal(err)

	// creating stream for Scalar.Pick from the hash
	blocky, err := aes.NewCipher(iv)
	log.ErrFatal(err)
	GuStream := cipher.NewCTR(blocky, iv)
	gupoint := suite.Point()
	if len(GuHash) > gupoint.EmbedLen() {
		panic("GuHash is too big.")
	}
	Gu := gupoint.Embed(GuHash, GuStream)

	for i, si := range db.Cothority.List {
		cl := guard.NewClient()
		// blankpoints needed for computations.
		blankpoint := suite.Point()
		blankscalar := suite.Scalar()
		// pwbytes and blindbytes are actually scalars that are
		// initialized to the values of the bytes.
		pwbytes := suite.Scalar()
		pwbytes.SetBytes(pwhash)
		blindbytes := suite.Scalar()
		blindbytes.SetBytes(blinds[i])
		// The following sections of the code perform the computations
		// to Create Xi, here called sendy.
		blankscalar.Add(pwbytes, blindbytes).MarshalBinary()
		_, err := blankpoint.Mul(blankscalar.Mul(blindbytes, pwbytes), Gu).MarshalBinary()
		sendy := blankpoint.Mul(blankscalar.Mul(blindbytes, pwbytes), Gu)
		rep, cerr := cl.SendToGuard(si, uid, epoch, sendy)
		log.ErrFatal(cerr)
		// This section of the program removes the blinding factor from
		// the Zi for storage.
		reply, err := blankpoint.Mul(blankscalar.Inv(blindbytes), rep.Msg).MarshalBinary()
		log.ErrFatal(err)
		// This section Xors the data with the response.
		block, err := aes.NewCipher(reply)
		stream := cipher.NewCTR(block, getuser(uid).Iv)
		msg := make([]byte, len([]byte(getuser(uid).Keys[i])))
		stream.XORKeyStream(msg, []byte(getuser(uid).Keys[i]))
		keys[i] = string(msg)
	}
	k, err := s.Combine(keys)
	log.ErrFatal(err)
	if len(k) == 0 {
		log.Fatal("You entered the wrong password")
	}
	block, err := aes.NewCipher([]byte(k))
	log.ErrFatal(err)
	aesgcm, err := cipher.NewGCM(block)
	log.ErrFatal(err)
	plaintext, err := aesgcm.Open(nil, getuser(uid).Salt, getuser(uid).Data, nil)
	log.ErrFatal(err)
	log.Print(string(plaintext))
}
func setpass(c *cli.Context) error {
	uid := []byte(c.Args().Get(0))
	Pass := c.Args().Get(1)
	usrdata := []byte(c.Args().Get(2))
	set(c, uid, []byte(EPOCH), string(Pass), usrdata)
	b, err := network.Marshal(db)
	log.ErrFatal(err)
	err = ioutil.WriteFile("config.bin", b, 0660)
	return nil
}
func get(c *cli.Context) error {
	uid := []byte(c.Args().Get(0))
	pass := c.Args().Get(1)
	getpass(c, uid, []byte(EPOCH), string(pass))
	return nil
}
