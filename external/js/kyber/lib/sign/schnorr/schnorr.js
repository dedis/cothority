const group = require("./../../group");
// XXX to be changed to an interface style
const nist = group.nist;
const curve = nist.Curve;
const scalarCons = nist.Scalar;
const pointCons = nist.Point;

const hash = require("hash.js");
/**
 *
 * Sign computes a Schnorr signature over the given message.
 *
 * @param privateKey private key scalar to sign with
 * @param message message over which the signature is computed
 * @return signature as a Uint8Array
 * */
function Sign(suite, privateKey, message) {
  if (suite.constructor !== curve) {
    throw "first argument must be a suite";
  }
  if (privateKey.constructor !== scalarCons) {
    throw "second argument must be a scalar";
  }
  if (message.constructor !== Uint8Array) {
    throw "third argument must be Uint8Array";
  }

  // generate r & R
  const r = suite.scalar().pick();
  const R = suite.point().mul(r, null);
  const buffR = R.marshalBinary();

  // generate public key
  const pub = suite.point().mul(privateKey, null);

  // generate challenge
  const challenge = hashSchnorr(suite, buffR, pub.marshalBinary(), message);

  // generate signature
  const s = suite.scalar().mul(privateKey, challenge);
  s.add(s, r);

  // concatenate R || s
  const buffS = s.marshalBinary();
  const buffSig = new Uint8Array(buffR.length + buffS.length);
  buffSig.set(buffR);
  buffSig.set(buffS, buffR.length);
  return buffSig;
}

/**
 *
 * Verify verify if the signature of the message is valid under the given public
 * key.
 *
 * @params suite suite to use
 * @params publicKey public key under which to verify the signature
 * @params message message that is signed
 * @params signature signature made over the given message
 * @return boolean true if signature is valid or false otherwise.
 * */
function Verify(suite, publicKey, message, signature) {
  if (suite.constructor !== curve) {
    throw "first argument must be a suite";
  }
  if (publicKey.constructor !== pointCons) {
    throw "second argument must be a point";
  }
  if (message.constructor !== Uint8Array) {
    throw "third argument must be a Uint8Array";
  }
  if (signature.constructor !== Uint8Array) {
    throw "fourth argument must be a Uint8Array";
  }

  // check the signature size
  const plen = suite.pointLen();
  const slen = suite.scalarLen();
  const totalSize = plen + slen;
  if (signature.length != totalSize) {
    return false;
  }

  // unmarshal R || s
  const buffR = signature.slice(0, plen);
  const R = suite.point();
  R.unmarshalBinary(buffR);

  const buffs = signature.slice(plen, signature.lengh);
  const s = suite.scalar();
  s.unmarshalBinary(buffs);

  // recompute challenge = H(R || P || M)
  const buffPub = publicKey.marshalBinary();
  const challenge = hashSchnorr(suite, buffR, buffPub, message);

  // compute sG
  const left = suite.point().mul(s, null);
  // compute R + challenge * Public
  const right = suite.point().mul(challenge, publicKey);
  right.add(right, R);

  if (!right.equal(left)) {
    return false;
  }
  return true;
}

/**
 *
 * hashSchnorr returns a scalar out of hashing the given inputs.
 * @params inputs a list of Uint8Array
 * @return a scalar
 *
 **/
function hashSchnorr(suite, ...inputs) {
  const h = hash.sha512();
  for (let i of inputs) {
    h.update(i);
  }
  const scalar = suite.scalar();
  scalar.setBytes(Uint8Array.from(h.digest()));
  return scalar;
}

module.exports.sign = Sign;
module.exports.verify = Verify;
