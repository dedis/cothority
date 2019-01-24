"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const bn_js_1 = __importDefault(require("bn.js"));
/**
 * Field of size p
 * This object acts as an immutable and then any modification will instantiate
 * a new object.
 */
class GfP {
    constructor(value) {
        this.v = new bn_js_1.default(value);
    }
    /**
     * Get the BigNumber value
     * @returns the BN object
     */
    getValue() {
        return this.v;
    }
    /**
     * Compare the sign of the number
     * @returns -1 for a negative, 1 for a positive and 0 for zero
     */
    signum() {
        return this.v.cmpn(0);
    }
    /**
     * Check if the number is one
     * @returns true for one, false otherwise
     */
    isOne() {
        return this.v.eq(new bn_js_1.default(1));
    }
    /**
     * Check if the number is zero
     * @returns true for zero, false otherwise
     */
    isZero() {
        return this.v.isZero();
    }
    /**
     * Add the value of a to the current value
     * @param a the value to add
     * @returns the new value
     */
    add(a) {
        return new GfP(this.v.add(a.v));
    }
    /**
     * Subtract the value of a to the current value
     * @param a the value to subtract
     * @return the new value
     */
    sub(a) {
        return new GfP(this.v.sub(a.v));
    }
    /**
     * Multiply the current value by a
     * @param a the value to multiply
     * @returns the new value
     */
    mul(a) {
        return new GfP(this.v.mul(a.v));
    }
    /**
     * Get the square of the current value
     * @returns the new value
     */
    sqr() {
        return new GfP(this.v.sqr());
    }
    /**
     * Get the power k of the current value
     * @param k the coefficient
     * @returns the new value
     */
    pow(k) {
        return new GfP(this.v.pow(k));
    }
    /**
     * Get the unsigned modulo p of the current value
     * @param p the modulus
     * @returns the new value
     */
    mod(p) {
        return new GfP(this.v.umod(p));
    }
    /**
     * Get the modular inverse of the current value
     * @param p the modulus
     * @returns the new value
     */
    invmod(p) {
        return new GfP(this.v.invm(p));
    }
    /**
     * Get the negative of the current value
     * @returns the new value
     */
    negate() {
        return new GfP(this.v.neg());
    }
    /**
     * Left shift by k bits of the current value
     * @param k number of positions to switch
     * @returns the new value
     */
    shiftLeft(k) {
        return new GfP(this.v.shln(k));
    }
    /**
     * Compare the current value with a
     * @param o the value to compare
     * @returns -1 when o is greater, 1 when smaller and 0 when equal
     */
    compareTo(o) {
        return this.v.cmp(o.v);
    }
    /**
     * Check the equality with o
     * @param o the object to compare
     * @returns true when equal, false otherwise
     */
    equals(o) {
        return this.v.eq(o.v);
    }
    /**
     * Convert the group field element into a buffer in big-endian
     * and a fixed size.
     * @returns the buffer
     */
    toBytes() {
        return this.v.toArrayLike(Buffer, 'be', GfP.ELEM_SIZE);
    }
    /**
     * Get the hexadecimal representation of the element
     * @returns the hex string
     */
    toString() {
        return this.toHex();
    }
    /**
     * Get the hexadecimal shape of the element without leading zeros
     * @returns the hex shape in a string
     */
    toHex() {
        return this.v.toString('hex');
    }
}
GfP.ELEM_SIZE = 256 / 8;
exports.default = GfP;
