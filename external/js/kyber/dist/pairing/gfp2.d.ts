/// <reference types="node" />
import BN from 'bn.js';
import GfP from './gfp';
declare type BNType = Buffer | string | number | BN;
/**
 * Group field of size p^2
 * This object acts as an immutable and then any modification will instantiate
 * a new object.
 */
export default class GfP2 {
    private static ZERO;
    private static ONE;
    static zero(): GfP2;
    static one(): GfP2;
    private x;
    private y;
    constructor(x: BNType | GfP, y: BNType | GfP);
    /**
     * Get the x value of this element
     * @returns the x element
     */
    getX(): GfP;
    /**
     * Get the y value of this element
     * @returns the y element
     */
    getY(): GfP;
    /**
     * Check if the value is zero
     * @returns true when zero, false otherwise
     */
    isZero(): boolean;
    /**
     * Check if the value is one
     * @returns true when one, false otherwise
     */
    isOne(): boolean;
    /**
     * Get the conjugate of the element
     * @return the conjugate
     */
    conjugate(): GfP2;
    /**
     * Get the negative of the element
     * @returns the negative
     */
    negative(): GfP2;
    /**
     * Add a to the current element
     * @param a the other element to add
     * @returns the new element
     */
    add(a: GfP2): GfP2;
    /**
     * Subtract a to the current element
     * @param a the other element to subtract
     * @returns the new element
     */
    sub(a: GfP2): GfP2;
    /**
     * Multiply a to the current element
     * @param a the other element to multiply
     * @returns the new element
     */
    mul(a: GfP2): GfP2;
    /**
     * Multiply the current element by the scalar k
     * @param k the scalar to multiply with
     * @returns the new element
     */
    mulScalar(k: GfP): GfP2;
    /**
     * Set e=ξa where ξ=i+3 and return the new element
     * @returns the new element
     */
    mulXi(): GfP2;
    /**
     * Get the square value of the element
     * @returns the new element
     */
    square(): GfP2;
    /**
     * Get the inverse of the element
     * @returns the new element
     */
    invert(): GfP2;
    /**
     * Check the equality of the elements
     * @param o the object to compare
     * @returns true when both are equal, false otherwise
     */
    equals(o: any): o is GfP2;
    /**
     * Get the string representation of the element
     * @returns the string representation
     */
    toString(): string;
}
export {};
