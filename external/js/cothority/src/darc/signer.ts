import { Scalar, Point } from "@dedis/kyber";
import Identity from "./identity";
import Signature from "./signature";

export default class Signer {
    constructor() { }

    /**
     * Signs the sha256 hash of the message. It must return
     * an array of bytes that can be verified by the
     * corresponding identity-implementation.
     * 
     * @param msg the message to sign
     * @returns the signature
     */
    sign(msg: Buffer): Signature {
        throw new Error("Not implemented");
    }

    /**
     * Returns the private key of the signer, or throws a NoPrivateKey exception.
     *
     * @returns the private key as a Scalar
     */
    get private(): Scalar {
        throw new Error("Not implemented");
    }

    /**
     * Returns the public key of the signer or throws a NoPublicKey exception.
     *
     * @returns the public key as a Point
     */
    get public(): Point {
        throw new Error("Not implemented");
    }

    /**
     * Returns an identity of the signer.
     *
     * @returns the identity of the signer
     */
    get identity(): Identity {
        throw new Error("Not implemented");
    }
}
