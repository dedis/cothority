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

	"github.com/dedis/cothority/network"
)

// User is a representation of the Users data in the code, and is used to store all relevant information
type User struct {
	// Name or UserID
	Name []byte
	// Salt used for the password-hash
	Salt []byte
	// Xored keys with hash
	Keys [][]byte
	// Data AEAD-encrypted with key
	Data []byte
}

// Database is a structure that stores Cothority(the list of guard servers), and a list of all users within the database
type Database struct {
	Cothority *sda.Roster
	Users     []User
}

var db *Database

var mastersalt []byte

func main() {
	network.RegisterMessageType(&Database{})
	salty := make([]byte, 12)
	rand.Read(salty)
	mastersalt = salty
	app := cli.NewApp()
	app.Name = "Guard"
	app.Usage = "Get and print status of all servers of a file."

	app.Flags = []cli.Flag{
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
	app.Commands = []cli.Command{
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
	app.Before = func(c *cli.Context) error {
		b, err := ioutil.ReadFile("config.bin")
		if os.IsNotExist(err) {
			return nil
		}
		log.ErrFatal(err, "The config.bin file threw an error")
		_, msg, err := network.UnmarshalRegistered(b)
		log.ErrFatal(err, "UnmarshalRegistered messeed up")
		var ok bool
		db, ok = msg.(*Database)
		if !ok {
			log.Fatal("We are not OK")
		}
		return nil
	}
	app.Run(os.Args)
}

func xor(x []byte, y []byte) []byte {
	z := make([]byte, len(x))

	for i := 0; i < len(x); i++ {
		z[i] = x[i] ^ y[i]
	}
	return z
}

// readGroup takes a toml file name and reads the file, returning the entities within
func readGroup(tomlFileName string) (*sda.Roster, error) {
	log.Print("Reading From File")
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

func set(c *cli.Context, UID []byte, epoch []byte, password string, userdata []byte) {
	k := make([]byte, 32)
	rand.Read(k)
	//pwhash is the password hash that will be sent to the guard servers
	pwhash := abstract.Sum(network.Suite, []byte(password), mastersalt)
	//secretkeys is the Shamir Secret share of the keys
	secretkeys := s.Create(2, len(db.Cothority.List), string(k))
	//salty is a list of salts that will be used to encrypt the pwhash before sending it to the guard, derived from a single
	//master salt
	salty := saltgen(mastersalt, len(db.Cothority.List))
	responses := make([][]byte, len(db.Cothority.List))
	keys := make([][]byte, len(db.Cothority.List))
	for i := 0; i < len(db.Cothority.List); i++ {
		cl := guard.NewClient()
		sendy := abstract.Sum(network.Suite, pwhash, salty[i])
		rep, err := cl.GetGuard(db.Cothority.List[i], UID, epoch, sendy)
		log.ErrFatal(err)
		responses[i] = rep.Msg
		keys[i] = xor([]byte(secretkeys[i]), responses[i])
	}
	//This is the code that seals the user data using the master key and saves it to the db
	block, _ := aes.NewCipher(k)
	aesgcm, _ := cipher.NewGCM(block)
	ciphertext := aesgcm.Seal(nil, mastersalt, userdata, nil)
	db.Users = append(db.Users, User{UID, mastersalt, keys, ciphertext})
}

// saltgen is a function that generates all the keys and salts given a length and an initial salt
func saltgen(salt []byte, count int) [][]byte {
	salts := make([][]byte, count)
	tlas := salt
	for i := 0; i < count; i++ {
		salts[i] = tlas
		tlas = abstract.Sum(network.Suite, tlas)
	}
	return salts
}

// setup when you setup the password
func setup(c *cli.Context) error {
	db = &Database{}
	groupToml := c.Args().First()
	log.Print("Setup is working")
	var err error
	t, err := readGroup(groupToml)
	db.Cothority = t
	log.ErrFatal(err)
	b, err := network.MarshalRegisteredType(db)
	log.ErrFatal(err)
	log.Print("Setup is working")
	err = ioutil.WriteFile("config.bin", b, 0660)
	log.ErrFatal(err)
	return nil
}

// getuser returns the user that the UID matches
func getuser(UID []byte) *User {

	for i := 0; i < len(db.Users); i++ {
		if bytes.Equal((db.Users[i]).Name, UID) {
			return &db.Users[i]
		}
	}
	//this is necessary because there needs to be a return at the end but
	return nil
}

// getpass contacts the guard servers, then gets the passwords and reconstructs the secret keys
func getpass(c *cli.Context, UID []byte, epoch []byte, pass string) {
	if getuser(UID) == nil {
		log.ErrFatal(nil, "Wrong Username")
	}
	pwhash := abstract.Sum(network.Suite, []byte(pass), getuser(UID).Salt)
	salty := saltgen(getuser(UID).Salt, len(db.Cothority.List))
	responses := make([]string, len(db.Cothority.List))
	keys := make([]string, len(db.Cothority.List))
	for i := 0; i < len(db.Cothority.List); i++ {
		cl := guard.NewClient()
		sendy := abstract.Sum(network.Suite, pwhash, salty[i])
		rep, err := cl.GetGuard(db.Cothority.List[i], UID, epoch, sendy)
		log.ErrFatal(err)
		responses[i] = string(rep.Msg)
		keys[i] = string(xor(getuser(UID).Keys[i], rep.Msg))
	}
	k := s.Combine(keys)
	if len([]byte(k)) == 0 {
		log.Fatal("You entered the wrong password")
	}
	block, err := aes.NewCipher([]byte(k))
	log.ErrFatal(err)
	aesgcm, err := cipher.NewGCM(block)
	log.ErrFatal(err)
	plaintext, err := aesgcm.Open(nil, getuser(UID).Salt, getuser(UID).Data, nil)
	log.ErrFatal(err)
	log.Print(string(plaintext))
}
func setpass(c *cli.Context) error {
	UID := []byte(c.Args().Get(0))
	Epoch := []byte{'E', 'P', 'O', 'C', 'H'}
	Pass := c.Args().Get(1)
	usrdata := []byte(c.Args().Get(2))
	set(c, UID, Epoch, string(Pass), usrdata)
	b, err := network.MarshalRegisteredType(db)
	log.ErrFatal(err)
	err = ioutil.WriteFile("config.bin", b, 0660)
	return nil
}
func get(c *cli.Context) error {
	UID := []byte(c.Args().Get(0))
	Epoch := []byte{'E', 'P', 'O', 'C', 'H'}
	Pass := c.Args().Get(1)
	getpass(c, UID, Epoch, string(Pass))
	return nil
}
