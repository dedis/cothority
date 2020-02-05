import { curve, Point, sign } from "@dedis/kyber";
import Ed25519Point from "@dedis/kyber/curve/edwards25519/point";
// @ts-ignore
import Base58 from "base-58";
import { Message, Properties } from "protobufjs";
import { registerMessage } from "../protobuf";
import { IIdentity } from "./identity-wrapper";

const { schnorr } = sign;
const ed25519 = curve.newCurve("edwards25519");

export default class IdentityDid extends Message<IdentityDid>
  implements IIdentity {
  static register() {
    registerMessage("IdentityDID", IdentityDid);
  }

  point: Point;

  readonly method: string;

  readonly did: string;

  readonly walletHandle: number;

  readonly poolHandle: number;

  readonly indy: Indy;

  protected _point: Buffer;

  constructor(props?: Properties<IdentityDid>) {
    super(props);
    this.did = props.did;
    if (this.did && this.did.startsWith("did:sov")) {
      this.did = this.did.substring(this.did.lastIndexOf(":") + 1);
      this.method = "sov";
    } else {
      this.method = props.method;
    }
    this.walletHandle = props.walletHandle;
    this.poolHandle = props.poolHandle;
    this.indy = props.indy;
  }

  async init() {
    const keyBase58 = await this.indy.keyForDid(
      this.poolHandle,
      this.walletHandle,
      this.did,
    );
    this._point = Base58.decode(keyBase58);
    this.point = new Ed25519Point();
    this.point.unmarshalBinary(this._point);
  }

  /** @inheritdoc */
  verify(msg: Buffer, signature: Buffer): boolean {
    return schnorr.verify(ed25519, this.point, msg, signature);
  }

  /** @inheritdoc */
  toBytes(): Buffer {
    return Buffer.from(this.toString());
  }

  /** @inheritdoc */
  toString() {
    return `did:${this.method}:${this.did}`;
  }
}
