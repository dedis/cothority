package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	oidc "github.com/coreos/go-oidc"
	"github.com/pkg/browser"
	cli "github.com/urfave/cli"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/authprox"
	"go.dedis.ch/cothority/v3/byzcoin"
	bcadminlib "go.dedis.ch/cothority/v3/byzcoin/bcadmin/lib"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/cothority/v3/eventlog"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/share"
	"go.dedis.ch/kyber/v3/sign/dss"
	"go.dedis.ch/kyber/v3/sign/schnorr"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/cfgpath"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
	"go.dedis.ch/protobuf"
	"golang.org/x/oauth2"
)

type config struct {
	Name       string
	EventLogID byzcoin.InstanceID
}

type bcConfig struct {
	Roster    onet.Roster
	ByzCoinID skipchain.SkipBlockID
}

// Presets for -clientid and -clientsecret.
var clientIDs = map[string]string{
	"https://accounts.google.com": "742239812619-g1rqb2esv99gplco7chck7ir3c22g4pf.apps.googleusercontent.com",
	"https://oauth.dedis.ch/dex":  "dedis",
}
var clientSecrets = map[string]string{
	"https://accounts.google.com": "wYLW80agBpK-EyuXzKqEwieK",
	"https://oauth.dedis.ch/dex":  "6143443e4635074ddef90ac7bc71443ceed7e6df",
}

var cmds = cli.Commands{
	{
		Name:    "create",
		Usage:   "create an event log",
		Aliases: []string{"c"},
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "sign",
				Usage: "the ed25519 private key that will sign the create transaction",
			},
			cli.StringFlag{
				Name:   "bc",
				EnvVar: "BC",
				Usage:  "the ByzCoin config",
			},
			cli.StringFlag{
				Name:  "darc",
				Usage: "the DarcID that has the spawn:evenlog rule (default is the genesis DarcID)",
			},
		},
		Action: create,
	},
	{
		Name:  "login",
		Usage: "login using OpenID Connect",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:   "bc",
				EnvVar: "BC",
				Usage:  "the ByzCoin config",
			},
			cli.StringFlag{
				Name:  "issuer",
				Usage: "the issuer URL",
				Value: "https://oauth.dedis.ch/dex",
			},
			cli.StringFlag{
				Name:  "clientsecret",
				Usage: "the client secret",
			},
			cli.StringFlag{
				Name:  "clientid",
				Usage: "the client id",
			},
		},
		Action: login,
	},
	{
		Name:    "log",
		Usage:   "log one or more messages",
		Aliases: []string{"l"},
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "sign",
				Usage: "the ed25519 private key that will sign transactions",
			},
			cli.StringFlag{
				Name:   "bc",
				EnvVar: "BC",
				Usage:  "the ByzCoin config",
			},
			cli.StringFlag{
				Name:   "el",
				EnvVar: "EL",
				Usage:  "the eventlog id, from \"el create\"",
			},
			cli.StringFlag{
				Name:  "topic, t",
				Usage: "the topic of the log",
			},
			cli.StringFlag{
				Name:  "content, c",
				Usage: "the text of the log",
			},
			cli.IntFlag{
				Name:  "wait, w",
				Usage: "wait for block inclusion (default: do not wait)",
				Value: 0,
			},
		},
		Action: doLog,
	},
	{
		Name:    "search",
		Usage:   "search for messages",
		Aliases: []string{"s"},
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:   "bc",
				EnvVar: "BC",
				Usage:  "the ByzCoin config",
			},
			cli.StringFlag{
				Name:   "el",
				EnvVar: "EL",
				Usage:  "the eventlog id, from \"el create\"",
			},
			cli.StringFlag{
				Name:  "topic, t",
				Usage: "limit results to logs with this topic",
			},
			cli.IntFlag{
				Name:  "count, c",
				Usage: "limit results to X events",
			},
			cli.StringFlag{
				Name:  "from",
				Usage: "return events from this time (accepts mm-dd-yyyy or relative times like '10m ago')",
			},
			cli.StringFlag{
				Name:  "to",
				Usage: "return events to this time (accepts mm-dd-yyyy or relative times like '10m ago')",
			},
			cli.DurationFlag{
				Name:  "for",
				Usage: "return events for this long after the from time (when for is given, to is ignored)",
			},
		},
		Action: search,
	},
	{
		Name:    "key",
		Usage:   "generates a new keypair and prints the public key in the stdout",
		Aliases: []string{"k"},
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "save",
				Usage: "file in which the user wants to save the public key instead of printing it",
			},
			cli.StringFlag{
				Name:  "print",
				Usage: "print the private and public key",
			},
		},
		Action: key,
	},
}

