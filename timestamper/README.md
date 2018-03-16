Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../README.md) ::
[Applications](../doc/Applications.md) ::
Timestamper

# Timestamper

*WARNING* - this thing doesn't exist at all - this is just some documentation
that looked nice to be kept around...

This service offers a collective signature at regular intervals (epochs) of a
hash the client provides. The collective signature is done on the
merkle-tree-root of all hashes sent in one epoch concatenated with the time of
the signature.

## NSDI-version
The following calls should be implemented in the service:
See https://github.com/dedis/cothority/issues/554#issuecomment-243092585

# API calls

## SetupStamper
Update: This will be only locally (not a message which is sent around). The description below is for what might be implemented later (after NSDI).

Destination: first conode in the ‘roster to be used’
* Input:
  * roster to be used
  * epoch-length
* Action:
  * pings all elements of the ‘roster to be used’ to make sure they are alive
* Saves:
  * ID of stamper and corresponding ‘roster to be used’
* Returns:
  * ID of stamper
  * Collective public key
  * error if a threshold of conodes in ‘roster to be used’ are not responding

## SignHash
* Destination: first conode in the ‘roster to be used’
* Input:
  * ID of stamper
  * hash to be signed
* Action:
  * Collects all hashes during one epoch
  * When the epoch is over
    * creates a merkle-tree of all hashes
    * Asks the roster belonging to ID to CoSi the merkle-tree-root concatenated with the time (seconds since start of Unix-epoch)
* Saves:
  * nothing
* Returns:
  * CoSi on merkle-tree-root concatenated with time
  * merkle-tree-root and inclusion-proof of ‘hash to be signed’
  * time

## VerifyHash
* Destination: none - verifies locally only
* Input:
  * structure from SignHash
* Action:
  * checks the inclusion-proof
  * verifies the signature
* Returns:
  * OK if the check and the verification pass, an error otherwise

# Improvements

These are improvements that can be done once the basic service is working. This list also defines what does not need to be included in the first version:

* all nodes of the roster verify the time
* if the time is off by more than a threshold, they should refuse to sign
* the root-node will simply restart a round with all nodes who accepted to sign and update the mask of the cosi-signature
all nodes accept hashes to be signed
* every node needs to do his only merkle tree at the end of an epoch
* every individual merkle-tree-root needs to be sent up to the root
