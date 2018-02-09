"use strict";

const Scalar = require("./scalar");
const Point = require("./point");
const crypto = require("crypto");
const elliptic = require("elliptic");
const EdDSA = elliptic.eddsa;
const ec = new EdDSA("ed25519");
const BN = require("bn.js");
const orderRed = BN.red(ec.curve.n);
const group = require("../../index.js");

/**
 * @module curves/edwards25519/curve
 */

/**
 * Represents an Ed25519 curve
 */
class Edwards25519 extends group.Group {
  constructor() {
    super();
    this.curve = ec.curve;
    this.orderRed = orderRed;
  }

  /**
   * Return the name of the curve
   *
   * @returns {string}
   */
  string() {
    return "Ed25519";
  }

  /**
   * Returns 32, the size in bytes of a Scalar on Ed25519 curve
   *
   * @returns {number}
   */
  scalarLen() {
    return 32;
  }

  /**
   * Returns a new Scalar for the prime-order subgroup of Ed25519 curve
   *
   * @returns {module:curves/edwards25519/scalar~Scalar}
   */
  scalar() {
    return new Scalar(this, this.orderRed);
  }

  /**
   * Returns 32, the size of a Point on Ed25519 curve
   *
   * @returns {number}
   */
  pointLen() {
    return 32;
  }

  /**
   * Creates a new point on the Ed25519 curve
   *
   * @returns {module:curves/edwards25519/point~Point}
   */
  point() {
    return new Point(this);
  }

  /**
   * NewKey returns a formatted Ed25519 key (avoiding subgroup attack by requiring
   * it to be a multiple of 8).
   * @returns {module:curves/edwards25519/scalar~Scalar}
   */
  newKey() {
    let bytes = crypto.randomBytes(32);
    let hash = crypto.createHash("sha512");
    hash.update(bytes);
    let scalar = Uint8Array.from(hash.digest());
    scalar[0] &= 0xf8;
    scalar[31] &= 0x3f;
    scalar[31] &= 0x40;

    return this.scalar().setBytes(scalar);
  }
}
module.exports = Edwards25519;
