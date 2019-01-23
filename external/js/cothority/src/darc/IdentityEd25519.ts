const curve = require("@dedis/kyber-js").curve.newCurve("edwards25519");
const Schnorr = require("@dedis/kyber-js").sign.schnorr;
import {Identity} from "~/lib/cothority/darc/Identity";

/**
 * @extends Identity
 */
export class IdentityEd25519 extends Identity {
  _pub: any;

  /**
   * @param {Point}pub
   */
  constructor(pub) {
    super();
    this._pub = pub;
  }

  /**
   * Creates an IdentityEd25519 from a protobuf representation.

   * @param {Object} proto
   * @return {IdentityEd25519}
   */
  static fromProtobuf(proto) {
    let point = curve.point();
    point.unmarshalBinary(proto.point);
    return new IdentityEd25519(point);
  }

  /**
   * Creates an identity from a public key
   * @param {Uint8Array} publicKey - the key
   * @return {IdentityEd25519} - the identity
   */
  static fromPublicKey(publicKey) {
    let point = curve.point();
    point.unmarshalBinary(publicKey);
    return new IdentityEd25519(point);
  }

  /**
   * @return {Uint8Array} - the public key, in a byte array format
   */
  get public() {
    return this._pub.marshalBinary();
  }

  /**
   * Creates an IdentityEd25519 from a SignerEd25519.

   * @param {Signer} signer
   * @return {IdentityEd25519}
   */
  static fromSigner(signer) {
    return new IdentityEd25519(signer.public);
  }

  /**
   * Verify that a message is correctly signed
   *
   * @param msg
   * @param signature
   * @return {boolean}
   */
  verify(msg, signature) {
    return Schnorr.verify(curve, this._pub, msg, signature);
  }

  toString() {
    return this.typeString() + ":" + this._pub.toString().toLowerCase();
  }

  typeString() {
    return "ed25519";
  }

  /**
   * Create an object with all the necessary field needed to be a valid message
   * in the sense of protobufjs. This object can then be used with the "create"
   * method of protobuf
   *
   * @return {Object}
   */
  toObject(): object {
    return {
      ed25519: {
        point: this._pub.marshalBinary()
      }
    };
  }
}
