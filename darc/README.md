Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](https://github.com/dedis/cothority/tree/main/README.md) ::
[Building Blocks](https://github.com/dedis/cothority/tree/main/doc/BuildingBlocks.md) ::
[ByzCoin](../README.md)
Distributed Access Right Controls

# Distributed Access Right Controls

In most of our projects we need some kind of access control to protect
resources. Instead of having a simple password or public key for
authentication, we want to have access control that can be: evolved with a
threshold number of keys be delegated. So instead of having a fixed list of
identities that are allowed to access a resource, the goal is to have an
evolving description of who is allowed or not to access a certain resource.

The primary type is a Darc, which contains a set of rules that determine
what type of permission are granted for any identity. A Darc can be updated
by performing an evolution.  That is, the identities that have the `evolve`
permission in the old Darc can create a signature that signs off the new
Darc. Evolutions can be performed any number of times, which creates a chain
of Darcs, also known as a path. A path can be verified by starting at the
oldest Darc (also known as the base Darc), walking down the path and
verifying the signature at every step.

As mentioned before, it is possible to perform delegation. For example,
instead of giving the `evolve` permission to (public key) identities, we can
give it to other Darcs. For example, suppose the newest Darc in some path,
let's called it darc_A, has the `evolve` permission set to true for another
darc: darc_B. Then darc_B is allowed to evolve the path.

Of course, we do not want to have static rules that allow only one signer.
Our Darc implementation supports an expression language where the user can
use logical operators to specify the rule.  For example, the expression
`darc:a & ed25519:b | ed25519:c` means that `darc:a` and at least one of
`ed25519:b` and `ed25519:c` must sign.

## Delegation

In the case of the `darc:` expression, one darc delegates the permissions to
a second darc. To verify the signature, the expression will be validated
if the `sign` rule of the second darc is validated.

Here is an example of two darcs, where the second darc is allowed to evolve
the first darc:

```
Darc a
Rule: "evolve": "darc:b"
```
```
Darc b
Rule: "sign": "ed25519:deadbeef"
```

Now if a request to evolve Darc_a comes in, it is enough to have this request
signed by the private key corresponding to the public `deadbeef`.

## Expressions

Package expression contains the definition and implementation of a simple
language for defining complex policies. We define the language in extended-BNF notation,
the syntax we use is from: https://en.wikipedia.org/wiki/Extended_Backus%E2%80%93Naur_form

```
  expr = term, [ '&', term ]*
  term = factor, [ '|', factor ]*
  factor = '(', expr, ')' | id
  id = [0-9a-z]+, ':', [0-9a-f]+
```

Examples:
```
  ed25519:deadbeef // every id evaluates to a boolean
```
```
  (a:a & b:b) | (c:c & d:d)
```

In the simplest case, the evaluation of an expression is performed against a
set of valid ids.  Suppose we have the expression (a:a & b:b) | (c:c & d:d),
and the set of valid ids is [a:a, b:b], then the expression will evaluate to
true.  If the set of valid ids is [a:a, c:c], then the expression will evaluate
to false. However, the user is able to provide a ValueCheckFn to customise how
the expressions are evaluated.

### EXTENSION - NOT YET IMPLEMENTED:
To support threshold signatures, we extend the syntax to include the following.
```
  thexpr = '[', id, [ ',', id ]*, ']', '/', digit
```
