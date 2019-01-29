import { Message } from "protobufjs";
import IdentityWrapper from "./identity-wrapper";

/**
 * Signature created by a signer that contains the actual signature
 * and the identity of the signer
 */
export default class Signature extends Message<Signature> {
  readonly signature: Buffer;
  readonly signer: IdentityWrapper;
}
