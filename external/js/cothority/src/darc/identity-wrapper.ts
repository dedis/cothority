import { Message } from "protobufjs";
import IdentityDarc from "./identity-darc";
import IdentityEd25519 from "./identity-ed25519";

/**
 * Protobuf representation of an identity
 */
export default class IdentityWrapper extends Message<IdentityWrapper> {
  readonly ed25519: IdentityEd25519;
  readonly darc: IdentityDarc;
}
