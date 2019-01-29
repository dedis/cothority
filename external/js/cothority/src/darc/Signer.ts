import { Identity } from "./Identity";
import { Signature } from "./Signature";
import { Scalar, Point } from "@dedis/kyber";

export class Signer {
    constructor() { }

    /**
     * Signs the sha256 hash of the message. It must return
     * an array of bytes that can be verified by the
     * corresponding identity-implementation.

     * @param {Uint8Array} msg
     */
    sign(msg: Buffer): Signature {
        throw new Error("Not implemented");
    }

    /**
     * Returns the private key of the signer, or throws a NoPrivateKey exception.
     *
     * @return {Scalar}
     */
    get private(): Scalar {
        throw new Error("Not implemented");
    }

    /**
     * Returns the public key of the signer or throws a NoPublicKey exception.
     *
     * @return {Point}
     */
    get public(): Point {
        throw new Error("Not implemented");
    }

    /**
     * Returns an identity of the signer.
     *
     * @return {Identity}
     */
    get identity(): Identity {
        throw new Error("Not implemented");
    }
}
