import { BLAKE2Xs } from "@stablelib/blake2xs";
import BN from "bn.js";
import { Point } from "../..";
import { BN256G1Point, BN256G2Point } from "../../pairing/point";
import BN256Scalar from "../../pairing/scalar";
import * as BLS from "../bls";
import Mask from "../mask";

const COEF_SIZE = 128 / 8;

export type BdnSignature = Buffer;

/**
 * Generate the list of coefficients for the list of public keys
 * 
 * @param pubkeys The list of public keys
 * @return The list of coefficients as BigNumber
 */
export function hashPointToR(pubkeys: Point[]): BN[] {
    const peers = pubkeys.map(p => p.marshalBinary())

    const xof = new BLAKE2Xs()
    peers.forEach(p => xof.update(p))

    const out = Buffer.allocUnsafe(COEF_SIZE * peers.length)
    xof.stream(out)

    const coefs = []
    for (let i = 0; i < peers.length; i++) {
        coefs[i] = new BN(out.slice(i * COEF_SIZE, (i + 1) * COEF_SIZE), 'le')
    }

    return coefs
}

/**
 * Aggregate the list of points according to the mask participation.
 * The length of the mask and the list of points must match. Intermediate
 * non-participating points can be null.
 * 
 * @param mask      The mask with the participation
 * @param points    The list of points to aggregate
 */
function aggregatePoints(mask: Mask, points: Point[]) {
    if (mask.getCountTotal() !== points.length) {
        throw new Error("Length of mask and points does not match")
    }

    const coefs = hashPointToR(mask.publics);

    let agg: Point = null;
    for (let i = 0; i < coefs.length; i++) {
        if (mask.isIndexEnabled(i)) {
            const c = new BN256Scalar(coefs[i]);
            const p = points[i].clone();

            p.mul(c, p);
            // R is in the range [1; 2^128] inclusive thus (c + 1) * p
            p.add(p, points[i]);
            if (agg == null) {
                agg = p;
            } else {
                agg.add(agg, p);
            }
        }
    }

    return agg;
}

/**
 * Aggregate the public keys of the mask
 * 
 * @param mask The mask with the participation and the list of points
 * @return The new point representing the aggregation
 */
export function aggregatePublicKeys(mask: Mask): Point {
    return aggregatePoints(mask, mask.publics);
}

/**
 * Aggregate a list of signatures according to the given mask
 * 
 * @param mask The mask with the participation
 * @param sigs The signatures as bytes
 * @return The new point representing the aggregation
 */
export function aggregateSignatures(mask: Mask, sigs: Buffer[]) {
    const points = sigs.map((s) => {
        if (!s) {
            return null;
        }

        const p = new BN256G1Point();
        p.unmarshalBinary(s);
        return p;
    });

    return aggregatePoints(mask, points);
}

/**
 * Sign the message using the given secret
 * 
 * @param msg       The Message to sign
 * @param secret    The secret key
 * @return The BDN signature
 */
export function sign(msg: Buffer, secret: BN256Scalar): BdnSignature {
    return BLS.sign(msg, secret);
}

/**
 * Verify the given signature against the message using the mask
 * to aggregate the public keys
 * 
 * @param msg   The message
 * @param mask  The mask with the public keys
 * @param sig   The signature as bytes
 * @return true if the signature matches, false otherwise
 */
export function verify(msg: Buffer, mask: Mask, sig: Buffer): boolean {
    const pub = aggregatePublicKeys(mask) as BN256G2Point;

    return BLS.verify(msg, pub, sig);
}
