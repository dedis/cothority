import GfP2 from './gfp2';
import GfP from './gfp';
/**
 * Group field of size p^6
 * This object acts as an immutable and then any modification will instantiate
 * a new object.
 */
export default class GfP6 {
    private static ZERO;
    private static ONE;
    /**
     * Get the addition identity for this group field
     * @returns the element
     */
    static zero(): GfP6;
    /**
     * Get the multiplication identity for this group field
     * @returns the element
     */
    static one(): GfP6;
    private x;
    private y;
    private z;
    constructor(x?: GfP2, y?: GfP2, z?: GfP2);
    /**
     * Get the x value of the group field element
     * @returns the x element
     */
    getX(): GfP2;
    /**
     * Get the y value of the group field element
     * @returns the y element
     */
    getY(): GfP2;
    /**
     * Get the z value of the group field element
     * @returns the z element
     */
    getZ(): GfP2;
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
     * Get the negative of the element
     * @returns the new element
     */
    neg(): GfP6;
    frobenius(): GfP6;
    frobeniusP2(): GfP6;
    /**
     * Add b to the current element
     * @param b the element to add
     * @returns the new element
     */
    add(b: GfP6): GfP6;
    /**
     * Subtract b to the current element
     * @param b the element to subtract
     * @returns the new element
     */
    sub(b: GfP6): GfP6;
    /**
     * Multiply the current element by b
     * @param b the element to multiply with
     * @returns the new element
     */
    mul(b: GfP6): GfP6;
    /**
     * Multiply the current element by a scalar
     * @param b the scalar
     * @returns the new element
     */
    mulScalar(b: GfP2): GfP6;
    /**
     * Multiply the current element by a GFp element
     * @param b the GFp element
     * @returns the new element
     */
    mulGfP(b: GfP): GfP6;
    mulTau(): GfP6;
    /**
     * Get the square of the current element
     * @returns the new element
     */
    square(): GfP6;
    /**
     * Get the inverse of the element
     * @returns the new element
     */
    invert(): GfP6;
    /**
     * Check the equality with the other object
     * @param o the other object
     * @returns true when both are equal, false otherwise
     */
    equals(o: any): o is GfP6;
    /**
     * Get the string representation of the element
     * @returns a string representation
     */
    toString(): string;
}
