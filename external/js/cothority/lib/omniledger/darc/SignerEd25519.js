const Signer = require("./Signer");
const curve = require("@dedis/kyber-js").curve.newCurve("edwards25519");
const Schnorr = require("@dedis/kyber-js").sign.schnorr;
const Identity = require("./IdentityEd25519");

/**
 * @extends Signer
 */
class SignerEd25519 extends Signer {
  constructor(pub, priv) {
    super();
    this._pub = pub;
    this._priv = priv;
  }

  static fromByteArray(bytes) {
    const priv = curve.scalar();
    priv.unmarshalBinary(bytes);
    return new SignerEd25519(curve.point().base().mul(priv), priv);
  }

  get private() {
    return this._priv;
  }

  get public() {
    return this._pub;
  }

  get identity() {
    return new Identity(this._pub);
  }

  sign(msg) {
    return Schnorr.sign(curve, this._priv, msg);
  }
}

module.exports = SignerEd25519;
