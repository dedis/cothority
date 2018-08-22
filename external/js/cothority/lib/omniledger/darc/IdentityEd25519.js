const curve = require("@dedis/kyber-js").curve.newCurve("edwards255519");
const Schnorr = require("@dedis/kyber-js").sign.schnorr;
const Identity = require("./Identity");

/**
 * @extends Identity
 */
class IdentityEd25519 extends Identity {
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
   * Creates an IdentityEd25519 from a SignerEd25519.

   * @param {Signer} signer
   * @return {IdentityEd25519}
   */
  static fromSigner(signer) {
    let point = curve.point();
    point.unmarshalBinary(signer.point);
    return new IdentityEd25519(point);
  }

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
  toProtobufValidMessage() {
    return {
      point: this._pub.marshalBinary()
    };
  }
}
