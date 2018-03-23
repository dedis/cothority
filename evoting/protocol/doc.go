/*
Package protocol implements the three principal protocols of the evoting service.

DKG: Distributed key generation algorithm to setup a shared secret amongst the
responsible conodes for each newly opened election.

Shuffle: The participating conodes each create a Neff shuffle with
corresponding proof of the encrypted ballots and store the result back on the
election skipchain.

Decrypt: Each conode checks the integrity of the mixes and partially decrypts
the last mix with its part of the shared secret. The result is then appended to
the election skipchain.
*/
package protocol
