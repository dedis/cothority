"use strict";

const BN = require("bn.js");
const crypto = require("crypto");
const random = require("../../random");
const group = require("../../index.js");

/**
 * @module curves/nist/scalar
 */

/**
 * Scalar
 * @param {module:curves/nist/curve~Weirstrass} curve
 * @param {BN.Red} red - BN.js Reduction context
 * @constructor
 */
class Scalar extends group.Scalar {
  constructor(curve, red) {
    super();
    this.ref = {
      arr: new BN(0, 16).toRed(red),
      red: red,
      curve: curve
    };
    this.inspect = this.toString.bind(this);
    this.string = this.toString.bind(this);
  }

  /**
   * Equality test for two Scalars derived from the same Group
   *
   * @param {module:curves/nist/scalar~Scalar} s2 Scalar
   * @return {boolean}
   */
  equal(s2) {
    return this.ref.arr.fromRed().cmp(s2.ref.arr.fromRed()) == 0;
  }

  /**
   * Sets the receiver equal to another Scalar a
   *
   * @param {module:curves/nist/scalar~Scalar} a Scalar
   * @return {module:curves/nist/scalar~Scalar}
   */
  set(a) {
    this.ref = a.ref;
    return this;
  }

  /**
   * Returns a copy of the scalar
   *
   * @return {module:curves/nist/scalar~Scalar}
   */
  clone() {
    return new Scalar(this.ref.curve, this.ref.red).setBytes(
      new Uint8Array(this.ref.arr.fromRed().toArray("be"))
    );
  }

  /**
   * Set to the additive identity (0)
   *
   * @return {module:curves/nist/scalar~Scalar}
   */
  zero() {
    this.ref.arr = new BN(0, 16).toRed(this.ref.red);
    return this;
  }

  /**
   * Set to the modular sums of scalars s1 and s2
   *
   * @param {module:curves/nist/scalar~Scalar} s1 Scalar
   * @param {module:curves/nist/scalar~Scalar} s2 Scalar
   * @return {module:curves/nist/scalar~Scalar} s1 + s2
   */
  add(s1, s2) {
    this.ref.arr = s1.ref.arr.redAdd(s2.ref.arr);
    return this;
  }

  /**
   * Set to the modular difference
   *
   * @param {module:curves/nist/scalar~Scalar} s1 Scalar
   * @param {module:curves/nist/scalar~Scalar} s2 Scalar
   * @return {module:curves/nist/scalar~Scalar} s1 - s2
   */
  sub(s1, s2) {
    this.ref.arr = s1.ref.arr.redSub(s2.ref.arr);
    return this;
  }

  /**
   * Set to the modular negation of scalar a
   *
   * @param {module:curves/nist/scalar~Scalar} a Scalar
   * @return {module:curves/nist/scalar~Scalar}
   */
  neg(a) {
    this.ref.arr = a.ref.arr.redNeg();
    return this;
  }

  /**
   * Set to the multiplicative identity (1)
   *
   * @return {module:curves/nist/scalar~Scalar}
   */
  one() {
    this.ref.arr = new BN(1, 16).toRed(this.ref.red);
    return this;
  }

  /**
   * Set to the modular products of scalars s1 and s2
   *
   * @param {module:curves/nist/scalar~Scalar} s1
   * @param {module:curves/nist/scalar~Scalar} s2
   * @return {module:curves/nist/scalar~Scalar}
   */
  mul(s1, s2) {
    this.ref.arr = s1.ref.arr.redMul(s2.ref.arr);
    return this;
  }

  /**
   * Set to the modular division of scalar s1 by scalar s2
   *
   * @param {module:curves/nist/scalar~Scalar} s1
   * @param {module:curves/nist/scalar~Scalar} s2
   * @return {module:curves/nist/scalar~Scalar}
   */
  div(s1, s2) {
    this.ref.arr = s1.ref.arr.redMul(s2.ref.arr.redInvm());
    return this;
  }

  /**
   * Set to the modular inverse of scalar a
   *
   * @param {module:curves/nist/scalar~Scalar} a
   * @return {module:curves/nist/scalar~Scalar}
   */
  inv(a) {
    this.ref.arr = a.ref.arr.redInvm();
    return this;
  }

  /**
   * Sets the scalar from big-endian Uint8Array
   * and reduces to the appropriate modulus
   * @param {Uint8Array} b
   *
   * @throws {TypeError} when b is not Uint8Array
   * @return {module:curves/nist/scalar~Scalar}
   */
  setBytes(b) {
    if (b.constructor !== Uint8Array) {
      throw new TypeError("b should be a Uint8Array");
    }
    this.ref.arr = new BN(b, 16, "be").toRed(this.ref.red);
    return this;
  }

  /**
   * Returns a big-endian representation of the scalar
   *
   * @return {Uint8Array}
   */
  bytes() {
    return new Uint8Array(this.ref.arr.fromRed().toArray("be"));
  }

  toString() {
    let bytes = this.ref.arr.fromRed().toArray("be");
    return Array.from(bytes, b => {
      return ("0" + (b & 0xff).toString(16)).slice(-2);
    }).join("");
  }

  /**
   * Set to a random scalar
   *
   * param {function} [callback] - to generate randomBytes of given length
   * @return {module:curves/nist/scalar~Scalar}
   */
  pick(callback) {
    callback = callback || crypto.randomBytes;
    let bytes = random.int(this.ref.curve.curve.n, callback);
    this.setBytes(bytes);
    return this;
  }

  marshalSize() {
    return this.ref.curve.scalarLen();
  }

  /**
   * Returns the binary representation (big endian) of the scalar
   *
   * @return {Uint8Array}
   */
  marshalBinary() {
    return new Uint8Array(
      this.ref.arr.fromRed().toArray("be", this.ref.curve.scalarLen())
    );
  }

  /**
   * Reads the binary representation (big endian) of scalar
   *
   * @param {Uint8Array} bytes
   * @return {undefined}
   */
  unmarshalBinary(bytes) {
    if (bytes.constructor !== Uint8Array) {
      throw new TypeError("bytes should be a Uint8Array");
    }

    if (bytes.length !== this.marshalSize()) {
      throw new Error("bytes.length != marshalSize");
    }
    let bnObj = new BN(bytes, 16);
    if (bnObj.cmp(this.ref.curve.curve.n) > 0) {
      throw new Error("bytes > q");
    }
    this.setBytes(bytes);
  }
}

module.exports = Scalar;
