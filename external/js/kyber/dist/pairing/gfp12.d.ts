import BN from 'bn.js';
import GfP6 from './gfp6';
/**
 * Group field element of size p^12
 * This object acts as an immutable and then any modification will instantiate
 * a new object.
 */
export default class GfP12 {
    private static ZERO;
    private static ONE;
    /**
     * Get the addition identity for this group field
     * @returns the zero element
     */
    static zero(): GfP12;
    /**
     * Get the multiplication identity for this group field
     * @returns the one element
     */
    static one(): GfP12;
    private x;
    private y;
    constructor(x?: GfP6, y?: GfP6);
    /**
     * Get the x value of the element
     * @returns the x element
     */
    getX(): GfP6;
    /**
     * Get the y value of the element
     * @returns the y element
     */
    getY(): GfP6;
    /**
     * Check if the element is zero
     * @returns true when zero, false otherwise
     */
    isZero(): boolean;
    /**
     * Check if the element is one
     * @returns true when one, false otherwise
     */
    isOne(): boolean;
    /**
     * Get the conjugate of the element
     * @returns the new element
     */
    conjugate(): GfP12;
    /**
     * Get the negative of the element
     * @returns the new element
     */
    neg(): GfP12;
    frobenius(): GfP12;
    frobeniusP2(): GfP12;
    /**
     * Add b to the current element
     * @param b the element to add
     * @returns the new element
     */
    add(b: GfP12): GfP12;
    /**
     * Subtract b to the current element
     * @param b the element to subtract
     * @returns the new element
     */
    sub(b: GfP12): GfP12;
    /**
     * Multiply b by the current element
     * @param b the element to multiply with
     * @returns the new element
     */
    mul(b: GfP12): GfP12;
    /**
     * Multiply the current element by a scalar
     * @param k the scalar
     * @returns the new element
     */
    mulScalar(k: GfP6): GfP12;
    /**
     * Get the power k of the current element
     * @param k the coefficient
     * @returns the new element
     */
    exp(k: BN): GfP12;
    /**
     * Get the square of the current element
     * @returns the new element
     */
    square(): GfP12;
    /**
     * Get the inverse of the current element
     * @returns the new element
     */
    invert(): GfP12;
    /**
     * Check the equality with the object
     * @param o the object to compare
     * @returns true when both are equal, false otherwise
     */
    equals(o: any): o is GfP12;
    /**
     * Get the string representation of the element
     * @returns the string representation
     */
    toString(): string;
}
