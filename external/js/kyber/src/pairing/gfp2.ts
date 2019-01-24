import BN from 'bn.js';
import GfP from './gfp';
import { p } from './constants';

type BNType = Buffer | string | number | BN;

/**
 * Group field of size p^2
 * This object acts as an immutable and then any modification will instantiate
 * a new object.
 */
export default class GfP2 {
    private static ZERO = new GfP2(0, 0);
    private static ONE = new GfP2(0, 1);

    public static zero(): GfP2 {
        return GfP2.ZERO;
    }

    public static one(): GfP2 {
        return GfP2.ONE;
    }

    private x: GfP;
    private y: GfP;

    constructor(x: BNType | GfP, y: BNType | GfP) {
        this.x = x instanceof GfP ? x : new GfP(x || 0);
        this.y = y instanceof GfP ? y : new GfP(y || 0);
    }

    /**
     * Get the x value of this element
     * @returns the x element
     */
    getX(): GfP {
        return this.x;
    }

    /**
     * Get the y value of this element
     * @returns the y element
     */
    getY(): GfP {
        return this.y;
    }

    /**
     * Check if the value is zero
     * @returns true when zero, false otherwise
     */
    isZero(): boolean {
        return this.x.getValue().eqn(0) && this.y.getValue().eqn(0);
    }

    /**
     * Check if the value is one
     * @returns true when one, false otherwise
     */
    isOne(): boolean {
        return this.x.getValue().eqn(0) && this.y.getValue().eqn(1);
    }

    /**
     * Get the conjugate of the element
     * @return the conjugate
     */
    conjugate(): GfP2 {
        return new GfP2(this.x.negate(), this.y);
    }

    /**
     * Get the negative of the element
     * @returns the negative
     */
    negative(): GfP2 {
        return new GfP2(this.x.negate(), this.y.negate());
    }

    /**
     * Add a to the current element
     * @param a the other element to add
     * @returns the new element
     */
    add(a: GfP2): GfP2 {
        const x = this.x.add(a.x).mod(p);
        const y = this.y.add(a.y).mod(p);
        return new GfP2(x, y);
    }

    /**
     * Subtract a to the current element
     * @param a the other element to subtract
     * @returns the new element
     */
    sub(a: GfP2): GfP2 {
        const x = this.x.sub(a.x).mod(p);
        const y = this.y.sub(a.y).mod(p);
        return new GfP2(x, y);
    }

    /**
     * Multiply a to the current element
     * @param a the other element to multiply
     * @returns the new element
     */
    mul(a: GfP2): GfP2 {
        let tx = this.x.mul(a.y);
        let t = a.x.mul(this.y);
        tx = tx.add(t).mod(p);

        let ty = this.y.mul(a.y).mod(p);
        t = this.x.mul(a.x).mod(p);
        ty = ty.sub(t).mod(p);

        return new GfP2(tx, ty);
    }

    /**
     * Multiply the current element by the scalar k
     * @param k the scalar to multiply with
     * @returns the new element
     */
    mulScalar(k: GfP): GfP2 {
        const x = this.x.mul(k);
        const y = this.y.mul(k);

        return new GfP2(x, y);
    }

    /**
     * Set e=ξa where ξ=i+3 and return the new element
     * @returns the new element
     */
    mulXi(): GfP2 {
        let tx = this.x.add(this.x);
        tx = tx.add(this.x);
        tx = tx.add(this.y);

        let ty = this.y.add(this.y);
        ty = ty.add(this.y);
        ty = ty.sub(this.x);

        return new GfP2(tx, ty);
    }

    /**
     * Get the square value of the element
     * @returns the new element
     */
    square(): GfP2 {
        const t1 = this.y.sub(this.x);
        const t2 = this.x.add(this.y);

        const ty = t1.mul(t2).mod(p);
        // intermediate modulo is due to a missing implementation
        // in the library that is actually using the unsigned left
        // shift any time
        const tx = this.x.mul(this.y).mod(p).shiftLeft(1).mod(p);

        return new GfP2(tx, ty);
    }

    /**
     * Get the inverse of the element
     * @returns the new element
     */
    invert(): GfP2 {
        let t = this.y.mul(this.y);
        let t2 = this.x.mul(this.x);
        t = t.add(t2);

        const inv = t.invmod(p);
        const tx = this.x.negate().mul(inv).mod(p);
        const ty = this.y.mul(inv).mod(p);

        return new GfP2(tx, ty);
    }

    /**
     * Check the equality of the elements
     * @param o the object to compare
     * @returns true when both are equal, false otherwise
     */
    equals(o: any): o is GfP2 {
        return this.x.equals(o.x) && this.y.equals(o.y);
    }

    /**
     * Get the string representation of the element
     * @returns the string representation
     */
    toString(): string {
        return `(${this.x.toHex()},${this.y.toHex()})`;
    }
}
