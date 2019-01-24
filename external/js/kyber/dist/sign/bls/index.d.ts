/// <reference types="node" />
import BN256Scalar from '../../pairing/scalar';
import { BN256G2Point } from '../../pairing/point';
export declare type BlsSignature = Buffer;
/**
 * Sign the message with the given secret key
 * @param msg the message to sign
 * @param secret the private key
 */
export declare function sign(msg: Buffer, secret: BN256Scalar): BlsSignature;
/**
 * Verify the signature of the message with the public key
 * @param msg the message
 * @param pub the public key as a point
 * @param sig the signature as a buffer
 */
export declare function verify(msg: Buffer, pub: BN256G2Point, sig: Buffer): boolean;
