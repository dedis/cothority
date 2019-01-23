import {Identity} from "~/lib/cothority/darc/Identity";
import {Signature} from "~/lib/cothority/darc/Signature";

export class Signer {
    constructor() {
    }

    /**
     * Signs the sha256 hash of the message. It must return
     * an array of bytes that can be verified by the
     * corresponding identity-implementation.

     * @param {Uint8Array} msg
     */
    sign(msg): Signature {
        throw new Error("Not implemented");
    }

    /**
     * Returns the private key of the signer, or throws a NoPrivateKey exception.
     *
     * @return {Scalar}
     */
    get private(): any {
        throw new Error("Not implemented");
    }

    /**
     * Returns the public key of the signer or throws a NoPublicKey exception.
     *
     * @return {Point}
     */
    get public(): any {
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
