"use strict";

const BN = require("bn.js");
const crypto = require("crypto");
const Scalar = require("./scalar");
const constants = require("../../constants");

module.exports = Point;

/**
 * @module group/nist/point
 */

/**
 * Represents a Point on the nist curve
 *
 * The value of the parameters is expected in little endian form if being
 * passed as a Uint8Array
 * @constructor
 *
 * @param {module:group/nist~Weierstrass} curve - Weierstrass curve
 * @param {(number|Uint8Array|BN.jsObject)} x
 * @param {(number|Uint8Array|BN.jsObject)} y
 */
function Point(curve, x, y) {
  let _x = x || null;
  let _y = y || null;

  if (x !== undefined && x.constructor === Uint8Array) {
    _x = new BN(x, 16, "le");
  }
  if (y !== undefined && y.constructor === Uint8Array) {
    _y = new BN(y, 16, "le");
  }

  // the point reference is stored in an object to make set()
  // consistent.
  this.ref = {
    curve: curve,
    point: curve.curve.point(_x, _y)
  };
}

/**
 * Returns the little endian representation of the y coordinate of
 * the Point
 *
 * @return {string}
 */
Point.prototype.toString = function() {
  if (this.ref.point.inf) {
    return "(0,0)";
  }
  return (
    "(" +
    this.ref.point.x.fromRed().toString(10) +
    "," +
    this.ref.point.y.fromRed().toString(10) +
    ")"
  );
};

Point.prototype.string = Point.prototype.toString;
Point.prototype.inspect = Point.prototype.toString;

/**
 * Tests for equality between two Points derived from the same group
 *
 * @param {module:group/nist~Point} p2 Point object to compare
 * @return {boolean}
 */
Point.prototype.equal = function(p2) {
  if (this.ref.point.isInfinity() ^ p2.ref.point.isInfinity()) {
    return false;
  }
  if (this.ref.point.isInfinity() & p2.ref.point.isInfinity()) {
    return true;
  }
  return (
    this.ref.point.x.cmp(p2.ref.point.x) === 0 &&
    this.ref.point.y.cmp(p2.ref.point.y) === 0
  );
};

// Set point to be equal to p2

/**
 * set Set the current point to be equal to p2
 *
 * @param {module:group/nist~Point} p2 Point object
 * @return {module:group/nist~Point}
 */
Point.prototype.set = function(p2) {
  this.ref = p2.ref;
  return this;
};

/**
 * Creates a copy of the current point
 *
 * @return {module:group/nist~Point} new Point object
 */
Point.prototype.clone = function() {
  const point = this.ref.point;
  return new Point(this.ref.curve, point.x, point.y);
};

/**
 * Set to the neutral element for the curve
 * Modifies the receiver
 *
 * @return {module:group/nist~Point}
 */
Point.prototype.null = function() {
  this.ref.point = this.ref.curve.curve.point(null, null);
  return this;
};

/**
 * Set to the standard base point for this curve
 * Modifies the receiver
 *
 * @return {module:group/nist~Point}
 */
Point.prototype.base = function() {
  const g = this.ref.curve.curve.g;
  this.ref.point = this.ref.curve.curve.point(g.x, g.y);
  return this;
};

/**
 * Returns the length (in bytes) of the embedded data
 *
 * @return {number}
 */
Point.prototype.embedLen = function() {
  // Reserve the most-significant 8 bits for pseudo-randomness.
  // Reserve the least-significant 8 bits for embedded data length.
  // (Hopefully it's unlikely we'll need >=2048-bit curves soon.)
  return (this.ref.curve.curve.p.bitLength() - 8 - 8) >> 3;
};

/**
 * Returns a Point with data embedded in the y coordinate
 *
 * @param {Uint8Array} data - data to embed with length <= embedLen
 * @param {function} [callback] - to generate a random byte array of given length
 *
 * @throws {TypeError} if data is not Uint8Array
 * @throws {Error} if data.length > embedLen
 * @return {module:group/nist~Point}
 */
Point.prototype.embed = function(data, callback) {
  if (data.constructor !== Uint8Array) {
    throw new TypeError("data should be Uint8Array");
  }

  let l = this.ref.curve._coordLen();
  let dl = this.embedLen();
  if (data.length > dl) {
    throw new Error("data.length > dl");
  }

  if (dl > data.length) {
    dl = data.length;
  }

  callback = callback || crypto.randomBytes;

  while (true) {
    const bitLen = this.ref.curve.curve.p.bitLength();
    const buffLen = bitLen >> 3;
    let buff = callback(buffLen);
    let bytes = Uint8Array.from(buff);
    let highbits = bitLen & 7;
    if (highbits != 0) {
      bytes[0] &= ~(0xff << highbits);
    }

    if (dl > 0) {
      bytes[l - 1] = dl; // encode length in lower 8 bits
      bytes.set(data, l - dl - 1); // copy in data to embed
    }
    //console.log(bytes);

    let x = new BN(bytes, 16, "be");
    if (x.cmp(this.ref.curve.curve.p) > 0) {
      continue;
    }

    let xRed = x.toRed(this.ref.curve.curve.red);
    let aX = xRed.redMul(this.ref.curve.curve.a);
    // y^2 = x^3 + ax + b
    let y2 = xRed
      .redSqr()
      .redMul(xRed)
      .redAdd(aX)
      .redAdd(this.ref.curve.curve.b);

    let y = y2.redSqrt();

    let b = callback(1);
    if ((b[0] & 0x80) !== 0) {
      y = this.ref.curve.curve.p.sub(y).toRed(this.ref.curve.curve.red);
    }

    // check if it is a valid point
    let y2t = y.redSqr();
    if (y2t.cmp(y2) === 0) {
      return new Point(this.ref.curve, xRed, y);
    }
  }
};

