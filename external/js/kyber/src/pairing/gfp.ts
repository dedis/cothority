import BN from 'bn.js';
import { BNType } from '../constants';

/**
 * Field of size p
 * This object acts as an immutable and then any modification will instantiate
 * a new object.
 */
export default class GfP {
    private static ELEM_SIZE = 256 / 8;

    private v: BN;

    constructor(value: BNType) {
        this.v = new BN(value);
    }

    /**
     * Get the BigNumber value
     * @returns the BN object
     */
    getValue(): BN {
        return this.v;
    }

    /**
     * Compare the sign of the number
     * @returns -1 for a negative, 1 for a positive and 0 for zero
     */
    signum(): -1 | 0 | 1 {
        return this.v.cmpn(0);
    }

    /**
     * Check if the number is one
     * @returns true for one, false otherwise
     */
    isOne(): boolean {
        return this.v.eq(new BN(1));
    }

    /**
     * Check if the number is zero
     * @returns true for zero, false otherwise
     */
    isZero(): boolean {
        return this.v.isZero();
    }

    /**
     * Add the value of a to the current value
     * @param a the value to add
     * @returns the new value
     */
    add(a: GfP): GfP {
        return new GfP(this.v.add(a.v));
    }

    /**
     * Subtract the value of a to the current value
     * @param a the value to subtract
     * @return the new value
     */
    sub(a: GfP): GfP {
        return new GfP(this.v.sub(a.v));
    }

    /**
     * Multiply the current value by a
     * @param a the value to multiply
     * @returns the new value
     */
    mul(a: GfP): GfP {
        return new GfP(this.v.mul(a.v));
    }

    /**
     * Get the square of the current value
     * @returns the new value
     */
    sqr(): GfP {
        return new GfP(this.v.sqr());
    }

    /**
     * Get the power k of the current value
     * @param k the coefficient
     * @returns the new value
     */
    pow(k: BN) {
        return new GfP(this.v.pow(k));
    }

    /**
     * Get the unsigned modulo p of the current value
     * @param p the modulus
     * @returns the new value
     */
    mod(p: BN): GfP {
        return new GfP(this.v.umod(p));
    }

    /**
     * Get the modular inverse of the current value
     * @param p the modulus
     * @returns the new value
     */
    invmod(p: BN): GfP {
        return new GfP(this.v.invm(p));
    }

    /**
     * Get the negative of the current value
     * @returns the new value
     */
    negate(): GfP {
        return new GfP(this.v.neg());
    }

    /**
     * Left shift by k bits of the current value
     * @param k number of positions to switch
     * @returns the new value
     */
    shiftLeft(k: number): GfP {
        return new GfP(this.v.shln(k));
    }

    /**
     * Compare the current value with a
     * @param o the value to compare
     * @returns -1 when o is greater, 1 when smaller and 0 when equal
     */
    compareTo(o: any): 0 | -1 | 1 {
        return this.v.cmp(o.v);
    }

    /**
     * Check the equality with o
     * @param o the object to compare
     * @returns true when equal, false otherwise
     */
    equals(o: any): o is GfP {
        return this.v.eq(o.v);
    }

    /**
     * Convert the group field element into a buffer in big-endian
     * and a fixed size.
     * @returns the buffer
     */
    toBytes(): Buffer {
        return this.v.toArrayLike(Buffer, 'be', GfP.ELEM_SIZE);
    }

    /**
     * Get the hexadecimal representation of the element
     * @returns the hex string
     */
    toString(): string {
        return this.toHex();
    }

    /**
     * Get the hexadecimal shape of the element without leading zeros
     * @returns the hex shape in a string
     */
    toHex(): string {
        return this.v.toString('hex');
    }
}
