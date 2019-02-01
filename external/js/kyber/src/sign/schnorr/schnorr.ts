import { Group, Scalar, Point } from "../../index"
import { createHash } from "crypto"

/**
 * Sign computes a Schnorr signature over the given message.
 * @param suite         the group to use to sign
 * @param privateKey    the private key
 * @param message       the message that will be signed
 * @returns             the signature as a buffer
 */
export function sign(suite: Group, privateKey: Scalar, message: Buffer): Buffer {
    // generate r & R
    const r = suite.scalar().pick();
    const R = suite.point().mul(r, null);
    const buffR = R.marshalBinary();
    
    // generate public key
    const pub = suite.point().mul(privateKey, null);
    
    // generate challenge
    const challenge = hashSchnorr(suite, buffR, pub.marshalBinary(), message);
    
    // generate signature
    const s = suite.scalar().mul(privateKey, challenge);
    s.add(s, r);
    
    // concatenate R || s
    const buffS = s.marshalBinary();
    const buffSig = Buffer.allocUnsafe(buffR.length + buffS.length);
    buffR.copy(buffSig);
    buffS.copy(buffSig, buffR.length);
    return buffSig;
}

/**
 * Verify verifies if the signature of the message is valid under the given public
 * key.
 * @param suite     the group to use to verify
 * @param publicKey the public key
 * @param message   the message signed
 * @param signature the signature of the message
 * @returns         true when the signature is correct for the given message and public key
 */
export function verify(suite: Group, publicKey: Point, message: Buffer, signature: Buffer): boolean {
    // check the signature size
    const plen = suite.pointLen();
    const slen = suite.scalarLen();
    const totalSize = plen + slen;
    if (signature.length != totalSize) {
        return false;
    }
    
    // unmarshal R || s
    const buffR = signature.slice(0, plen);
    const R = suite.point();
    R.unmarshalBinary(buffR);
    
    const buffs = signature.slice(plen, signature.length);
    const s = suite.scalar();
    s.unmarshalBinary(buffs);
    
    // recompute challenge = H(R || P || M)
    const buffPub = publicKey.marshalBinary();
    const challenge = hashSchnorr(suite, buffR, buffPub, message);
    
    // compute sG
    const left = suite.point().mul(s, null);
    // compute R + challenge * Public
    const right = suite.point().mul(challenge, publicKey);
    right.add(right, R);
    
    if (!right.equals(left)) {
        return false;
    }
    return true;
}

/**
 * hashSchnorr returns a scalar out of hashing the given inputs.
 * @param suite     the group to use to create the scalar
 * @param inputs    the different inputs as buffer
 * @returns the scalar resulting from the hash of the inputs
 */
export function hashSchnorr(suite: Group, ...inputs: Buffer[]): Scalar {
    const h = createHash("sha512");
    for (let i of inputs) {
        h.update(i);
    }
    const scalar = suite.scalar();
    scalar.setBytes(h.digest());
    return scalar;
}