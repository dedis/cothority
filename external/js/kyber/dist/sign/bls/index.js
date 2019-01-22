"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const crypto_1 = require("crypto");
const point_1 = require("../../pairing/point");
function hashToPoint(msg) {
    const h = crypto_1.createHash('sha256');
    h.update(msg);
    const p = new point_1.BN256G1Point(h.digest());
    return p;
}
/**
 * Sign the message with the given secret key
 * @param msg the message to sign
 * @param secret the private key
 */
function sign(msg, secret) {
    const HM = hashToPoint(msg);
    HM.mul(secret, HM);
    return HM.marshalBinary();
}
exports.sign = sign;
/**
 * Verify the signature of the message with the public key
 * @param msg the message
 * @param pub the public key as a point
 * @param sig the signature as a buffer
 */
function verify(msg, pub, sig) {
    const HM = hashToPoint(msg);
    const left = HM.pair(pub);
    const s = new point_1.BN256G1Point();
    s.unmarshalBinary(sig);
    const right = s.pair(new point_1.BN256G2Point().base());
    return left.equals(right);
}
exports.verify = verify;
