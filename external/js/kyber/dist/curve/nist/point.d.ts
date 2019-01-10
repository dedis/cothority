/// <reference types="node" />
import { BNType } from 'bn.js';
import { Point } from "../../index";
import Weierstrass from "./curve";
import NistScalar from "./scalar";
/**
* Represents a Point on the nist curve
*
* The value of the parameters is expected in little endian form if being
* passed as a buffer
*/
export default class NistPoint implements Point {
    ref: {
        curve: Weierstrass;
        point: any;
    };
    constructor(curve: Weierstrass, x?: BNType, y?: BNType);
    string(): string;
    inspect(): string;
    /**
    * Returns the little endian representation of the y coordinate of
    * the Point
    */
    toString(): string;
    /**
    * Tests for equality between two Points derived from the same group
    */
    equal(p2: NistPoint): boolean;
    /**
    * set Set the current point to be equal to p2
    */
    set(p2: NistPoint): NistPoint;
    /**
    * Creates a copy of the current point
    */
    clone(): NistPoint;
    /**
    * Set to the neutral element for the curve
    * Modifies the receiver
    */
    null(): NistPoint;
    /**
    * Set to the standard base point for this curve
    * Modifies the receiver
    */
    base(): NistPoint;
    /**
    * Returns the length (in bytes) of the embedded data
    */
    embedLen(): number;
    /**
    * Returns a Point with data embedded in the y coordinate
    *
    * @throws {Error} if data.length > embedLen
    */
    embed(data: Buffer, callback?: (length: number) => Buffer): NistPoint;
    /**
    * Extract embedded data from a point
    *
    * @throws {Error} when length of embedded data > embedLen
    */
    data(): Buffer;
    /**
    * Returns the sum of two points on the curve
    * Modifies the receiver
    */
    add(p1: NistPoint, p2: NistPoint): NistPoint;
    /**
    * Subtract two points
    * Modifies the receiver
    */
    sub(p1: NistPoint, p2: NistPoint): NistPoint;
    /**
    * Finds the negative of a point p
    * Modifies the receiver
    */
    neg(p: NistPoint): NistPoint;
    /**
    * Multiply point p by scalar s.
    * If p is not passed then multiplies the base point of the curve with
    * scalar s
    * Modifies the receiver
    */
    mul(s: NistScalar, p?: NistPoint): NistPoint;
    /**
    * Selects a random point
    */
    pick(callback?: (length: number) => Buffer): NistPoint;
    marshalSize(): number;
    /**
    * converts a point into the form specified in section 4.3.6 of ANSI X9.62.
    */
    marshalBinary(): Buffer;
    /**
    * Convert a buffer back to a curve point.
    * Accepts only uncompressed point as specified in section 4.3.6 of ANSI X9.62
    * @throws {Error} when bytes does not correspond to a valid point
    */
    unmarshalBinary(bytes: Buffer): void;
}
