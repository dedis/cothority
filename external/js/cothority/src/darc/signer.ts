import { Scalar, Point } from "@dedis/kyber";
import Identity from "./identity";
import Signature from "./signature";

export default interface Signer extends Identity {
    /**
     * Signs the sha256 hash of the message. It must return
     * an array of bytes that can be verified by the
     * corresponding identity-implementation.
     * 
     * @param msg the message to sign
     * @returns the signature
     */
    sign(msg: Buffer): Signature;
}
