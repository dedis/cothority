"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const gfp_1 = __importDefault(require("./gfp"));
const constants_1 = require("./constants");
/**
 * Group field of size p^2
 * This object acts as an immutable and then any modification will instantiate
 * a new object.
 */
class GfP2 {
    constructor(x, y) {
        this.x = x instanceof gfp_1.default ? x : new gfp_1.default(x || 0);
        this.y = y instanceof gfp_1.default ? y : new gfp_1.default(y || 0);
    }
    static zero() {
        return GfP2.ZERO;
    }
    static one() {
        return GfP2.ONE;
    }
    /**
     * Get the x value of this element
     * @returns the x element
     */
    getX() {
        return this.x;
    }
    /**
     * Get the y value of this element
     * @returns the y element
     */
    getY() {
        return this.y;
    }
    /**
     * Check if the value is zero
     * @returns true when zero, false otherwise
     */
    isZero() {
        return this.x.getValue().eqn(0) && this.y.getValue().eqn(0);
    }
    /**
     * Check if the value is one
     * @returns true when one, false otherwise
     */
    isOne() {
        return this.x.getValue().eqn(0) && this.y.getValue().eqn(1);
    }
    /**
     * Get the conjugate of the element
     * @return the conjugate
     */
    conjugate() {
        return new GfP2(this.x.negate(), this.y);
    }
    /**
     * Get the negative of the element
     * @returns the negative
     */
    negative() {
        return new GfP2(this.x.negate(), this.y.negate());
    }
    /**
     * Add a to the current element
     * @param a the other element to add
     * @returns the new element
     */
    add(a) {
        const x = this.x.add(a.x).mod(constants_1.p);
        const y = this.y.add(a.y).mod(constants_1.p);
        return new GfP2(x, y);
    }
    /**
     * Subtract a to the current element
     * @param a the other element to subtract
     * @returns the new element
     */
    sub(a) {
        const x = this.x.sub(a.x).mod(constants_1.p);
        const y = this.y.sub(a.y).mod(constants_1.p);
        return new GfP2(x, y);
    }
    /**
     * Multiply a to the current element
     * @param a the other element to multiply
     * @returns the new element
     */
    mul(a) {
        let tx = this.x.mul(a.y);
        let t = a.x.mul(this.y);
        tx = tx.add(t).mod(constants_1.p);
        let ty = this.y.mul(a.y).mod(constants_1.p);
        t = this.x.mul(a.x).mod(constants_1.p);
        ty = ty.sub(t).mod(constants_1.p);
        return new GfP2(tx, ty);
    }
    /**
     * Multiply the current element by the scalar k
     * @param k the scalar to multiply with
     * @returns the new element
     */
    mulScalar(k) {
        const x = this.x.mul(k);
        const y = this.y.mul(k);
        return new GfP2(x, y);
    }
    /**
     * Set e=ξa where ξ=i+3 and return the new element
     * @returns the new element
     */
    mulXi() {
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
    square() {
        const t1 = this.y.sub(this.x);
        const t2 = this.x.add(this.y);
        const ty = t1.mul(t2).mod(constants_1.p);
        // intermediate modulo is due to a missing implementation
        // in the library that is actually using the unsigned left
        // shift any time
        const tx = this.x.mul(this.y).mod(constants_1.p).shiftLeft(1).mod(constants_1.p);
        return new GfP2(tx, ty);
    }
    /**
     * Get the inverse of the element
     * @returns the new element
     */
    invert() {
        let t = this.y.mul(this.y);
        let t2 = this.x.mul(this.x);
        t = t.add(t2);
        const inv = t.invmod(constants_1.p);
        const tx = this.x.negate().mul(inv).mod(constants_1.p);
        const ty = this.y.mul(inv).mod(constants_1.p);
        return new GfP2(tx, ty);
    }
    /**
     * Check the equality of the elements
     * @param o the object to compare
     * @returns true when both are equal, false otherwise
     */
    equals(o) {
        return this.x.equals(o.x) && this.y.equals(o.y);
    }
    /**
     * Get the string representation of the element
     * @returns the string representation
     */
    toString() {
        return `(${this.x.toHex()},${this.y.toHex()})`;
    }
}
GfP2.ZERO = new GfP2(0, 0);
GfP2.ONE = new GfP2(0, 1);
exports.default = GfP2;
