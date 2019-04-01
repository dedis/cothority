import { BNType } from '../constants';

/**
 * Field of size p
 * This object acts as an immutable and then any modification will instantiate
 * a new object.
 */
export default class GfP {
    private static ELEM_SIZE = 256 / 8;

    private v: bigint;

    constructor(value: BNType | bigint) {
        this.v = BigInt(value);
    }

    /**
     * Get the BigNumber value
     * @returns the BN object
     */
    getValue(): bigint {
        return this.v;
    }

    /**
     * Compare the sign of the number
     * @returns -1 for a negative, 1 for a positive and 0 for zero
     */
    signum(): -1 | 0 | 1 {
        return this.v < 0 ? -1 : (this.v > 0 ? 1 : 0);
    }

    /**
     * Check if the number is one
     * @returns true for one, false otherwise
     */
    isOne(): boolean {
        return this.v === 1n;
    }

    /**
     * Check if the number is zero
     * @returns true for zero, false otherwise
     */
    isZero(): boolean {
        return this.v === 0n;
    }

    /**
     * Add the value of a to the current value
     * @param a the value to add
     * @returns the new value
     */
    add(a: GfP): GfP {
        return new GfP(this.v + a.v);
    }

    /**
     * Subtract the value of a to the current value
     * @param a the value to subtract
     * @return the new value
     */
    sub(a: GfP): GfP {
        return new GfP(this.v - a.v);
    }

    /**
     * Multiply the current value by a
     * @param a the value to multiply
     * @returns the new value
     */
    mul(a: GfP): GfP {
        return new GfP(this.v * a.v);
    }

    /**
     * Get the square of the current value
     * @returns the new value
     */
    sqr(): GfP {
        return new GfP(this.v ** 2n);
    }

    /**
     * Get the power k of the current value
     * @param k the coefficient
     * @returns the new value
     */
    pow(k: bigint) {
        return new GfP(this.v ** k);
    }

    /**
     * Get the unsigned modulo p of the current value
     * @param p the modulus
     * @returns the new value
     */
    mod(p: bigint): GfP {
        let t = this.v % p;
        if (t < 0n) {
            t += p
        }

        return new GfP(t);
    }

    /**
     * Get the modular inverse of the current value
     * @param p the modulus
     * @returns the new value
     */
    invmod(p: bigint): GfP {
        let b0 = p;
        let t, q: bigint;
        let x0 = 0n;
        let x1 = 1n;
        let a = this.v;
        while (a > 1n) {
            q = a / p;
            t = p;
            p = a % p;
            a = t;
            t = x0;
            x0 = x1 - q * x0;
            x1 = t;
        }

        if (x1 < 0n) {
            x1 += b0;
        }

        return new GfP(x1);
    }

    /**
     * Get the negative of the current value
     * @returns the new value
     */
    negate(): GfP {
        return new GfP(-this.v);
    }

    /**
     * Left shift by k bits of the current value
     * @param k number of positions to switch
     * @returns the new value
     */
    shiftLeft(k: bigint): GfP {
        return new GfP(this.v << k);
    }

    /**
     * Compare the current value with a
     * @param o the value to compare
     * @returns -1 when o is greater, 1 when smaller and 0 when equal
     */
    compareTo(o: GfP): 0 | -1 | 1 {
        if (this.v === o.v) {
            return 0;
        } else if (this.v > o.v) {
            return 1;
        }

        return -1;
    }

    /**
     * Check the equality with o
     * @param o the object to compare
     * @returns true when equal, false otherwise
     */
    equals(o: any): o is GfP {
        return this.v === o.v;
    }

    /**
     * Convert the group field element into a buffer in big-endian
     * and a fixed size.
     * @returns the buffer
     */
    toBytes(): Buffer {
        return Buffer.from(this.v.toString(16), 'hex');
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
        return this.v.toString(16);
    }
}
