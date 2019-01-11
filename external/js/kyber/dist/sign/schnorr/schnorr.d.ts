/// <reference types="node" />
import { Group, Scalar, Point } from "../../index";
/**
 * Sign computes a Schnorr signature over the given message.
 * @param suite         the group to use to sign
 * @param privateKey    the private key
 * @param message       the message that will be signed
 * @returns             the signature as a buffer
 */
export declare function sign(suite: Group, privateKey: Scalar, message: Buffer): Buffer;
/**
 * Verify verifies if the signature of the message is valid under the given public
 * key.
 * @param suite     the group to use to verify
 * @param publicKey the public key
 * @param message   the message signed
 * @param signature the signature of the message
 * @returns         true when the signature is correct for the given message and public key
 */
export declare function verify(suite: Group, publicKey: Point, message: Buffer, signature: Buffer): boolean;
/**
 * hashSchnorr returns a scalar out of hashing the given inputs.
 * @param suite     the group to use to create the scalar
 * @param inputs    the different inputs as buffer
 * @returns the scalar resulting from the hash of the inputs
 */
export declare function hashSchnorr(suite: Group, ...inputs: Buffer[]): Scalar;
