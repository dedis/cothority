import { Identity } from "./Identity";
import { Message } from "protobufjs";
import { IdentityEd25519 } from "./IdentityEd25519";

export class IdentityWrapper extends Message<IdentityWrapper> {
  readonly ed25519: IdentityEd25519;
}

export class Signature extends Message<Signature> {
  readonly signature: Buffer;
  readonly signer: IdentityWrapper;
}