var cliApp = cli.NewApp()
var dataDir = ""
var gitTag = "dev"

func init() {
	cliApp.Name = "el"
	cliApp.Usage = "Create and work with event logs."
	cliApp.Version = gitTag
	cliApp.Commands = cmds
	cliApp.Flags = []cli.Flag{
		cli.IntFlag{
			Name:  "debug, d",
			Value: 0,
			Usage: "debug-level: 1 for terse, 5 for maximal",
		},
		cli.StringFlag{
			Name:  "config, c",
			Value: cfgpath.GetDataPath(cliApp.Name),
			Usage: "path to configuration-directory",
		},
	}
	cliApp.Before = func(c *cli.Context) error {
		log.SetDebugVisible(c.Int("debug"))
		bcadminlib.ConfigPath = c.String("config")
		return nil
	}

	network.RegisterMessage(&openidCfg{})
}

func main() {
	rand.Seed(time.Now().UTC().UnixNano())
	log.ErrFatal(cliApp.Run(os.Args))
}

// getClient will create a new eventlog.Client, given the input
// available in the commandline. If priv is false, then it will not
// look for a private key and set up the signers. (This is used for
// searching, which does not require having a private key available
// because it does not submit transactions.)
func getClient(c *cli.Context, priv bool) (*eventlog.Client, error) {
	bc := c.String("bc")
	if bc == "" {
		return nil, errors.New("--bc flag is required")
	}

	cfgBuf, err := ioutil.ReadFile(bc)
	if err != nil {
		return nil, err
	}
	var cfg bcConfig
	err = protobuf.DecodeWithConstructors(cfgBuf, &cfg,
		network.DefaultConstructors(cothority.Suite))
	if err != nil {
		return nil, err
	}

	cl := eventlog.NewClient(byzcoin.NewClient(cfg.ByzCoinID, cfg.Roster))

	d, err := cl.ByzCoin.GetGenDarc()
	if err != nil {
		return nil, err
	}
	cl.DarcID = d.GetBaseID()

	// The caller doesn't want/need signers.
	if !priv {
		return cl, nil
	}

	// First look for a valid OpenID Config. If we've got it to get the Signer.
	ocfg, err := load()
	if err == nil {
		s, err := ocfg.getSigners(cl)
		if err == nil {
			cl.Signers = s
		} else {
			return nil, fmt.Errorf("could not make OpenID signer: %v", err)
		}
	} else {
		// Otherwise, get the private key from the cmdline.
		sstr := c.String("sign")
		if sstr == "" {
			return nil, errors.New("--sign is required")
		}
		signer, err := bcadminlib.LoadKeyFromString(sstr)
		if err != nil {
			return nil, err
		}
		cl.Signers = []darc.Signer{*signer}
	}
	return cl, nil
}

func create(c *cli.Context) error {
	cl, err := getClient(c, true)
	if err != nil {
		return err
	}

	e := c.String("darc")
	if e == "" {
		genDarc, err := cl.ByzCoin.GetGenDarc()
		if err != nil {
			return err
		}
		cl.DarcID = genDarc.GetBaseID()
	} else {
		eb, err := bcadminlib.StringToDarcID(e)
		if err != nil {
			return err
		}
		cl.DarcID = darc.ID(eb)
	}

	err = cl.Create()
	if err != nil {
		return err
	}

	log.Infof("export EL=%x", cl.Instance.Slice())
	return bcadminlib.WaitPropagation(c, cl.ByzCoin)
}

func doLog(c *cli.Context) error {
	cl, err := getClient(c, true)
	if err != nil {
		return err
	}
	e := c.String("el")
	if e == "" {
		return errors.New("--el is required")
	}
	eb, err := hex.DecodeString(e)
	if err != nil {
		return err
	}
	cl.Instance = byzcoin.NewInstanceID(eb)

	t := c.String("topic")
	content := c.String("content")
	w := c.Int("wait")

	// Content is set, so one shot log.
	if content != "" {
		_, err := cl.LogAndWait(w, eventlog.NewEvent(t, content))
		return err
	}

	// Content is empty, so read from stdin.
	s := bufio.NewScanner(os.Stdin)
	for s.Scan() {
		_, err := cl.LogAndWait(w, eventlog.NewEvent(t, s.Text()))
		if err != nil {
			return err
		}
	}
	return bcadminlib.WaitPropagation(c, cl.ByzCoin)
}

