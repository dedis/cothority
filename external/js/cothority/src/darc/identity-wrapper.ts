import { Message } from "protobufjs/light";
import IdentityDarc from "./identity-darc";
import IdentityEd25519 from "./identity-ed25519";

/**
 * Protobuf representation of an identity
 */
export default class IdentityWrapper extends Message<IdentityWrapper> {
  readonly ed25519: IdentityEd25519;
  readonly darc: IdentityDarc;

  /**
   * Get the inner identity as bytes
   * @returns the bytes
   */
  toBytes(): Buffer {
    if (this.ed25519) {
      return this.ed25519.public.marshalBinary();
    }
    if (this.darc) {
      return this.darc.toBytes();
    }

    return Buffer.from([]);
  }
}

/**
 * Identity is an abstract class for all the Darcs's identities
 */
export interface IIdentity {
  /**
   * Returns true if the verification of signature on the sha-256 of msg is
   * successful or false if not.
   * @param msg       the message to verify
   * @param signature the signature to verify
   * @returns true when the signature matches the message, false otherwise
   */
  verify(msg: Buffer, signature: Buffer): boolean;

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
