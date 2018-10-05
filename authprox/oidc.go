package authprox

import (
	"context"
	"fmt"
	"sync"

	"github.com/coreos/go-oidc"
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

func (o *oidcValidator) FindClaim(issuerStr string, input []byte) (claim, extraData string, err error) {
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
		is.p, err = oidc.NewProvider(is.ctx, is.Issuer)
		if err != nil {
			return
		}
		is.cfg.SkipClientIDCheck = true
		is.v = is.p.Verifier(&is.cfg)
		o.issuers = append(o.issuers, is)
	}

	idToken, err := is.v.Verify(is.ctx, string(input))
	if err != nil {
		return
	}

	var claims struct {
		Email string `json:"email"`
	}
	err = idToken.Claims(&claims)
	if err != nil {
		err = fmt.Errorf("could not find the email claim: %v", err)
		return
	}

	return claims.Email, "", nil
}
