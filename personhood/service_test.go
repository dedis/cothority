package personhood

import (
	"bytes"
	"fmt"
	"github.com/stretchr/testify/require"
	"go.dedis.ch/cothority/v3/byzcoin"
	contracts2 "go.dedis.ch/cothority/v3/byzcoin/contracts"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/cothority/v3/darc/expression"
	"go.dedis.ch/cothority/v3/personhood/contracts"
	"go.dedis.ch/cothority/v3/personhood/user"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/protobuf"
	"regexp"
	"strings"
	"testing"
	"time"
)

var genesisRules = []string{
	"spawn:" + contracts.ContractCredentialID,
	"spawn:" + contracts.ContractSpawnerID,
	"spawn:" + contracts2.ContractCoinID,
	"invoke:" + contracts2.ContractCoinID + ".mint",
	"invoke:" + contracts2.ContractCoinID + ".transfer",
}

func TestMain(m *testing.M) {
	log.SetShowTime(true)
	log.MainTest(m)
}

var baseURL = "https://something.com"

type testService struct {
	*byzcoin.BCTest
	*testing.T
	s             *Service
	genesisCoinID byzcoin.InstanceID
	emailDarc     *darc.Darc
	baseUser      *user.User
	dummyEmail    bytes.Buffer
}

func newTestService(t *testing.T) *testService {
	ts := &testService{
		BCTest: byzcoin.NewBCTestDefault(t),
		T:      t,
	}
	ts.AddGenesisRules(genesisRules...)
	ts.CreateByzCoin()
	ts.s = ts.Servers[0].Service(ServiceName).(*Service)

	userName := "testUser"
	var err error
	ts.baseUser, err = user.NewFromByzcoin(ts.Client,
		ts.GenesisDarc.GetBaseID(),
		ts.Signer, userName)
	require.NoError(t, err)

	id := darc.NewIdentityDarc(ts.baseUser.SignerDarc.GetBaseID()).String()
	rules := darc.NewRules()
	require.NoError(t, rules.AddRule("_sign", expression.Expr(id)))
	require.NoError(t, rules.AddRule(byzcoin.ContractDarcInvokeEvolve, expression.Expr(id)))
	ts.emailDarc = darc.NewDarc(rules, []byte("Email Darc"))
	as := ts.baseUser.GetActiveSpawner()
	require.NoError(t, as.SpawnDarc(*ts.emailDarc))
	require.NoError(t, as.SendTransaction())

	return ts
}

func (ts *testService) callSetup() *EmailSetupReply {
	deviceURL, err := ts.baseUser.AddDevice(baseURL, "email")
	require.NoError(ts, err)

	emSetup := &EmailSetup{
		ByzCoinID:   ts.Genesis.SkipChainID(),
		DeviceURL:   deviceURL,
		EmailDarcID: byzcoin.NewInstanceID(ts.emailDarc.GetBaseID()),
		SMTPHost:    "localhost:25",
		SMTPFrom:    "root@localhost",
		SMTPReplyTo: "root@something.com",
		BaseURL:     baseURL,
	}

	require.NoError(ts, emSetup.Sign(ts.Local.GetPrivate(ts.Servers[0])))
	setupReply, err := ts.s.EmailSetup(emSetup)
	require.NoError(ts, err)
	ts.s.storage.EmailConfig.dummy = &ts.dummyEmail

	require.NoError(ts, ts.Client.WaitPropagation(-1))

	return setupReply
}

func (ts *testService) updateEmailDarc() {
	require.NoError(ts, ts.Client.WaitPropagation(-1))
	edBuf, err := ts.Client.GetInstance(byzcoin.NewInstanceID(ts.emailDarc.
		GetBaseID()), byzcoin.ContractDarcID)
	require.NoError(ts, err)
	require.NoError(ts, protobuf.Decode(edBuf, ts.emailDarc))
}

func TestService_EmailSetup(t *testing.T) {
	ts := newTestService(t)
	defer ts.CloseAll()

	deviceURL, err := ts.baseUser.AddDevice(baseURL, "email")
	require.NoError(t, err)

	emSetup := &EmailSetup{
		ByzCoinID:   ts.Genesis.SkipChainID(),
		DeviceURL:   deviceURL,
		EmailDarcID: byzcoin.NewInstanceID(ts.emailDarc.GetBaseID()),
	}
	require.NoError(t, emSetup.Sign(ts.Local.GetPrivate(ts.Servers[0])))
	setupReply, err := ts.s.EmailSetup(emSetup)
	require.NoError(t, err)

	log.Lvlf2("User is: %+v / reply is: %+v", ts.baseUser, setupReply)
}

