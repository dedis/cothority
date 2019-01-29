import { Message } from 'protobufjs';
import { curve, sign, Point } from '@dedis/kyber';
import Identity from "./identity";
import { registerMessage } from '../protobuf';
import IdentityWrapper from './identity-wrapper';

const { schnorr } = sign;
const ed25519 = curve.newCurve('edwards25519');

/**
 * Identity of an Ed25519 signer
 */
export default class IdentityEd25519 extends Message<IdentityEd25519> implements Identity {
  readonly point: Buffer;

  /**
   * Get the public key as a point
   */
  get public(): Point {
    const p = ed25519.point();
    p.unmarshalBinary(this.point);

    return p;
  }

  /** @inheritdoc */
  verify(msg: Buffer, signature: Buffer): boolean {
    return schnorr.verify(ed25519, this.public, msg, signature);
  }

  /** @inheritdoc */
  typeString() {
    return "ed25519";
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
    return `${this.typeString()}:${this.public.toString().toLowerCase()}`;
  }
}

registerMessage('IdentityEd25519', IdentityEd25519);
