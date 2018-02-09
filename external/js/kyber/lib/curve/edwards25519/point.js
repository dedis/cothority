"use strict";

const BN = require("bn.js");
const crypto = require("crypto");
const Scalar = require("./scalar");
const group = require("../../index.js");

/**
 * @module curves/edwards25519/point
 */

/**
 * Represents a Point on the twisted edwards curve
 * (X:Y:Z:T) satisfying x=X/Z, y=Y/Z, XY=ZT
 *
 * The value of the parameters is expcurveted in little endian form if being
 * passed as a Uint8Array
 * @constructor
 *
 * @param {module:curves/edwards25519~Edwards25519} curve
 * @param {(number|Uint8Array|BN.jsObjcurvet)} X
 * @param {(number|Uint8Array|BN.jsObjcurvet)} Y
 * @param {(number|Uint8Array|BN.jsObjcurvet)} Z
 * @param {(number|Uint8Array|BN.jsObjcurvet)} T
 */
class Point extends group.Point {
  constructor(curve, X, Y, Z, T) {
    super();
    let _X = X;
    let _Y = Y;
    let _Z = Z;
    let _T = T;

    if (X !== undefined && X.constructor === Uint8Array) {
      _X = new BN(X, 16, "le");
    }
    if (Y !== undefined && Y.constructor === Uint8Array) {
      _Y = new BN(Y, 16, "le");
    }
    if (Z !== undefined && Z.constructor === Uint8Array) {
      _Z = new BN(Z, 16, "le");
    }
    if (T !== undefined && T.constructor === Uint8Array) {
      _T = new BN(T, 16, "le");
    }
    // the point reference is stored in an module:curves/edwards25519/point~Point to make set()
    // consistent.
    this.ref = {
      point: curve.curve.point(_X, _Y, _Z, _T),
      curve: curve
    };

    this.string = this.toString.bind(this);
    this.inspect = this.toString.bind(this);
  }

  /**
   * Returns the little endian representation of the y coordinate of
   * the Point
   *
   * @returns {string}
   */
  toString() {
    const bytes = this.marshalBinary();
    return Array.from(bytes, b =>
      ("0" + (b & 0xff).toString(16)).slice(-2)
    ).join("");
  }

  /**
   * Tests for equality between two Points derived from the same group
   *
   * @param {module:curves/edwards25519/point~Point} p2 Point module:curves/edwards25519/point~Point to compare
   * @returns {boolean}
   */
  equal(p2) {
    const b1 = this.marshalBinary();
    const b2 = p2.marshalBinary();
    for (var i = 0; i < 32; i++) {
      if (b1[i] !== b2[i]) {
        return false;
      }
    }
    return true;
  }

  // Set point to be equal to p2

  /**
   * set Set the current point to be equal to p2
   *
   * @param {module:curves/edwards25519/point~Point} p2 Point module:curves/edwards25519/point~Point
   * @returns {module:curves/edwards25519/point~Point}
   */
  set(p2) {
    this.ref = p2.ref;
    return this;
  }

  /**
   * Creates a copy of the current point
   *
   * @returns {module:curves/edwards25519/point~Point} new Point module:curves/edwards25519/point~Point
   */
  clone() {
    const point = this.ref.point;
    return new Point(this.ref.curve, point.x, point.y, point.z, point.t);
  }

  /**
   * Set to the neutral element, which is (0, 1) for twisted Edwards
   * Curve
   *
   * @returns {module:curves/edwards25519/point~Point}
   */
  null() {
    this.ref.point = this.ref.curve.curve.point(0, 1, 1, 0);
    return this;
  }

  /**
   * Set to the standard base point for this curve
   *
   * @returns {module:curves/edwards25519/point~Point}
   */
  base() {
    this.ref.point = this.ref.curve.curve.point(
      this.ref.curve.curve.g.getX(),
      this.ref.curve.curve.g.getY()
    );
    return this;
  }

  /**
   * Returns the length (in bytes) of the embedded data
   *
   * @returns {number}
   */
  embedLen() {
    // Reserve the most-significant 8 bits for pseudo-randomness.
    // Reserve the least-significant 8 bits for embedded data length.
    // (Hopefully it's unlikely we'll need >=2048-bit curves soon.)
    return Math.floor((255 - 8 - 8) / 8);
  }