func TestService_EmailSignup(t *testing.T) {
	ts := newTestService(t)
	defer ts.CloseAll()
	ts.callSetup()

	signupReply, err := ts.s.EmailSignup(&EmailSignup{
		Email: "linus.gasser@epfl.ch",
		Alias: "ineiti25",
	})
	require.NoError(t, err)
	require.Equal(t, ESECreated, signupReply.Status)

	msg := ts.dummyEmail.String()
	require.True(t, strings.Contains(msg, baseURL))

	ts.updateEmailDarc()
	signRules := strings.Split(string(ts.emailDarc.Rules.GetSignExpr()), "|")
	require.Equal(t, 2, len(signRules))

	signupReply, err = ts.s.EmailSignup(&EmailSignup{
		Email: "linus.gasser@epfl.ch",
		Alias: "ineiti25",
	})
	require.NoError(t, err)
	require.Equal(t, signupReply.Status, ESEExists)

	ts.updateEmailDarc()
	signRules = strings.Split(string(ts.emailDarc.Rules.GetSignExpr()), "|")
	require.Equal(t, 2, len(signRules))
}

func TestService_EmailRecover(t *testing.T) {
	ts := newTestService(t)
	defer ts.CloseAll()
	ts.callSetup()

	signupReply, err := ts.s.EmailSignup(&EmailSignup{
		Email: "linus.gasser@epfl.ch",
		Alias: "ineiti25",
	})
	require.NoError(t, err)
	require.Equal(t, ESECreated, signupReply.Status)

	msg := ts.dummyEmail.String()
	require.True(t, strings.Contains(msg, baseURL))
	urlRegexp := regexp.MustCompile(fmt.Sprintf("%s?[^\r]*", baseURL))
	newUserURL := string(urlRegexp.Find([]byte(msg)))

	newUserSignup, err := user.NewFromURL(ts.Client, newUserURL)
	require.NoError(t, err)
	require.NotEqual(t, ts.baseUser.CredIID, newUserSignup.CredIID)

	reply, err := ts.s.EmailRecover(&EmailRecover{
		Email: "linus.gasser2@epfl.ch",
	})
	require.Error(t, err)
	require.Equal(t, EREUnknown, reply.Status)

	ts.dummyEmail.Reset()
	reply, err = ts.s.EmailRecover(&EmailRecover{
		Email: "linus.gasser@epfl.ch",
	})
	// Need to wait for the update to go through the network.
	require.NoError(t, err)
	require.Equal(t, ERERecovered, reply.Status)
	time.Sleep(time.Second)

	msg = ts.dummyEmail.String()
	recoverURL := string(urlRegexp.Find([]byte(msg)))
	recoverUser, err := user.NewFromURL(ts.Client, recoverURL)
	require.NoError(t, err)
	require.Equal(t, newUserSignup.CredIID, recoverUser.CredIID)
}

func TestEmailConfig_SendMail(t *testing.T) {
	t.Skip("This only works within the EPFL network")
	ec := emailConfig{
		SMTPConfig:  "mail.epfl.ch:25",
		SMTPFrom:    "noreply@epfl.ch",
		SMTPReplyTo: "c4dt-services@listes.epfl.ch",
	}
	require.NoError(t, ec.SendMail("linus.gasser@epfl.ch", "Test",
		fmt.Sprintf("This is a test message at %s",
			time.Now().Format(time.RFC3339))))
}

func TestEmailConfig_SendMailDummy(t *testing.T) {
	var dummy bytes.Buffer
	ec := emailConfig{dummy: &dummy}
	to := "linus.gasser@epfl.ch"
	subject := "Test"
	body := "This is a test message"
	require.NoError(t, ec.SendMail(to, subject, body))
	msg := dummy.String()
	require.True(t, strings.Contains(msg, to))
	require.True(t, strings.Contains(msg, subject))
	require.True(t, strings.Contains(msg, body))
}