/**
 * Extract embedded data from a point
 *
 * @throws {Error} when length of embedded data > embedLen
 * @return {Uint8Array}
 */
Point.prototype.data = function() {
  const l = this.ref.curve._coordLen();
  let b = Uint8Array.from(this.ref.point.x.fromRed().toArray("be", l));
  const dl = b[l - 1];
  if (dl > this.embedLen()) {
    throw new Error("invalid embed data length");
  }
  return b.slice(l - dl - 1, l - 1);
};

/**
 * Returns the sum of two points on the curve
 * Modifies the receiver
 *
 * @param {module:group/nist~Point} p1 Point object, addend
 * @param {module:group/nist~Point} p2 Point object, addend
 * @return {module:group/nist~Point} p1 + p2
 */
Point.prototype.add = function(p1, p2) {
  const point = p1.ref.point;
  this.ref.point = this.ref.curve.curve
    .point(point.x, point.y)
    .add(p2.ref.point);
  return this;
};

/**
 * Subtract two points
 * Modifies the receiver
 *
 * @param {module:group/nist~Point} p1 Point object
 * @param {module:group/nist~Point} p2 Point object
 * @return {module:group/nist~Point} p1 - p2
 */
Point.prototype.sub = function(p1, p2) {
  const point = p1.ref.point;
  this.ref.point = this.ref.curve.curve
    .point(point.x, point.y)
    .add(p2.ref.point.neg());
  return this;
};

/**
 * Finds the negative of a point p
 * Modifies the receiver
 *
 * @param {module:group/nist~Point} p Point to negate
 * @return {module:group/nist~Point} -p
 */
Point.prototype.neg = function(p) {
  this.ref.point = p.ref.point.neg();
  return this;
};

/**
 * Multiply point p by scalar s.
 * If p is not passed then multiplies the base point of the curve with
 * scalar s
 * Modifies the receiver
 *
 * @param {module:group/nist~Scalar} s Scalar
 * @param {module:group/nist~Point} [p=null] Point
 * @return {module:group/nist~Point}
 */
Point.prototype.mul = function(s, p) {
  if (s.constructor !== Scalar) {
    throw new TypeError("s should be a Scalar");
  }
  p = p || null;
  const arr = s.ref.arr.fromRed();
  this.ref.point =
    p !== null ? p.ref.point.mul(arr) : this.ref.curve.curve.g.mul(arr);
  return this;
};

/**
 * Selects a random point
 *
 * @param {function} [callback] - to generate a random byte array of given length
 * @return {module:group/nist~Point}
 */
Point.prototype.pick = function(callback) {
  return this.embed(new Uint8Array(), callback);
};

Point.prototype.marshalSize = function() {
  // uncompressed ANSI X9.62 representation
  return this.ref.curve.pointLen();
};

/**
 * converts a point into the form specified in section 4.3.6 of ANSI X9.62.
 *
 * @return {Uint8Array} byte representation
 */
Point.prototype.marshalBinary = function() {
  const byteLen = this.ref.curve._coordLen();
  let buf = new Uint8Array(this.ref.curve.pointLen());
  buf[0] = 4; // uncompressed point

  let xBytes = this.ref.point.x.fromRed().toArray("be");
  buf.set(xBytes, 1 + byteLen - xBytes.length);
  let yBytes = this.ref.point.y.fromRed().toArray("be");
  buf.set(yBytes, 1 + 2 * byteLen - yBytes.length);

  return buf;
};

/**
 * Convert a Uint8Array back to a curve point.
 * Accepts only uncompressed point as specified in section 4.3.6 of ANSI X9.62
 * @param {Uint8Array} bytes
 *
 * @throws {TypeError} when bytes is not Uint8Array
 * @throws {Error} when bytes does not correspond to a valid point
 */
Point.prototype.unmarshalBinary = function(bytes) {
  if (bytes.constructor !== Uint8Array) {
    throw new TypeError("bytes should be a Uint8Array");
  }
  const byteLen = this.ref.curve._coordLen();
  if (bytes.length != 1 + 2 * byteLen) {
    throw new Error();
  }
  // not an uncompressed point
  if (bytes[0] != 4) {
    throw new Error("unmarshalBinary only accepts uncompressed point");
  }
  let x = new BN(bytes.slice(1, 1 + byteLen), 16);
  let y = new BN(bytes.slice(1 + byteLen), 16);
  if (x.cmp(constants.zeroBN) === 0 && y.cmp(constants.zeroBN) === 0) {
    this.ref.point = this.ref.curve.curve.point(null, null);
    return this;
  }
  this.ref.point = this.ref.curve.curve.point(x, y);
  if (!this.ref.curve.curve.validate(this.ref.point)) {
    throw new Error("point is not on curve");
  }
};
