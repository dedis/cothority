import * as curve from "./curve";
import * as sign from "./sign";
import PointFactory from './point-factory';

export interface Group {
  /**
   * Get the length of the buffer after marshaling the scalar
   * @returns the length
   */
  scalarLen(): number;

  /**
   * Make a scalar compatible with this group
   * @returns the new scalar
   */
  scalar(): Scalar;

  /**
   * Get the length of the buffer after marshaling the point
   * @returns the length
   */
  pointLen(): number;

  /**
   * Make a point compatible with this group
   * @returns the new point
   */
  point(): Point;
}

export interface Point {
  /**
   * Make a point set to the neutral element
   * @returns the new point
   */
  null(): Point;

  /**
   * Make a point set to the standard base for the curve
   * @returns the new point
   */
  base(): Point;

  /**
   * Make a random point
   * @param callback  buffer generator function
   * @returns the new point
   */
  pick(callback?: (length: number) => Buffer): Point;

  /**
   * Use the given point to set the current one
   * @param p the point to use
   * @returns the point
   */
  set(p: Point): Point;

  /**
   * Make a clone of the current point
   * @returns the cloned point
   */
  clone(): Point;

  /**
   * Get the size of the buffer after embedding
   * @returns the length
   */
  embedLen(): number;

  /**
   * Get a Point with data embedded in the y coordinate
   * @param data
   * @param callback  buffer generator function
   * @returns         the new point
   */
  embed(data: Buffer, callback?: (length: number) => Buffer): Point;

  /**
   * Extract embedded data from a point
   * @returns the buffer
   */
  data(): Buffer;

  /**
   * Get the sum of two points
   * @param p1  the first point
   * @param p2  the second point
   * @returns   the new point resulting from the sum
   */
  add(p1: Point, p2: Point): Point;

  /**
   * Subtract two points
   * @param p1  the first point
   * @param p2  the second point
   * @returns   the new point resulting from the subtraction
   */
  sub(p1: Point, p2: Point): Point;

  /**
   * Get the negative of a point
   * @param p the point
   * @returns the negative point
   */
  neg(p: Point): Point;

  /**
   * Multiply the point by a scalar
   * @param s the scalar
   * @param p the point
   * @returns the point resulting from the multiplication
   */
  mul(s: Scalar, p?: Point): Point;

  /**
   * Converts a point into the form specified in section 4.3.6 of ANSI X9.62.
   * @returns the buffer
   */
  marshalBinary(): Buffer;

  /**
   * Convert a buffer back to a curve point.
   * Accepts only uncompressed point as specified in section 4.3.6 of ANSI X9.62.
   * Don't use this to send the point through the network but toProto instead.
   */
  unmarshalBinary(bytes: Buffer): void;

  /**
   * Get the length of the buffer after marshaling the point
   * @returns the length as a number
   */
  marshalSize(): number;

  /**
   * Check if the given point is the same
   * @param p2  the point to compare
   * @returns   true when both are equal
   */
  equals(p2: Point): boolean;

  /**
   * Get a string representation of the point
   * @returns the string representation
   */
  toString(): string;

  /**
   * Encode the point to be passed through a protobuf channel. Use this
   * instead of marshalBinary to send the point over the network.
   */
  toProto(): Buffer;
}

export interface Scalar {
  /**
   * Returns the binary representation (big endian) of the scalar
   * @returns the buffer
   */
  marshalBinary(): Buffer;

  /**
   * Reads the binary representation (big endian) of scalar
   * @param bytes the buffer
   */
  unmarshalBinary(bytes: Buffer): void;

  /**
   * Get the length of the buffer after marshaling the scalar
   * @returns the length as a number
   */
  marshalSize(): number;

  /**
   * Sets the receiver equal to another Scalar a
   * @param a the new scalar
   * @return the current scalar set to the new value
   */
  set(a: Scalar): Scalar;

  /**
   * Get a copy of the scalar
   * @returns a new clone of the scalar
   */
  clone(): Scalar;

  /**
   * Set to the additive identity (0)
   * @returns the scalar
   */
  zero(): Scalar;

  /**
   * Get the modular sum of the two scalars
   * @param a the first scalar
   * @param b the second scalar
   * @returns the new scalar resulting from the sum
   */
  add(a: Scalar, b: Scalar): Scalar;

  /**
   * Get the modular difference of the two scalars
   * @param a the first scalar
   * @param b the second scalar
   * @returns the new scalar resulting from the subtraction
   */
  sub(a: Scalar, b: Scalar): Scalar;

  /**
   * Set to the modular negation of scalar a
   * @param a the reference scalar
   * @returns the negative scalar
   */
  neg(a: Scalar): Scalar;

  /**
   * Set to the modular division of scalar s1 by scalar s2
   * @param a the dividend
   * @param b the quotient
   * @returns the new scalar resulting from the division
   */
  div(a: Scalar, b: Scalar): Scalar;

  /**
   * Get the modular multiplication of two scalars
   * @param a the first scalar
   * @param b the second scalar
   * @returns the new scalar resulting from the multiplication
   */
  mul(s1: Scalar, b: Scalar): Scalar;

  /**
   * Get the modular inverse of a scalar
   * @param a the scalar
   * @returns the new scalar resulting from the inversion
   */
  inv(a: Scalar): Scalar;

  /**
   * Get the multiplication identity
   * @returns the scalar
   */
  one(): Scalar;

  /**
   * Get a random scalar
   * @param callback  the buffer generator function
   * @returns         a random scalar
   */
  pick(callback?: (length: number) => Buffer): Scalar;

  /**
   * Populate the scalar using the big-endian buffer
   * @returns the scalar
   */
  setBytes(bytes: Buffer): Scalar;

  /**
   * Equality test for two Scalars derived from the same Group
   * @param s2  the scalar to test against
   * @returns   true when both are equal
   */
  equals(s2: Scalar): boolean;
}

export {
  curve,
  sign,
  PointFactory,
}

export default {
  curve,
  sign,
  PointFactory,
}
