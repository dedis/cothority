/**
 * Group is an abstract class for curves
 */
class Group {
  constructor() {}

  scalarLen() {
    throw new Error("Not implemented");
  }

  scalar() {
    throw new Error("Not implemented");
  }

  pointLen() {
    throw new Error("Not implemented");
  }

  point() {
    throw new Error("Not implemented");
  }
}

/**
 * Point is an abstract class for representing
 * a point on an elliptic curve
 */
class Point {
  constructor() {}

  equal() {
    throw new Error("Not implemented");
  }

  null() {
    throw new Error("Not implemented");
  }

  base() {
    throw new Error("Not implemented");
  }

  pick(callback) {
    throw new Error("Not implemented");
  }

  set() {
    throw new Error("Not implemented");
  }

  clone() {
    throw new Error("Not implemented");
  }

  embedLen() {
    throw new Error("Not implemented");
  }

  embed(data, callback) {
    throw new Error("Not implemented");
  }

  data() {
    throw new Error("Not implemented");
  }

  add(p1, p2) {
    throw new Error("Not implemented");
  }

  sub(p1, p2) {
    throw new Error("Not implemented");
  }

  neg(p) {
    throw new Error("Not implemented");
  }

  mul(s, p) {
    throw new Error("Not implemented");
  }

  marshalBinary() {
    throw new Error("Not implemented");
  }

  unmarshalBinary(bytes) {
    throw new Error("Not implemented");
  }
}

/**
 * Scalar is an abstract class for representing a scalar
 * to be used in elliptic curve operations
 */
class Scalar {
  marshalBinary() {
    throw new Error("Not implemented");
  }

  unmarshalBinary(bytes) {
    throw new Error("Not implemented");
  }

  equal() {
    throw new Error("Not implemented");
  }

  set(a) {
    throw new Error("Not implemented");
  }

  clone() {
    throw new Error("Not implemented");
  }

  zero() {
    throw new Error("Not implemented");
  }

  add(a, b) {
    throw new Error("Not implemented");
  }

  sub(a, b) {
    throw new Error("Not implemented");
  }

  neg(a) {
    throw new Error("Not implemented");
  }

  div(a, b) {
    throw new Error("Not implemented");
  }

  inv(a) {
    throw new Error("Not implemented");
  }

  inv(a) {
    throw new Error("Not implemented");
  }

  pick(callback) {
    throw new Error("Not implemented");
  }

  setBytes(bytes) {
    throw new Error("Not implemented");
  }
}

module.exports = {
  Point,
  Scalar,
  Group
};
