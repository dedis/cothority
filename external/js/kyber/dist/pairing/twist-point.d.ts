import BN from 'bn.js';
import GfP2 from './gfp2';
/**
 * Point class used by G2
 */
export default class TwistPoint {
    static generator: TwistPoint;
    private x;
    private y;
    private z;
    private t;
    constructor(x?: GfP2, y?: GfP2, z?: GfP2, t?: GfP2);
    /**
     * Get the x element of the point
     * @returns the x element
     */
    getX(): GfP2;
    /**
     * Get the y element of the point
     * @returns the y element
     */
    getY(): GfP2;
    /**
     * Get the z element of the point
     * @returns the z element
     */
    getZ(): GfP2;
    /**
     * Get the t element of the point
     * @returns the t element
     */
    getT(): GfP2;
    /**
     * Check if the point is on the curve, meaning it's a valid point
     * @returns true for a valid point, false otherwise
     */
    isOnCurve(): boolean;
    /**
     * Set the point to the infinity value
     */
    setInfinity(): void;
    /**
     * Check if the point is the infinity
     * @returns true when the infinity, false otherwise
     */
    isInfinity(): boolean;
    /**
     * Add a to b and set the value to the point
     * @param a first point
     * @param b second point
     */
    add(a: TwistPoint, b: TwistPoint): void;
    /**
     * Compute the double of the given point and set the value
     * @param a the point
     */
    double(a: TwistPoint): void;
    /**
     * Multiply a point by a scalar and set the value to the point
     * @param a the point
     * @param k the scalar
     */
    mul(a: TwistPoint, k: BN): void;
    /**
     * Normalize the point coordinates
     */
    makeAffine(): void;
    /**
     * Compute the negative of a and set the value to the point
     * @param a the point
     */
    neg(a: TwistPoint): void;
    /**
     * Fill the point with the values of a
     * @param a the point
     */
    copy(a: TwistPoint): void;
    /**
     * Get the a clone of the current point
     * @returns a copy of the point
     */
    clone(): TwistPoint;
    /**
     * Check the equality between two points
     * @returns true when both are equal, false otherwise
     */
    equals(o: any): o is TwistPoint;
    /**
     * Get the string representation of the point
     * @returns the string representation
     */
    toString(): string;
}
