/// <reference types="node" />
import { Group, Scalar, Point } from "../../index";
export declare function sign(suite: Group, privateKey: Scalar, message: Buffer): Buffer;
/**
*
* Verify verifies if the signature of the message is valid under the given public
* key.
* */
export declare function verify(suite: Group, publicKey: Point, message: Buffer, signature: Buffer): boolean;
/**
*
* hashSchnorr returns a scalar out of hashing the given inputs.
**/
export declare function hashSchnorr(suite: Group, ...inputs: Buffer[]): Scalar;
