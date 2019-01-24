"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const gfp6_1 = __importDefault(require("./gfp6"));
const gfp_1 = __importDefault(require("./gfp"));
const constants_1 = require("./constants");
/**
 * Group field element of size p^12
 * This object acts as an immutable and then any modification will instantiate
 * a new object.
 */
class GfP12 {
    constructor(x, y) {
        this.x = x || gfp6_1.default.zero();
        this.y = y || gfp6_1.default.zero();
    }
    /**
     * Get the addition identity for this group field
     * @returns the zero element
     */
    static zero() {
        return GfP12.ZERO;
    }
    /**
     * Get the multiplication identity for this group field
     * @returns the one element
     */
    static one() {
        return GfP12.ONE;
    }
    /**
     * Get the x value of the element
     * @returns the x element
     */
    getX() {
        return this.x;
    }
    /**
     * Get the y value of the element
     * @returns the y element
     */
    getY() {
        return this.y;
    }
    /**
     * Check if the element is zero
     * @returns true when zero, false otherwise
     */
    isZero() {
        return this.x.isZero() && this.y.isZero();
    }
    /**
     * Check if the element is one
     * @returns true when one, false otherwise
     */
    isOne() {
        return this.x.isZero() && this.y.isOne();
    }
    /**
     * Get the conjugate of the element
     * @returns the new element
     */
    conjugate() {
        const x = this.x.neg();
        return new GfP12(x, this.y);
    }
    /**
     * Get the negative of the element
     * @returns the new element
     */
    neg() {
        const x = this.x.neg();
        const y = this.y.neg();
        return new GfP12(x, y);
    }
    frobenius() {
        const x = this.x.frobenius().mulScalar(constants_1.xiToPMinus1Over6);
        const y = this.y.frobenius();
        return new GfP12(x, y);
    }
    frobeniusP2() {
        const x = this.x.frobeniusP2().mulGfP(new gfp_1.default(constants_1.xiToPSquaredMinus1Over6));
        const y = this.y.frobeniusP2();
        return new GfP12(x, y);
    }
    /**
     * Add b to the current element
     * @param b the element to add
     * @returns the new element
     */
    add(b) {
        const x = this.x.add(b.x);
        const y = this.y.add(b.y);
        return new GfP12(x, y);
    }
    /**
     * Subtract b to the current element
     * @param b the element to subtract
     * @returns the new element
     */
    sub(b) {
        const x = this.x.sub(b.x);
        const y = this.y.sub(b.y);
        return new GfP12(x, y);
    }
    /**
     * Multiply b by the current element
     * @param b the element to multiply with
     * @returns the new element
     */
    mul(b) {
        const x = this.x.mul(b.y)
            .add(b.x.mul(this.y));
        const y = this.y.mul(b.y)
            .add(this.x.mul(b.x).mulTau());
        return new GfP12(x, y);
    }
    /**
     * Multiply the current element by a scalar
     * @param k the scalar
     * @returns the new element
     */
    mulScalar(k) {
        const x = this.x.mul(k);
        const y = this.y.mul(k);
        return new GfP12(x, y);
    }
    /**
     * Get the power k of the current element
     * @param k the coefficient
     * @returns the new element
     */
    exp(k) {
        let sum = GfP12.one();
        let t;
        for (let i = k.bitLength() - 1; i >= 0; i--) {
            t = sum.square();
            if (k.testn(i)) {
                sum = t.mul(this);
            }
            else {
                sum = t;
            }
        }
        return sum;
    }
    /**
     * Get the square of the current element
     * @returns the new element
     */
    square() {
        const v0 = this.x.mul(this.y);
        let t = this.x.mulTau();
        t = this.y.add(t);
        let ty = this.x.add(this.y);
        ty = ty.mul(t).sub(v0);
        t = v0.mulTau();
        ty = ty.sub(t);
        return new GfP12(v0.add(v0), ty);
    }
    /**
     * Get the inverse of the current element
     * @returns the new element
     */
    invert() {
        let t1 = this.x.square();
        let t2 = this.y.square();
        t1 = t2.sub(t1.mulTau());
        t2 = t1.invert();
        return new GfP12(this.x.neg(), this.y).mulScalar(t2);
    }
    /**
     * Check the equality with the object
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
        return `(${this.x.toString()}, ${this.y.toString()})`;
    }
}
GfP12.ZERO = new GfP12(gfp6_1.default.zero(), gfp6_1.default.zero());
GfP12.ONE = new GfP12(gfp6_1.default.zero(), gfp6_1.default.one());
exports.default = GfP12;