var none = time.Unix(0, 0)

// parseTime will accept either dates or "X ago" where X is a duration.
func parseTime(in string) (time.Time, error) {
	if strings.HasSuffix(in, " ago") {
		in = strings.Replace(in, " ago", "", -1)
		d, err := time.ParseDuration(in)
		if err != nil {
			return none, err
		}
		return time.Now().Add(-1 * d), nil
	}
	tm, err := time.Parse("2006-01-02", in)
	if err != nil {
		return none, err
	}
	return tm, nil
}

func search(c *cli.Context) error {
	req := &eventlog.SearchRequest{
		Topic: c.String("topic"),
	}

	f := c.String("from")
	if f != "" {
		ft, err := parseTime(f)
		if err != nil {
			return err
		}
		req.From = ft.UnixNano()
	}

	forDur := c.Duration("for")
	if forDur == 0 {
		// No -for, parse -to.
		t := c.String("to")
		if t != "" {
			tt, err := parseTime(t)
			if err != nil {
				return err
			}
			req.To = tt.UnixNano()
		}
	} else {
		// Parse -for
		req.To = time.Unix(0, req.From).Add(forDur).UnixNano()
	}

	cl, err := getClient(c, false)
	if err != nil {
		return err
	}
	e := c.String("el")
	if e == "" {
		return errors.New("--el is required")
	}
	eb, err := hex.DecodeString(e)
	if err != nil {
		return err
	}
	cl.Instance = byzcoin.NewInstanceID(eb)

	resp, err := cl.Search(req)
	if err != nil {
		return err
	}

	ct := c.Int("count")

	for _, x := range resp.Events {
		const tsFormat = "2006-01-02 15:04:05"
		log.Infof("%v\t%v\t%v", time.Unix(0, x.When).Format(tsFormat), x.Topic, x.Content)

		if ct != 0 {
			ct--
			if ct == 0 {
				break
			}
		}
	}

	if resp.Truncated {
		return cli.NewExitError("", 1)
	}
	return nil
}

func login(c *cli.Context) error {
	is := c.String("issuer")
	if is == "" {
		return errors.New("--issuer flag is required")
	}
	bc := c.String("bc")
	if bc == "" {
		return errors.New("--bc flag is required")
	}

	// If these are not set, then set them out of the pre-sets, based on the
	// issuer. If the issuer is not found, they will be set back to "", which
	// will not work, but is no worse than before this code ran.
	cid := c.String("clientid")
	if cid == "" {
		cid = clientIDs[is]
	}
	csec := c.String("clientsecret")
	if csec == "" {
		csec = clientSecrets[is]
	}

	ctx := context.Background()
	p, err := oidc.NewProvider(ctx, is)
	if err != nil {
		return err
	}

	// This stuff was taken from github.com/dexidp/dex/cmd/example-app.
	var s struct {
		// What scopes does a provider support?
		//
		// See: https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata
		ScopesSupported []string `json:"scopes_supported"`
	}
	if err := p.Claims(&s); err != nil {
		return fmt.Errorf("failed to parse provider scopes_supported: %v", err)
	}

	log.Info("scopes supported", s.ScopesSupported)

	hasOffline := func() bool {
		for _, scope := range s.ScopesSupported {
			if scope == oidc.ScopeOfflineAccess {
				return true
			}
		}
		return false
	}()

	var scopes []string
	if len(s.ScopesSupported) == 0 || hasOffline {
		// scopes_supported is a "RECOMMENDED" discovery claim, not a required
		// one. If missing, assume that the provider follows the spec and has
		// an "offline_access" scope.
		scopes = []string{oidc.ScopeOfflineAccess}
	}
	scopes = append(scopes, "openid", "email")

	cfg := &oauth2.Config{
		ClientID:     cid,
		ClientSecret: csec,
		Endpoint:     p.Endpoint(),
		Scopes:       scopes,
		RedirectURL:  "urn:ietf:wg:oauth:2.0:oob",
	}

	// state is none because in the redirect to "OOB" case, (out of band;
	// meaning "the user has to copy and paste to your app) there's no place
	// to verify it.
	url := cfg.AuthCodeURL("none", oauth2.AccessTypeOffline)

	log.Info("Opening this URL in your browser:")
	log.Info("\t", url)
	browser.OpenURL(url)

	fmt.Print("Enter the access code now: ")

	r := bufio.NewReader(os.Stdin)
	code, err := r.ReadString('\n')
	if err != nil {
		return err
	}
	code = strings.TrimSpace(code)

	token, err := cfg.Exchange(ctx, code)
	if err != nil {
		return err
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return errors.New("no id_token in token response")
	}

	vc := &oidc.Config{ClientID: cfg.ClientID}
	idToken, err := p.Verifier(vc).Verify(ctx, rawIDToken)
	if err != nil {
		return fmt.Errorf("Failed to verify ID token: %v", err)
	}

	var claims struct {
		Email string `json:"email"`
	}
	err = idToken.Claims(&claims)
	if err != nil {
		return fmt.Errorf("could not find the email claim: %v", err)
	}

	// Need to look up the public key for this issuer
	pub, err := getPublic(c, is)
	if err != nil {
		return err
	}

	ocfg := &openidCfg{
		Issuer: is,
		Config: *cfg,
		Token:  *token,
		Data:   claims.Email,
		Public: pub,
	}
	var fn string
	if fn, err = ocfg.save(); err != nil {
		return err
	}
	log.Info("Login information saved into", fn)
	return nil
}

