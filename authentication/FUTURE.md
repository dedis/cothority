It is based on the policy library from Sandra Siby which is based on the
darc-library from Linus.

# Authentication with policy-darcs

The authentication service keeps track of access policies with the policy
darcs. Upon startup, there is one policy darc with the following setup:
```
  Policy: "admin"
  User: "public_key_of_conode"
  Policy: "*"
  User: "public_key_of_conode"
```
This means that by using the private key of the conode, the policy can
be evolved. Also by using the private key of the conode, the policy can
authenticate against all services.

The policies should at least support the following methods of authentication:
- ed25519 public key authentication
- X509 public key authentication
- Darc authentication
Future implementations might also do the following:
- PoP authentication
- CISC authentication - having a darc on a skipchain and following its
  latest version thereon

# ServiceProcessor for services to follow authentication

For authentication, it offers a new ServiceProcessor that can be embedded
in the service instead of onet.ServiceProcessor. When registering a
handler, the handler gets a new argument of type Authentication:
```
  type Authentication struct{
    Signature *darc.Signature
    IP net.Address
  }
```
The Signature field can be nil in case there hasn't been a policy set up
for the service. This can happen if the admin decides to change the standard
policy and removes the `Policy:"*"`. Then everybody is allowed to connect
to all services that use authentication.

# API for the clients

The API of the services wanting to authenticate need to add a new field in
their messages that is of type `policy.Signature` and is the first field
available. In the protobuf definition this field is optional, as under some
circumstances it might be nil.

On the API side, the following methods are available:
```
  // GetPolicy returns the latest version of the chosen policy with the ID
  // given. If the ID == null, the basic policy is returned.
  GetPolicy(ID darc.ID)(*darc.Policy, error)

  // UpdatePolicy updates an existing policy. Following the policy-library,
  // it needs to be signed by one of the admin users.
  UpdatePolicy(newPolicy darc.Policy) error

  // UpdatePolicyPIN can be used in case the private key is not available,
  // but if the user has access to the logs of the server. On the first
  // call the PIN == "", and the server will print a 6-digit PIN in the log
  // files. When he receives the policy and the correct PIN, the server will
  // auto-sign the policy using his private key and add it to the policy-list.
  UpdatePolicyPIN(newPolicy policy, PIN string) error

  // AddPolicy can be used to add a new policy to the system that will be
  // referenced later with UpdatePolicy. For a new policy, it must be signed
  // by a user of the root-policy.
  AddPolicy(newPolicy policy, signature Signature)
```
