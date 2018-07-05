package darc

// ID is the identity of a Darc - which is the sha256 of its protobuf representation
// over invariant fields [Owners, Users, Version, Description]. Signature is excluded.
// An evolving Darc will change its identity.
type ID []byte

// Role indicates if this is an Owner or a User Policy.
type Role int

const (
	// Owner has the right to evolve the Darc to a new version.
	Owner Role = iota
	// User has the right to sign on behalf of the Darc for
	// whatever decision is asked for.
	User
)

// Copy creates a deep copy of the IdentityDarc.
func (id *Identity) Copy() *Identity {
	c := &Identity{}
	if id.Darc != nil {
		c.Darc = &IdentityDarc{make([]byte, len(id.Darc.ID))}
		copy(c.Darc.ID, id.Darc.ID)
	} else if id.Ed25519 != nil {
		c.Ed25519 = &IdentityEd25519{id.Ed25519.Point.Clone()}
	} else if id.X509EC != nil {
		c.X509EC = &IdentityX509EC{make([]byte, len(id.X509EC.Public))}
		copy(c.X509EC.Public, id.X509EC.Public)
	}
	return c
}
