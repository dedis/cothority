import { BN256G1Point, BN256G2Point } from '../../pairing/point';
import BN256Scalar from '../../pairing/scalar';

export type BlsSignature = Buffer;

/**
 * Sign the message with the given secret key
 * @param msg the message to sign
 * @param secret the private key
 */
export function sign(msg: Buffer, secret: BN256Scalar): BlsSignature {
    const HM = BN256G1Point.hashToPoint(msg);
    HM.mul(secret, HM);

    return HM.marshalBinary();
}

/**
 * Verify the signature of the message with the public key
 * @param msg the message
 * @param pub the public key as a point
 * @param sig the signature as a buffer
 */
export function verify(msg: Buffer, pub: BN256G2Point, sig: Buffer): boolean {
    const HM = BN256G1Point.hashToPoint(msg);
    const left = HM.pair(pub);

    const s = new BN256G1Point();
    s.unmarshalBinary(sig);
    const right = s.pair(new BN256G2Point().base());

    return left.equals(right);
}
