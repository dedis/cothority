import { curve, Point, PointFactory, sign } from "@dedis/kyber";
import { Message, Properties } from "protobufjs/light";
import { EMPTY_BUFFER, registerMessage } from "../protobuf";
import IdentityWrapper, { IIdentity } from "./identity-wrapper";

const { schnorr } = sign;
const ed25519 = curve.newCurve("edwards25519");

/**
 * Identity of an Ed25519 signer
 */
export default class IdentityEd25519 extends Message<IdentityEd25519> implements IIdentity {
  static register() {
    registerMessage("IdentityEd25519", IdentityEd25519);
  }

  readonly point: Buffer;

  private _public: Point;

  constructor(props?: Properties<IdentityEd25519>) {
    super(props);

    this.point = Buffer.from(this.point || EMPTY_BUFFER);
  }

  /**
   * Get the public key as a point
   */
  get public(): Point {
    if (!this._public) {
      this._public = PointFactory.fromProto(this.point);
    }

    return this._public;
  }

  /** @inheritdoc */
  verify(msg: Buffer, signature: Buffer): boolean {
    return schnorr.verify(ed25519, this.public, msg, signature);
  }

  /** @inheritdoc */
  toWrapper() {
    return new IdentityWrapper({ ed25519: this });
  }

  /** @inheritdoc */
  toBytes(): Buffer {
    return this.point;
  }

  /** @inheritdoc */
  toString() {
    return `ed25519:${this.public.toString().toLowerCase()}`;
  }
}
