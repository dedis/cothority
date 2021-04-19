package main

import (
	"encoding/hex"
	"fmt"
	"github.com/urfave/cli"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/byzcoin/bcadmin/lib"
	"go.dedis.ch/cothority/v3/personhood/user"
	"go.dedis.ch/onet/v3/log"
	"golang.org/x/xerrors"
	"regexp"
)

func userNew(c *cli.Context) error {
	if c.NArg() < 3 {
		return xerrors.New("please give: bc-xxx.cfg key-xxx.cfg user_name")
	}

	cfg, cl, signer, _, _, err := getBcKey(c)
	if err != nil {
		return xerrors.Errorf("while fetching configuration: %v", err)
	}
	name := c.Args().Get(2)

	newUser, err := user.NewFromByzcoin(cl, cfg.AdminDarc.GetBaseID(),
		*signer, name)
	if err != nil {
		return xerrors.Errorf("while creating new user: %v", err)
	}

	url := fmt.Sprintf("%s?credentialIID=%s&ephemeral=%s",
		c.String("base-url"), newUser.CredIID, newUser.Signer.Ed25519.Secret.String())
	log.Infof("URL for new user is:\n%s", url)

	return nil
}

func userShow(c *cli.Context) error {
	if c.NArg() < 2 {
		return xerrors.New("please give: bc-xxx.cfg credentialIID")
	}

	_, cl, err := lib.LoadConfig(c.Args().First())
	if err != nil {
		return xerrors.Errorf("while loading configuration: %v", err)
	}
	if len(c.Args().Get(1)) != 64 {
		return xerrors.New("credentialIID should be a hex-string of 32 bytes")
	}
	credID, err := hex.DecodeString(c.Args().Get(1))
	if err != nil {
		return xerrors.Errorf("couldn't decode credentialID")
	}

	userCred, err := user.New(cl, byzcoin.NewInstanceID(credID))
	if err != nil {
		return xerrors.Errorf("while fetching user from chain: %v", err)
	}

	log.Infof("User is:\n%s", userCred)

	return nil
}

func userConnect(c *cli.Context) error {
	if c.NArg() < 2 {
		return xerrors.New("please give: bc-xxx.cfg http[s]://...")
	}
	cfg, cl, err := lib.LoadConfig(c.Args().First())
	if err != nil {
		return xerrors.Errorf("couldn't load config file: %v", err)
	}

	url := c.Args().Get(1)
	credentialIIDStr := regexp.MustCompile("credentialIID=([a-f0-9]{64})").
		FindStringSubmatch(url)
	ephemeralStr := regexp.MustCompile("ephemeral=([a-f0-9]{64})").
		FindStringSubmatch(url)
	if len(credentialIIDStr) == 0 ||
		len(ephemeralStr) == 0 {
		return xerrors.New("not a URL for a new device")
	}

	credentialIID, err := hex.DecodeString(credentialIIDStr[1])
	if err != nil {
		return xerrors.Errorf("couldn't decode credentialIID: %v", err)
	}
	ephemeralBuf, err := hex.DecodeString(ephemeralStr[1])
	if err != nil {
		return xerrors.Errorf("couldn't decode ephemeral: %v", err)
	}
	ephemeralKey := cothority.Suite.Scalar()
	if err := ephemeralKey.UnmarshalBinary(ephemeralBuf); err != nil {
		return xerrors.Errorf("while creating ephemeralKey: %v", err)
	}

	log.Info("Fetching user credentials")
	userCred, err := user.New(cl, byzcoin.NewInstanceID(credentialIID))
	if err != nil {
		return xerrors.Errorf("while fetching user: %v", err)
	}

	log.Info("Switching to a new key")
	if err := userCred.DeviceSwitchKey(ephemeralKey); err != nil {
		return xerrors.Errorf("couldn't switch key: %v", err)
	}

	log.Info("Saving configuration and key")
	cfg.AdminIdentity = userCred.Signer.Identity()
	if _, err := lib.SaveConfig(cfg); err != nil {
		return xerrors.Errorf("saving config: %v", err)
	}
	if err := lib.SaveKey(userCred.Signer); err != nil {
		return xerrors.Errorf("saving signer: %v", err)
	}

	return nil
}
