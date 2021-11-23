package personhood

import (
	"crypto/sha256"
	"crypto/tls"
	"fmt"
	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/sign/schnorr"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"golang.org/x/xerrors"
	"io"
	"net"
	"net/smtp"
)

// Sign the request with the given private key.
func (es *EmailSetup) Sign(priv kyber.Scalar) error {
	sum := es.Hash()
	sig, err := schnorr.Sign(cothority.Suite, priv, sum)
	if err != nil {
		return xerrors.Errorf("couldn't sign: %v", err)
	}
	es.Signature = sig
	return nil
}

// Verify the request with the given public key.
func (es EmailSetup) Verify(pub kyber.Point) error {
	sum := es.Hash()
	return schnorr.Verify(cothority.Suite, pub, sum,
		es.Signature)
}

// Hash returns the hash of the request.
func (es EmailSetup) Hash() []byte {
	h := sha256.New()
	h.Write([]byte(es.DeviceURL))
	h.Write(es.EmailDarcID[:])
	h.Write([]byte(es.SMTPHost))
	h.Write([]byte(es.SMTPFrom))
	h.Write([]byte(es.SMTPReplyTo))
	return h.Sum(nil)
}

type emailConfig struct {
	ByzCoinID   skipchain.SkipBlockID
	Roster      onet.Roster
	BaseURL     string
	UserID      byzcoin.InstanceID
	UserSigner  darc.Signer
	EmailDarcID byzcoin.InstanceID
	SMTPFrom    string
	SMTPReplyTo string
	SMTPConfig  string
	// How many emails per day are to be sent maximum
	EmailsLimit int64
	// How many emails over the last 24h
	Emails int64
	// When the last email has been sent, as unix timestamp
	EmailsLast int64
	dummy      io.Writer
}

// SendMail sends an email to the given address. As the EPFL SMTP server
// supports sending without authentication, the steps need to be done
// manually.
func (ec emailConfig) SendMail(to, cc, subject, body string) error {
	if ec.dummy != nil {
		_, err := fmt.Fprintf(ec.dummy, "Sending email to %s: %s\n%s\n",
			to, subject, body)
		return err
	}
	if ec.SMTPConfig == "dummy:25" {
		log.Infof("Sending email to %s: %s\n%s\n", to, subject, body)
		return nil
	}

	client, err := smtp.Dial(ec.SMTPConfig)
	if err != nil {
		return xerrors.Errorf("failed to dial smtp server: %v", err)
	}
	host, _, err := net.SplitHostPort(ec.SMTPConfig)
	if err != nil {
		return xerrors.Errorf("failed to split host port: %v", err)
	}
	config := &tls.Config{ServerName: host}
	if err := client.StartTLS(config); err != nil {
		return xerrors.Errorf("failed to start tls: %v", err)
	}
	if err := client.Mail(ec.SMTPFrom); err != nil {
		return xerrors.Errorf("failed to set from: %v", err)
	}
	if err := client.Rcpt(to); err != nil {
		return xerrors.Errorf("failed to set to: %v", err)
	}
	if err := client.Rcpt(cc); err != nil {
		return xerrors.Errorf("failed to set cc: %v", err)
	}
	data, err := client.Data()
	if err != nil {
		return xerrors.Errorf("failed to send data: %v", err)
	}
	msg := []byte(fmt.Sprintf("To: %s\r\n"+
		"Reply-to: %s\r\n"+
		"CC: %s\r\n"+
		"From: %s\r\n"+
		"Subject: %s\r\n\r\n%s\r\n",
		to, ec.SMTPReplyTo, cc, ec.SMTPReplyTo, subject, body))
	if _, err := data.Write(msg); err != nil {
		return xerrors.Errorf("failed to write data: %v", err)
	}
	if err := data.Close(); err != nil {
		return xerrors.Errorf("failed to close data: %v", err)
	}
	if err := client.Quit(); err != nil {
		return xerrors.Errorf("failed to quit: %v", err)
	}

	return nil
}

func (ec *emailConfig) tooManyEmails(clockHours int64) bool {
	if ec.EmailsLimit == 0 {
		return false
	}
	if clockHours-ec.EmailsLast > 0 {
		newMails := clockHours - ec.EmailsLast
		if newMails > ec.Emails {
			ec.Emails = 0
		} else {
			ec.Emails -= newMails
		}
		ec.EmailsLast = clockHours
	}
	if ec.Emails >= ec.EmailsLimit {
		return true
	}

	ec.Emails++
	return false
}

func (ec *emailConfig) getClient() *byzcoin.Client {
	cl := byzcoin.NewClient(ec.ByzCoinID, ec.Roster)
	cl.UpdateNodes()
	ec.Roster = cl.Roster

	return cl
}