type openidCfg struct {
	Issuer     string
	Config     oauth2.Config
	Token      oauth2.Token
	Data       string
	Public     kyber.Point
	curRefresh string
}

func (o *openidCfg) save() (string, error) {
	// Do not save when not needed.
	if o.Token.RefreshToken == o.curRefresh {
		return "", nil
	}

	os.MkdirAll(bcadminlib.ConfigPath, 0755)
	fn := filepath.Join(bcadminlib.ConfigPath, "openid.cfg")

	// perms = 0600 because there is key material inside this file.
	f, err := os.OpenFile(fn, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return fn, fmt.Errorf("could not write %v: %v", fn, err)
	}

	buf, err := network.Marshal(o)
	if err != nil {
		return fn, err
	}
	_, err = f.Write(buf)
	if err != nil {
		return fn, err
	}

	// Remember the current refresh token, so we can detect the need to save later.
	o.curRefresh = o.Token.RefreshToken

	return fn, f.Close()
}

func load() (*openidCfg, error) {
	os.MkdirAll(bcadminlib.ConfigPath, 0755)
	fn := filepath.Join(bcadminlib.ConfigPath, "openid.cfg")

	buf, err := ioutil.ReadFile(fn)
	if err != nil {
		return nil, err
	}

	_, x, err := network.Unmarshal(buf, cothority.Suite)
	if err != nil {
		return nil, err
	}
	ocfg, ok := (x).(*openidCfg)
	if !ok {
		return nil, errors.New("wrong type")
	}

	// Mark this token as expired, because the round-trip thru save/load has caused us to lose
	// the raw field, which is not exported. So force a refresh on next use.
	ocfg.Token.Expiry = time.Unix(0, 0)

	return ocfg, err
}

