"use strict";

const BN = require("bn.js");
const crypto = require("crypto");
const random = require("../../random");
const group = require("../../index.js");

/**
 * @module curves/edwards25519/scalar
 */

/**
 * Scalar represents a value in GF(2^252 + 27742317777372353535851937790883648493)
 * @Constructor
 */
class Scalar extends group.Scalar {
  constructor(curve, red) {
    super();
    this.ref = {
      arr: new BN(0, 16).toRed(red),
      curve: curve,
      red: red
    };
    this.inspect = this.toString.bind(this);
    this.string = this.toString.bind(this);
  }

  /**
   * Equality test for two Scalars derived from the same Group
   *
   * @param {module:curves/edwards25519/scalar~Scalar} s2 Scalar
   * @returns {boolean}
   */
  equal(s2) {
    return this.ref.arr.fromRed().cmp(s2.ref.arr.fromRed()) == 0;
  }

  /**
   * Sets the receiver equal to another Scalar a
   *
   * @param {module:curves/edwards25519/scalar~Scalar} a Scalar
   * @returns {module:curves/edwards25519/scalar~Scalar}
   */
  set(a) {
    this.ref = a.ref;
    return this;
  }

  /**
   * Returns a copy of the scalar
   *
   * @returns {module:curves/edwards25519/scalar~Scalar}
   */
  clone() {
    return new Scalar(this.ref.curve, this.ref.red).setBytes(
      new Uint8Array(this.ref.arr.fromRed().toArray("le"))
    );
  }

  /**
   * Set to the additive identity (0)
   *
   * @returns {module:curves/edwards25519/scalar~Scalar}
   */
  zero() {
    this.ref.arr = new BN(0, 16).toRed(this.ref.red);
    return this;
  }

  /**
   * Set to the modular sums of scalars s1 and s2
   *
   * @param {module:curves/edwards25519/scalar~Scalar} s1 Scalar
   * @param {module:curves/edwards25519/scalar~Scalar} s2 Scalar
   * @returns {module:curves/edwards25519/scalar~Scalar} s1 + s2
   */
  add(s1, s2) {
    this.ref.arr = s1.ref.arr.redAdd(s2.ref.arr);
    return this;
  }

  /**
   * Set to the modular difference
   *
   * @param {module:curves/edwards25519/scalar~Scalar} s1 Scalar
   * @param {module:curves/edwards25519/scalar~Scalar} s2 Scalar
   * @returns {module:curves/edwards25519/scalar~Scalar} s1 - s2
   */
  sub(s1, s2) {
    this.ref.arr = s1.ref.arr.redSub(s2.ref.arr);
    return this;
  }

  /**
   * Set to the modular negation of scalar a
   *
   * @param {module:curves/edwards25519/scalar~Scalar} a Scalar
   * @returns {module:curves/edwards25519/scalar~Scalar}
   */
  neg(a) {
    this.ref.arr = a.ref.arr.redNeg();
    return this;
  }

  /**
   * Set to the multiplicative identity (1)
   *
   * @returns {module:curves/edwards25519/scalar~Scalar}
   */
  one() {
    this.ref.arr = new BN(1, 16).toRed(this.ref.red);
    return this;
  }

  /**
   * Set to the modular products of scalars s1 and s2
   *
   * @param {module:curves/edwards25519/scalar~Scalar} s1
   * @param {module:curves/edwards25519/scalar~Scalar} s2
   * @returns {module:curves/edwards25519/scalar~Scalar}
   */
  mul(s1, s2) {
    this.ref.arr = s1.ref.arr.redMul(s2.ref.arr);
    return this;
  }

  /**
   * Set to the modular division of scalar s1 by scalar s2
   *
   * @param s1
   * @param s2
   * @returns {module:curves/edwards25519/scalar~Scalar}
   */
  div(s1, s2) {
    this.ref.arr = s1.ref.arr.redMul(s2.ref.arr.redInvm());
    return this;
  }

  /**
   * Set to the modular inverse of scalar a
   *
   * @param a
   * @returns {module:curves/edwards25519/scalar~Scalar}
   */
  inv(a) {
    this.ref.arr = a.ref.arr.redInvm();
    return this;
  }

  /**
   * Sets the scalar from little-endian Uint8Array
   * and reduces to the appropriate modulus
   * @param {Uint8Array} b - bytes
   *
   * @throws {TypeError} when b is not Uint8Array
   * @returns {module:curves/edwards25519/scalar~Scalar}
   */
  setBytes(b) {
    if (b.constructor !== Uint8Array) {
      throw new TypeError();
    }
    this.ref.arr = new BN(b, 16, "le").toRed(this.ref.red);
    return this;
  }

  /**
   * Returns a big-endian representation of the scalar
   *
   * @returns {Uint8Array}
   */
  bytes() {
    return new Uint8Array(this.ref.arr.fromRed().toArray("be"));
  }

  toString() {
    let bytes = this.ref.arr.fromRed().toArray("le", 32);
    return Array.from(bytes, b =>
      ("0" + (b & 0xff).toString(16)).slice(-2)
    ).join("");
  }

  /**
   * Set to a random scalar
   *
   * @param {function} callback - to generate random byte array of given length
   * @returns {module:curves/edwards25519/scalar~Scalar}
   */
  pick(callback) {
    callback = callback || crypto.randomBytes;
    const bytes = random.int(this.ref.curve.curve.n, callback);
    this.ref.arr = new BN(bytes, 16).toRed(this.ref.red);
    return this;
  }

  marshalSize() {
    return 32;
  }

  /**
   * Returns the binary representation (little endian) of the scalar
   *
   * @returns {Uint8Array}
   */
  marshalBinary() {
    return new Uint8Array(this.ref.arr.fromRed().toArray("le", 32));
  }

  /**
   * Reads the binary representation (little endian) of scalar
   *
   * @param bytes
   */
  unmarshalBinary(bytes) {
    if (bytes.constructor !== Uint8Array) {
      throw new TypeError("bytes should be Uint8Array");
    }

    if (bytes.length > this.marshalSize()) {
      throw new Error("bytes.length > marshalSize");
    }
    this.ref.arr = new BN(bytes, 16, "le").toRed(this.ref.red);
  }
}
module.exports = Scalar;
