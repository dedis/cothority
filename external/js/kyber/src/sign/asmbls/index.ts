import { BLAKE2Xs } from "@stablelib/blake2xs";
import BN from "bn.js";
import { BN256G2Point } from "../../pairing/point";

export function hashPointToR(pubkeys: BN256G2Point[]): BN[] {
    const peers = pubkeys.map(p => p.marshalBinary())

    const xof = new BLAKE2Xs()
    peers.forEach(p => xof.update(p))

    const out = Buffer.allocUnsafe(16 * peers.length)
    xof.stream(out)

    const coefs = []
    for (let i = 0; i < peers.length; i++) {
        coefs[i] = new BN(out.slice(i * 16, (i + 1) * 16), 'le')
    }

    return coefs
}
