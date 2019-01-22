/// <reference types="node" />
import BN from 'bn.js';
import GfP from './gfp';
declare type BNType = Buffer | string | number | BN;
/**
 * Point class used by G1
 */
export default class CurvePoint {
    static generator: CurvePoint;
    private x;
    private y;
    private z;
    private t;
    constructor(x?: BNType, y?: BNType, z?: BNType, t?: BNType);
    /**
     * Get the x element of the point
     * @returns the x element
     */
    getX(): GfP;
    /**
     * Get the y element of the point
     * @returns the y element
     */
    getY(): GfP;
    /**
     * Check if the point is valid by checking if it is on the curve
     * @returns true when the point is valid, false otherwise
     */
    isOnCurve(): boolean;
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
     * Add a to b and set the value to the point
     * @param a the first point
     * @param b the second point
     */
    add(a: CurvePoint, b: CurvePoint): void;
    /**
     * Compute the double of a and set the value to the point
     * @param a the point to double
     */
    dbl(a: CurvePoint): void;
    /**
     * Multiply a by a scalar
     * @param a      the point to multiply
     * @param scalar the scalar
     */
    mul(a: CurvePoint, scalar: BN): void;
    /**
     * Normalize the point coordinates
     */
    makeAffine(): void;
    /**
     * Compute the negative of a and set the value to the point
     * @param a the point to negate
     */
    negative(a: CurvePoint): void;
    /**
     * Fill the point with the values of a
     * @param p the point to copy
     */
    copy(p: CurvePoint): void;
    /**
     * Get a clone of the current point
     * @returns a clone of the point
     */
    clone(): CurvePoint;
    /**
     * Check the equality between the point and the object
     * @param o the object
     * @returns true when both are equal, false otherwise
     */
    equals(o: any): o is CurvePoint;
    /**
     * Get the string representation of the point
     * @returns the string representation
     */
    toString(): string;
}
export {};
