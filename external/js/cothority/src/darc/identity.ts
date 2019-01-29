import IdentityWrapper from "./identity-wrapper";

/**
 * Identitiy is an abstract class for all the Darcs's identities
 */
export default interface Identity {
  /**
   * Returns true if the verification of signature on the sha-256 of msg is
   * successful or false if not.
   * @param msg       the message to verify
   * @param signature the signature to verify
   * @returns true when the signature matches the message, false otherwise
   */
  verify(msg: Buffer, signature: Buffer): boolean;

  /**
   * Get the type of the identity
   * @returns the type of the identity as a string
   */
  typeString(): string;

  /**
   * Get the wrapper used to encode the identity
   * @returns the wrapper
   */
  toWrapper(): IdentityWrapper;

  /**
   * Get the byte array representation of the public key of the identity
   * @returns the public key as buffer
   */
  toBytes(): Buffer;

  /**
   * Get the string representation of the identity
   * @return a string representation
   */
  toString(): string;
}