  /**
   * Returns a Point with data embedded in the y coordinate
   *
   * @param {Uint8Array} data to embed with length <= embedLen
   * @param {function} callback - to generate a random byte array of given length
   *
   * @throws {TypeError} if data is not Uint8Array
   * @throws {Error} if data.length > embedLen
   * @returns {module:curves/edwards25519/point~Point}
   */
  embed(data, callback) {
    if (data.constructor !== Uint8Array) {
      throw new TypeError("data should be Uint8Array");
    }

    let dl = this.embedLen();
    if (data.length > dl) {
      throw new Error("data.length > embedLen");
    }

    if (dl > data.length) {
      dl = data.length;
    }

    callback = callback || crypto.randomBytes;

    let point_obj = new Point(this.ref.curve);
    while (true) {
      let buff = callback(32);
      let bytes = Uint8Array.from(buff);

      if (dl > 0) {
        bytes[0] = dl; // encode length in lower 8 bits
        bytes.set(data, 1); // copy in data to embed
      }

      let bnp = new BN(bytes, 16, "le");

      //if (bnp.cmp(PFSCALAR) > 0) {
      //continue; // try again
      //}

      try {
        point_obj.unmarshalBinary(bytes);
      } catch (e) {
        continue; // try again
      }
      if (dl == 0) {
        point_obj.ref.point = point_obj.ref.point.mul(new BN(8));
        if (point_obj.ref.point.isInfinity()) {
          continue; // unlucky
        }
        return point_obj;
      }

      let q = point_obj.clone();
      q.ref.point = q.ref.point.mul(this.ref.curve.curve.n);
      if (q.ref.point.isInfinity()) {
        return point_obj;
      }
    }
  }

  /**
   * Extract embedded data from a point
   *
   * @throws {Error} when length of embedded data > embedLen
   * @returns {Uint8Array}
   */
  data() {
    const bytes = this.marshalBinary();
    const dl = bytes[0];
    if (dl > this.embedLen()) {
      throw new Error("invalid embedded data length");
    }
    return bytes.slice(1, dl + 1);
  }

  /**
   * Returns the sum of two points on the curve
   *
   * @param {module:curves/edwards25519/point~Point} p1 Point module:curves/edwards25519/point~Point, addend
   * @param {module:curves/edwards25519/point~Point} p2 Point module:curves/edwards25519/point~Point, addend
   * @returns {module:curves/edwards25519/point~Point} p1 + p2
   */
  add(p1, p2) {
    const point = p1.ref.point;
    this.ref.point = this.ref.curve.curve
      .point(point.x, point.y, point.z, point.t)
      .add(p2.ref.point);
    return this;
  }

  /**
   * Subtract two points
   *
   * @param {module:curves/edwards25519/point~Point} p1 Point module:curves/edwards25519/point~Point
   * @param {module:curves/edwards25519/point~Point} p2 Point module:curves/edwards25519/point~Point
   * @returns {module:curves/edwards25519/point~Point} p1 - p2
   */
  sub(p1, p2) {
    const point = p1.ref.point;
    this.ref.point = this.ref.curve.curve
      .point(point.x, point.y, point.z, point.t)
      .add(p2.ref.point.neg());
    return this;
  }

  /**
   * Finds the negative of a point p
   * For Edwards Curves, the negative of (x, y) is (-x, y)
   *
   * @param {module:curves/edwards25519/point~Point} p Point to negate
   * @returns {module:curves/edwards25519/point~Point} -p
   */
  neg(p) {
    this.ref.point = p.ref.point.neg();
    return this;
  }

  /**
   * Multiply point p by scalar s
   *
   * @param {module:curves/edwards25519/point~Point} s Scalar
   * @param {module:curves/edwards25519/point~Point} [p] Point
   * @returns {module:curves/edwards25519/point~Point}
   */
  mul(s, p) {
    if (s.constructor !== Scalar) {
      throw new TypeError("s should be a Scalar");
    }
    p = p || null;
    const arr = s.ref.arr.fromRed();
    this.ref.point =
      p !== null ? p.ref.point.mul(arr) : this.ref.curve.curve.g.mul(arr);
    return this;
  }

  /**
   * Selects a random point
   *
   * @param {function} callback - to generate a random byte array of given length
   * @returns {module:curves/edwards25519/point~Point}
   */
  pick(callback) {
    return this.embed(new Uint8Array(), callback);
  }

  marshalSize() {
    return 32;
  }

  /**
   * Convert a ed25519 curve point into a byte representation
   *
   * @returns {Uint8Array} byte representation
   */
  marshalBinary() {
    this.ref.point.normalize();

    const buffer = this.ref.point.getY().toArray("le", 32);
    buffer[31] ^= (this.ref.point.x.isOdd() ? 1 : 0) << 7;

    return Uint8Array.from(buffer);
  }

  /**
   * Convert a Uint8Array back to a ed25519 curve point
   * {@link tools.ietf.org/html/rfc8032#scurvetion-5.1.3}
   * @param {Uint8Array} bytes
   *
   * @throws {TypeError} when bytes is not Uint8Array
   * @throws {Error} when bytes does not correspond to a valid point
   * @returns {module:curves/edwards25519/point~Point}
   */
  unmarshalBinary(bytes) {
    if (bytes.constructor !== Uint8Array) {
      throw new TypeError("bytes should be a Uint8Array");
    }
    // we create a copy bcurveause the array might be modified
    const _bytes = new Uint8Array(32);
    _bytes.set(bytes, 0);

    const odd = _bytes[31] >> 7 === 1;

    _bytes[31] &= 0x7f;
    let bnp = new BN(_bytes, 16, "le");
    if (bnp.cmp(this.ref.curve.curve.p) >= 0) {
      throw new Error("bytes > p");
    }
    this.ref.point = this.ref.curve.curve.pointFromY(bnp, odd);
  }
}
module.exports = Point;
