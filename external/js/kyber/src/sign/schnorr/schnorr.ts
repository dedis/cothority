import { Group, Scalar, Point } from "../../index"
import { createHash } from "crypto"

/*
*
* Sign computes a Schnorr signature over the given message.
* */
export function sign(suite: Group, privateKey: Scalar, message: Buffer) {
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
*
* Verify verifies if the signature of the message is valid under the given public
* key.
* */
export function verify(suite: Group, publicKey: Point, message: Buffer, signature: Buffer) {
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
    
    if (!right.equal(left)) {
        return false;
    }
    return true;
}

/**
*
* hashSchnorr returns a scalar out of hashing the given inputs.
**/
export function hashSchnorr(suite: Group, ...inputs: Buffer[]) {
    const h = createHash("sha512");
    for (let i of inputs) {
        h.update(i);
    }
    const scalar = suite.scalar();
    scalar.setBytes(h.digest());
    return scalar;
}