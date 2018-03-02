# Authentication

The authentication service in the cothority allows for handling access to different
part of a conode using Distributed Access Rights Control policies (Darc-policies).
This allows to not only have a simple public/private key authentication, but to
delegate authentication to other schemes like PoP-tokens or group management
implementations where the members of the group handle their own keys and update
them independent of the main database.

When the conode is initialized, a first Darc-policy is setup that restricts the
services implementing it to be accessed only by the holder of the private key
of the conode. This basic policy can be updated either by signing a new Darc
using the private key of the conode, or by proofing access to the conode's server
by transmitting a correct PIN number.

The app in this directory helps setting up and managing the authentication to
one or more conodes.