func (o *openidCfg) getSigners(cl *eventlog.Client) ([]darc.Signer, error) {
	ts := o.Config.TokenSource(context.Background(), &o.Token)
	r := cl.ByzCoin.Roster
	n := len(r.List)
	T := threshold(n)

	// The callback from darc.Sign where we need to go contact the Authentication Proxies.
	cb := func(msg []byte) ([]byte, error) {
		tok, err := ts.Token()
		if err != nil {
			return nil, err
		}
		// After calling Token(), o.Token might have been updated, using up the old refresh
		// code. Call save, which will detect if the save is really needed or not.
		o.Token = *tok
		o.save()

		rawIDToken, ok := tok.Extra("id_token").(string)
		if !ok {
			return nil, errors.New("no id_token in token response")
		}

		// Make the random shares for the signature.
		rPri := share.NewPriPoly(cothority.Suite, T, nil, cothority.Suite.RandomStream())
		rShares := rPri.Shares(n)
		rPub := rPri.Commit(nil)
		_, rPubCommits := rPub.Info()

		// Make a shuffled list of server id's to contact.
		shuffled := rand.Perm(len(r.List))
		client := onet.NewClient(cothority.Suite, authprox.ServiceName)

		// Connect to servers, get sigs, stop when we achieve the threshold.
		wg := &sync.WaitGroup{}
		wg.Add(len(r.List))
		pChan := make(chan *share.PriShare, len(r.List))
		for _, i := range shuffled {
			go func(idx int) {
				s := r.List[idx]

				rpi := authprox.PriShare{
					I: rShares[idx].I,
					V: rShares[idx].V,
				}
				req := &authprox.SignatureRequest{
					Type:     "oidc",
					Issuer:   o.Issuer,
					AuthInfo: []byte(rawIDToken),
					Message:  msg,
					RandPri:  rpi,
					RandPubs: rPubCommits,
				}
				var resp authprox.SignatureResponse
				err := client.SendProtobuf(s, req, &resp)

				// If no error keep this partial. Otherwise keep going until we have enough.
				if err == nil {
					// Check the sig on the partial sig before trusting it.
					ps := &dss.PartialSig{
						Partial: &share.PriShare{
							I: resp.PartialSignature.Partial.I,
							V: resp.PartialSignature.Partial.V,
						},
						SessionID: resp.PartialSignature.SessionID,
						Signature: resp.PartialSignature.Signature,
					}
					err = schnorr.Verify(cothority.Suite, s.Public, ps.Hash(cothority.Suite), ps.Signature)
					if err == nil {
						pChan <- ps.Partial
					} else {
						log.Warnf("got an incorrectly signed partial signature from %v: %v", s, err)
					}
				} else {
					log.Warnf("could not get a partial signature from %v: %v", s, err)
				}

				wg.Done()
			}(i)
		}

		// TODO: In a perfect world, we'd be using an http.Client
		// with a context, and then once we got enough signatures, we'd
		// kill the outstanding requests. But for now, we will wait until
		// they all timeout, even if we've already got enough.
		wg.Wait()
		close(pChan)

		// TODO: If we "got enough", but in fact we talked to a dishonest signer
		// who sent us an incorrect signature, we won't know it until we reconstruct
		// the sig and then our txn is refused. The brute force method would be to get
		// sigs from all, then remove one sig at a time and retry as long as the
		// reconstructed sig is invalid. A nicer way would be for the signer to
		// send back some proof that it faithfully did the signature.
		var partials []*share.PriShare
		for p := range pChan {
			partials = append(partials, p)
		}
		if len(partials) < T {
			return nil, errors.New("not enough partial signatures")
		}

		gamma, err := share.RecoverSecret(cothority.Suite, partials, T, n)
		if err != nil {
			return nil, err
		}

		// RandomPublic || gamma
		var buff bytes.Buffer
		_, _ = rPub.Commit().MarshalTo(&buff)
		_, _ = gamma.MarshalTo(&buff)
		sig := buff.Bytes()
		return sig, nil
	}

	s := darc.NewSignerProxy(o.Data, o.Public, cb)
	return []darc.Signer{s}, nil
}

func getPublic(c *cli.Context, issuer string) (kyber.Point, error) {
	cl, err := getClient(c, false)
	if err != nil {
		return nil, err
	}

	client := onet.NewClient(cothority.Suite, authprox.ServiceName)

	var resp authprox.EnrollmentsResponse
	err = client.SendProtobuf(cl.ByzCoin.Roster.List[0], &authprox.EnrollmentsRequest{
		Types:   []string{"oidc"},
		Issuers: []string{issuer},
	}, &resp)
	if err != nil {
		return nil, err
	}

	if len(resp.Enrollments) == 0 {
		return nil, errors.New("found no enrollments")
	}
	if len(resp.Enrollments) > 1 {
		return nil, errors.New("found too many enrollments")
	}

	return resp.Enrollments[0].Public, nil
}

func key(c *cli.Context) error {
	if f := c.String("print"); f != "" {
		sig, err := bcadminlib.LoadSigner(f)
		if err != nil {
			return errors.New("couldn't load signer: " + err.Error())
		}
		log.Infof("Private: %s\nPublic: %s", sig.Ed25519.Secret, sig.Ed25519.Point)
		return nil
	}
	newSigner := darc.NewSignerEd25519(nil, nil)
	err := bcadminlib.SaveKey(newSigner)
	if err != nil {
		return err
	}

	var fo io.Writer

	save := c.String("save")
	if save == "" {
		fo = os.Stdout
	} else {
		file, err := os.Create(save)
		if err != nil {
			return err
		}
		fo = file
		defer func() {
			err := file.Close()
			if err != nil {
				log.Error(err)
			}
		}()
	}
	_, err = fmt.Fprintln(fo, newSigner.Identity().String())
	return err
}

func faultThreshold(n int) int {
	return (n - 1) / 3
}

func threshold(n int) int {
	return n - faultThreshold(n)
}
