package authprox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/coreos/go-oidc"
	"go.dedis.ch/onet/v4/log"
)

type oidcValidator struct {
	sync.Mutex
	issuers []*issuer
}

type issuer struct {
	Issuer string
	p      *oidc.Provider
	cfg    oidc.Config
	v      *oidc.IDTokenVerifier
	ctx    context.Context
}

func (o *oidcValidator) FindClaim(issuerStr string, input []byte) (string, string, error) {
	o.Lock()
	defer o.Unlock()

	// find existing, or create a new issuer
	var is *issuer
	for i := range o.issuers {
		if o.issuers[i].Issuer == issuerStr {
			is = o.issuers[i]
		}
	}
	if is == nil {
		is = &issuer{
			Issuer: issuerStr,
			ctx:    context.Background(),
		}
		var err error
		is.p, err = oidc.NewProvider(is.ctx, is.Issuer)
		if err != nil {
			return "", "", err
		}
		is.cfg.SkipClientIDCheck = true
		is.v = is.p.Verifier(&is.cfg)
		o.issuers = append(o.issuers, is)
	}

	// The docs say: Verify does NOT do nonce validation, which is the callers responsibility.
	// This means that the "state" part of the OAuth 2 flow needs to be
	// checked at the time the info arrives inbound to the client, i.e. via token.Exchange()
	// or in a JavaScript OAuth client.
	idToken, err := is.v.Verify(is.ctx, string(input))
	if err != nil {
		return "", "", err
	}

	var claims struct {
		Email string `json:"email"`
		Nonce string `json:"nonce"`
	}
	err = idToken.Claims(&claims)
	if err != nil {
		return "", "", fmt.Errorf("could not find the email claim: %v", err)
	}

	// Code to dump all the claims, for debugging.
	const dumpClaims = false
	if dumpClaims {
		var rclaims json.RawMessage
		idToken.Claims(&rclaims)
		buff := new(bytes.Buffer)
		json.Indent(buff, []byte(rclaims), "", "  ")
		log.Lvl2("claims", string(buff.Bytes()))
	}

	return claims.Email, claims.Nonce, nil
}
