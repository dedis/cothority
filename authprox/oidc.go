package authprox

import (
	"context"
	"fmt"
	"sync"

	"github.com/coreos/go-oidc"
)

type oidcValidator struct {
	sync.Mutex
	Issuer string
	p      *oidc.Provider
	cfg    oidc.Config
	v      *oidc.IDTokenVerifier
	ctx    context.Context
}

func newOidc(issuer string) Validator {
	return &oidcValidator{
		Issuer: issuer,
		ctx:    context.Background(),
	}
}

func (o *oidcValidator) FindClaim(input []byte) (claim, extraData string, err error) {
	o.Lock()
	defer o.Unlock()

	if o.p == nil {
		o.p, err = oidc.NewProvider(o.ctx, o.Issuer)
		if err != nil {
			return
		}
		o.cfg.SkipClientIDCheck = true
		o.v = o.p.Verifier(&o.cfg)
	}

	idToken, err := o.v.Verify(o.ctx, string(input))
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
