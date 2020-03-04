import { Properties } from "protobufjs";
import IdentityDid from "./identity-did";
import ISigner from "./signer";

export default class SignerDid extends IdentityDid implements ISigner {
  signFn: (data: Buffer, did: string) => Promise<Buffer>;

  constructor(
    signFn: (data: Buffer, did: string) => Promise<Buffer>,
    props: Properties<IdentityDid>,
  ) {
    super(props);
    this.signFn = signFn;
  }

  async sign(msg: Buffer): Promise<Buffer> {
    return this.signFn(msg, this.toString());
  }
}
