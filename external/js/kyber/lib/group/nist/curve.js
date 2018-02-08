"use strict";

const Scalar = require("./scalar");
const Point = require("./point");
const crypto = require("crypto");
const elliptic = require("elliptic");
const BN = require("bn.js");
const group = require("../../index.js");

/**
 * @module curves/nist/curve
 */

/**
 * Class Weierstrass defines the weierstrass form of
 * elliptic curves
 */
class Weierstrass extends group.Group {
  /**
   * Create a new Weierstrass Curve
   *
   * @param {object} config - Curve configuration
   * @param {String} config.name - Curve name
   * @param {(String|Uint8Array|BN.jsObject)} config.p - Order of the underlying field. Little Endian if string or Uint8Array.
   * @param {(String|Uint8Array|BN.jsObject)} config.a - Curve Parameter a. Little Endian if string or Uint8Array.
   * @param {(String|Uint8Array|BN.jsObject)} config.b - Curve Parameter b. Little Endian if string or Uint8Array.
   * @param {(String|Uint8Array|BN.jsObject)} config.n - Order of the base point. Little Endian if string or Uint8Array
   * @param {(String|Uint8Array|BN.jsObject)} config.gx - x coordinate of the base point. Little Endian if string or Uint8Array
   * @param {(String|Uint8Array|BN.jsObject)} config.gy - y coordinate of the base point. Little Endian if string or Uint8Array
   * @param {number} config.bitSize - the size of the underlying field.
   */
  constructor(config) {
    super();
    let { name, bitSize, gx, gy, ...options } = config;
    this.name = name;
    options["g"] = [new BN(gx, 16, "le"), new BN(gy, 16, "le")];
    for (let k in options) {
      if (k === "g") {
        continue;
      }
      options[k] = new BN(options[k], 16, "le");
    }
    this.curve = new elliptic.curve.short(options);
    this.bitSize = bitSize;
    this.redN = BN.red(options.n);
  }

  /**
   * Returns the name of the curve
   *
   * @returns {string}
   */
  string() {
    return this.name;
  }

  _coordLen() {
    return (this.bitSize + 7) >> 3;
  }

  /**
   * Returns the size in bytes of a scalar
   *
   * @returns {number}
   */
  scalarLen() {
    return (this.curve.n.bitLength() + 7) >> 3;
  }

  /**
   * Returns the size in bytes of a point
   *
   * @returns {module:curves/nist/scalar~Scalar}
   */
  scalar() {
    return new Scalar(this, this.redN);
  }

  /**
   * Returns the size in bytes of a point
   *
   * @returns {number}
   */
  pointLen() {
    // ANSI X9.62: 1 header byte plus 2 coords
    return this._coordLen() * 2 + 1;
  }

  /**
   * Returns a new Point
   *
   * @returns {module:curves/nist/point~Point}
   */
  point() {
    return new Point(this);
  }
}

module.exports = Weierstrass;
