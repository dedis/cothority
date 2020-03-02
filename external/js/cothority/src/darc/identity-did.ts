import { IIdentity } from "./identity-wrapper";
import { Message, Properties } from "protobufjs";
import { registerMessage } from "../protobuf";
import { sign, curve, Point } from "@dedis/kyber";
import Ed25519Point from "@dedis/kyber/curve/edwards25519/point";
// @ts-ignore
import Base58 from "base-58";

const { schnorr } = sign;
const ed25519 = curve.newCurve("edwards25519");

export default class IdentityDid extends Message<IdentityDid>
  implements IIdentity {
  static register() {
    registerMessage("IdentityDID", IdentityDid);
  }

  protected _point: Buffer;

  point: Point;

  readonly did: string;

  readonly walletHandle: number;

  readonly poolHandle: number;

  readonly indy: Indy;

  constructor(props?: Properties<IdentityDid>) {
    super(props);
    this.did = props.did;
    this.walletHandle = props.walletHandle;
    this.poolHandle = props.poolHandle;
    this.indy = props.indy;
  }

  async init() {
    const keyBase58 = await this.indy.keyForDid(
      this.poolHandle,
      this.walletHandle,
      this.did
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
    return Buffer.from(this.did);
  }

  /** @inheritdoc */
  toString() {
    return this.did;
  }
}

