import { Message } from "protobufjs";
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
