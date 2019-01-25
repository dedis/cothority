/**
 * Identitiy is an abstract class for all the Darcs's identities
 */
export interface Identity {
  /**
   * Returns true if the verification of signature on the sha-256 of msg is
   * successful or false if not.

   * @param {Uint8Array} msg
   * @param {Uint8Array} signature
   * @return {boolean}
   */
  verify(msg: Buffer, signature: Buffer): boolean;

  /**
   * @return {string}
   */
  toString(): string;

  toBytes(): Buffer;

  /**
   * @return {string}
   */
  typeString(): string;
}
