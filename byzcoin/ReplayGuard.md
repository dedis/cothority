Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](https://github.com/dedis/cothority/tree/master/README.md) ::
[Building Blocks](https://github.com/dedis/cothority/tree/master/doc/BuildingBlocks.md) ::
[ByzCoin](README.md) ::
Replay Guard

# Replay Guard
In order to prevent replay attacks, we introduce a counting mechanism. For
every instruction that the client submits, it must include a counter for every
identity that signs the instruction. If the most recent instruction is signed
by the signer at count n, then the next instruction that the same signer signs
must be on counter n+1. If the signer never signed an instruction before, then
the counter starts at one.

For example:
```
// suppose the counter state is at:
// signer1: 0, signer2: 1, signer3: 2
// to create a new transaction, the counters must be set like so:
ClientTransaction {
	Instruction {
		// payload
		SignerCounters: [1, 2, 3]
		Signers: ["signer1", "signer2", "signer3"]
	}
	Instruction {
		// payload
		SignerCounters: [2, 3]
		Signers: ["signer1", "signer2"]
	}
	Instruction {
		// payload
		SignerCounters: [3]
		Signers: ["signer1"]
	}
}
// the counter state on ByzCoin at the end will be:
// signer1: 3, signer2: 3, signer3: 3
```

We assume that the private keys are not shared between clients. So the clients
can easily predict what their counters will be without querying ByzCoin all the
time for the latest value of their counter. But if a client forgets its
counter, it can use the `GetSignerCounters` API to get the counters.
