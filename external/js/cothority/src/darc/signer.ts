import Identity from "./identity";

export default interface ISigner extends Identity {
    /**
     * Signs the sha256 hash of the message. It must return
     * an array of bytes that can be verified by the
     * corresponding identity-implementation.
     *
     * @param msg the message to sign
     * @returns the signature
     */
    sign(msg: Buffer): Buffer;
}
