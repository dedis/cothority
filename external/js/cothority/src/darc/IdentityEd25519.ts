import { curve, sign, Point } from '@dedis/kyber';
import {Identity} from "./Identity";
import { Message } from 'protobufjs';
import { registerMessage } from '../protobuf';
import { IdentityWrapper } from './Signature';

const { schnorr } = sign;

const ed25519 = curve.newCurve('edwards25519');

/**
 * @extends Identity
 */
export class IdentityEd25519 extends Message<IdentityEd25519> implements Identity {
  readonly point: Buffer;

  get public(): Point {
    const p = ed25519.point();
    p.unmarshalBinary(this.point);

    return p;
  }

  /**
   * Verify that a message is correctly signed
   *
   * @param msg
   * @param signature
   * @return {boolean}
   */
  verify(msg: Buffer, signature: Buffer): boolean {
    return schnorr.verify(ed25519, this.public, msg, signature);
  }

  toString() {
    return this.typeString() + ":" + this.public.toString().toLowerCase();
  }

  toWrapper() {
    return new IdentityWrapper({ ed25519: this });
  }

  toBytes(): Buffer {
    return this.point;
  }

  typeString() {
    return "ed25519";
  }
}

registerMessage('IdentityEd25519', IdentityEd25519);
