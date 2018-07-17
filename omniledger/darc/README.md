// Package darc implements Distributed Access Right Controls.
//
// In most of our projects we need some kind of access control to protect
// resources. Instead of having a simple password or public key for
// authentication, we want to have access control that can be: evolved with a
// threshold number of keys be delegated. So instead of having a fixed list of
// identities that are allowed to access a resource, the goal is to have an
// evolving description of who is allowed or not to access a certain resource.
//
// The primary type is a Darc, which contains a set of rules that determine
// what type of permission are granted for any identity. A Darc can be updated
// by performing an evolution.  That is, the identities that have the "evolve"
// permission in the old Darc can create a signature that signs off the new
// Darc. Evolutions can be performed any number of times, which creates a chain
// of Darcs, also known as a path. A path can be verified by starting at the
// oldest Darc (also known as the base Darc), walking down the path and
// verifying the signature at every step.
//
// As mentioned before, it is possible to perform delegation. For example,
// instead of giving the "evolve" permission to (public key) identities, we can
// give it to other Darcs. For example, suppose the newest Darc in some path,
// let's called it darc_A, has the "evolve" permission set to true for another
// darc: darc_B. Then darc_B is allowed to evolve the path.
//
// Of course, we do not want to have static rules that allow only one signer.
// Our Darc implementation supports an expression language where the user can
// use logical operators to specify the rule.  For exmple, the expression
// "darc:a & ed25519:b | ed25519:c" means that "darc:a" and at least one of
// "ed25519:b" and "ed25519:c" must sign. For more information please see the
// expression package.
