/// <reference types="node" />
import BN from 'bn.js';
import CurvePoint from './curve-point';
import TwistPoint from './twist-point';
import GfP12 from './gfp12';
export declare type BNType = number | string | number[] | Buffer | BN;
/**
 * Wrapper around the basic curve point. It acts as a mutable object and
 * then every modification is done in-place.
 */
export declare class G1 {
    private static ELEM_SIZE;
    private static MARSHAL_SIZE;
    private p;
    constructor(k?: BNType);
    /**
     * Get the curve point
     * @returns the point
     */
    getPoint(): CurvePoint;
    /**
     * Set the point to the generator of the curve
     */
    setBase(): void;
    /**
     * Set the point to infinity
     */
    setInfinity(): void;
    /**
     * Check if the point is the infinity
     * @returns true when infinity, false otherwise
     */
    isInfinity(): boolean;
    /**
     * Multiply the generator by the scalar k and set the value
     * @param k the scalar
     */
    scalarBaseMul(k: BN): void;
    /**
     * Multiply a by the scalar k and set the value
     * @param a the point
     * @param k the scalar
     */
    scalarMul(a: G1, k: BN): void;
    /**
     * Add a to b and set the value
     * @param a the first point
     * @param b the second point
     */
    add(a: G1, b: G1): void;
    /**
     * Compute the negative of a and set the value
     * @param the point to negate
     */
    neg(a: G1): void;
    /**
     * Get the buffer size after marshaling
     * @returns the length
     */
    marshalSize(): number;
    /**
     * Serialize the point into bytes
     * @returns the buffer
     */
    marshal(): Buffer;
    /**
     * Take a buffer to deserialize a point
     * @param bytes the buffer
     */
    unmarshal(bytes: Buffer): void;
    /**
     * Check the equality between the point and the object
     * @param o the object
     * @returns true when both are equal, false otherwise
     */
    equals(o: any): o is G1;
    /**
     * Get a clone of the element
     * @returns the new element
     */
    clone(): G1;
    /**
     * Get the string representation of the point
     * @returns the string representation
     */
    toString(): string;
}
/**
 * Wrapper around the twist point. It acts as a mutable object and
 * then every modification is done in-place.
 */
export declare class G2 {
    p: TwistPoint;
    private static ELEM_SIZE;
    private static MARSHAL_SIZE;
    constructor(k?: BNType);
    /**
     * Get the twist point
     * @returns the point
     */
    getPoint(): TwistPoint;
    /**
     * Set to the generator of the curve
     */
    setBase(): void;
    /**
     * Set the point to the infinity
     */
    setInfinity(): void;
    /**
     * Check if the point is the infinity
     * @returns true when infinity, false otherwise
     */
    isInfinity(): boolean;
    /**
     * Mutliply the generator by a scalar k and set the value
     * @param k the scalar
     */
    scalarBaseMul(k?: BN): void;
    /**
     * Multiply a by a scalar k and set the value
     * @param a the point
     * @param k the scalar
     */
    scalarMul(a: G2, k: BN): void;
    /**
     * Add a to b and set the value
     * @param a the first point
     * @param b the second point
     */
    add(a: G2, b: G2): void;
    /**
     * Compute the negative of a and set the value
     * @param a the point
     */
    neg(a: G2): void;
    /**
     * Get the size of the buffer after marshaling
     * @returns the size
     */
    marshalSize(): number;
    /**
     * Serialize the point into bytes
     * @returns the buffer
     */
    marshal(): Buffer;
    /**
     * Take a buffer and deserialize a point
     * @param bytes the buffer
     */
    unmarshal(bytes: Buffer): void;
    /**
     * Get a clone of G2
     * @returns the clone
     */
    clone(): G2;
    /**
     * Check the equality of the current point and the object
     * @param o the object
     * @returns true when both are equal, false otherwise
     */
    equals(o: any): o is G2;
    /**
     * Get the string representation of the point
     * @returns the string representation
     */
    toString(): string;
}
/**
 * Wrapper around the result of pairing of G1 and G2. It acts as a mutable
 * object and then every modification is done in-place.
 */
export declare class GT {
    private static ELEM_SIZE;
    private static MARSHAL_SIZE;
    static pair(g1: G1, g2: G2): GT;
    static one(): GT;
    private g;
    constructor(g?: GfP12);
    /**
     * Check if the point is one
     * @returns true when one, false otherwise
     */
    isOne(): boolean;
    /**
     * Multiply the point a by a scalar k and set the value
     * @param a the point
     * @param k the scalar
     */
    scalarMul(a: GT, k: BN): void;
    /**
     * Add two points a and b and set the value
     * @param a the first point
     * @param b the second point
     */
    add(a: GT, b: GT): void;
    /**
     * Compute the negative of a and set the value
     * @param a the point
     */
    neg(a: GT): void;
    /**
     * Serialize the point into bytes
     * @returns the buffer
     */
    marshal(): Buffer;
    /**
     * Take a buffer and deserialize a point
     * @param bytes the buffer
     */
    unmarshal(bytes: Buffer): void;
    /**
     * Check the equality of the point and an object
     * @param o the object
     * @returns true when both are equal, false otherwise
     */
    equals(o: any): o is GT;
    /**
     * Get the string representation of the point
     * @returns the string representation
     */
    toString(): string;
}
