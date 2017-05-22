# Discussion with Bryan on 19th of May 2017

## Write-transaction

The key for the file is given like:

g**r, Key * Y

The whole write-log needs to be signed by the writer, proofing that he knows
r.

## Read-transaction

Signed by the reader.

Data-part includes 
- hash of the write-transaction (skipblock-id)
- encrypted key

-> Request to re-encrypt to "C"

? collective signature is only in the forward-link of the previous block(s)

## SHU - Secret Holding Unit

Have to know which cothorities are allowed to sign off.

### PVSS

Can be done in a way that each node can re-encrypt the share to the
public-key of the Reader.

Each node does it's H**a_j and G**(z_i*x_i)
	- ElGamal-encrypts it to C 

-> time-vaults?
-> Shamir re-sharing?

### VSS

Each month a new share-holding group is created and all keys are re-encrypted
to the new share-holding group.

x: collective private key
X = g**x: collective public key

