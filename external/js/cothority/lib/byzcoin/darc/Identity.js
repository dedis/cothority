/**
 * Identitiy is an abstract class for all the Darcs's identities
 */
class Identity {
  constructor() {
  }

  /**
   * Returns a serialized version of the public key
   * @return {Uint8Array}
   */
  get public() {
    throw new Error("Not implemented");
  }

  /**
   * Returns true if the verification of signature on the sha-256 of msg is
   * successful or false if not.

   * @param {Uint8Array} msg
   * @param {Uint8Array} signature
   * @return {boolean}
   */
  verify(msg, signature) {
    throw new Error("Not implemented");
  }

  /**
   * @return {string}
   */
  toString() {
    throw new Error("Not implemented");
  }

  /**
   * @return {string}
   */
  typeString() {
    throw new Error("Not implemented");
  }

  /**
   * Create an object with all the necessary field needed to be a valid message
   * in the sense of protobufjs. This object can then be used with the "create"
   * method of protobuf
   *
   * @return {Object}
   */
  toProtobufValidMessage() {
    throw new Error("Not implemented");
  }
}

module.exports = Identity;
