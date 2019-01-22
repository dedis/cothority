"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const gfp2_1 = __importDefault(require("./gfp2"));
const constants_1 = require("./constants");
const gfp_1 = __importDefault(require("./gfp"));
/**
 * Group field of size p^6
 * This object acts as an immutable and then any modification will instantiate
 * a new object.
 */
class GfP6 {
    constructor(x, y, z) {
        this.x = x || gfp2_1.default.zero();
        this.y = y || gfp2_1.default.zero();
        this.z = z || gfp2_1.default.zero();
    }
    /**
     * Get the addition identity for this group field
     * @returns the element
     */
    static zero() {
        return GfP6.ZERO;
    }
    /**
     * Get the multiplication identity for this group field
     * @returns the element
     */
    static one() {
        return GfP6.ONE;
    }
    /**
     * Get the x value of the group field element
     * @returns the x element
     */
    getX() {
        return this.x;
    }
    /**
     * Get the y value of the group field element
     * @returns the y element
     */
    getY() {
        return this.y;
    }
    /**
     * Get the z value of the group field element
     * @returns the z element
     */
    getZ() {
        return this.z;
    }
    /**
     * Check if the element is zero
     * @returns true when zero, false otherwise
     */
    isZero() {
        return this.x.isZero() && this.y.isZero() && this.z.isZero();
    }
    /**
     * Check if the element is one
     * @returns true when one, false otherwise
     */
    isOne() {
        return this.x.isZero() && this.y.isZero() && this.z.isOne();
    }
    /**
     * Get the negative of the element
     * @returns the new element
     */
    neg() {
        const x = this.x.negative();
        const y = this.y.negative();
        const z = this.z.negative();
        return new GfP6(x, y, z);
    }
    frobenius() {
        const x = this.x.conjugate().mul(constants_1.xiTo2PMinus2Over3);
        const y = this.y.conjugate().mul(constants_1.xiToPMinus1Over3);
        const z = this.z.conjugate();
        return new GfP6(x, y, z);
    }
    frobeniusP2() {
        const x = this.x.mulScalar(new gfp_1.default(constants_1.xiTo2PSquaredMinus2Over3));
        const y = this.y.mulScalar(new gfp_1.default(constants_1.xiToPSquaredMinus1Over3));
        return new GfP6(x, y, this.z);
    }
    /**
     * Add b to the current element
     * @param b the element to add
     * @returns the new element
     */
    add(b) {
        const x = this.x.add(b.x);
        const y = this.y.add(b.y);
        const z = this.z.add(b.z);
        return new GfP6(x, y, z);
    }
    /**
     * Subtract b to the current element
     * @param b the element to subtract
     * @returns the new element
     */
    sub(b) {
        const x = this.x.sub(b.x);
        const y = this.y.sub(b.y);
        const z = this.z.sub(b.z);
        return new GfP6(x, y, z);
    }
    /**
     * Multiply the current element by b
     * @param b the element to multiply with
     * @returns the new element
     */
    mul(b) {
        const v0 = this.z.mul(b.z);
        const v1 = this.y.mul(b.y);
        const v2 = this.x.mul(b.x);
        let t0 = this.x.add(this.y);
        let t1 = b.x.add(b.y);
        let tz = t0.mul(t1);
        tz = tz.sub(v1).sub(v2).mulXi().add(v0);
        t0 = this.y.add(this.z);
        t1 = b.y.add(b.z);
        let ty = t0.mul(t1);
        t0 = v2.mulXi();
        ty = ty.sub(v0).sub(v1).add(t0);
        t0 = this.x.add(this.z);
        t1 = b.x.add(b.z);
        let tx = t0.mul(t1);
        tx = tx.sub(v0).add(v1).sub(v2);
        return new GfP6(tx, ty, tz);
    }
    /**
     * Multiply the current element by a scalar
     * @param b the scalar
     * @returns the new element
     */
    mulScalar(b) {
        const x = this.x.mul(b);
        const y = this.y.mul(b);
        const z = this.z.mul(b);
        return new GfP6(x, y, z);
    }
    /**
     * Multiply the current element by a GFp element
     * @param b the GFp element
     * @returns the new element
     */
    mulGfP(b) {
        const x = this.x.mulScalar(b);
        const y = this.y.mulScalar(b);
        const z = this.z.mulScalar(b);
        return new GfP6(x, y, z);
    }
    mulTau() {
        const tz = this.x.mulXi();
        return new GfP6(this.y, this.z, tz);
    }
    /**
     * Get the square of the current element
     * @returns the new element
     */
    square() {
        const v0 = this.z.square();
        const v1 = this.y.square();
        const v2 = this.x.square();
        const c0 = this.x.add(this.y).square().sub(v1).sub(v2).mulXi().add(v0);
        const c1 = this.y.add(this.z).square().sub(v0).sub(v1).add(v2.mulXi());
        const c2 = this.x.add(this.z).square().sub(v0).add(v1).sub(v2);
        return new GfP6(c2, c1, c0);
    }
    /**
     * Get the inverse of the element
     * @returns the new element
     */
    invert() {
        const A = this.z.square().sub(this.x.mul(this.y).mulXi());
        const B = this.x.square().mulXi().sub(this.y.mul(this.z));
        const C = this.y.square().sub(this.x.mul(this.z));
        const F = C.mul(this.y).mulXi().add(A.mul(this.z)).add(B.mul(this.x).mulXi()).invert();
        return new GfP6(C.mul(F), B.mul(F), A.mul(F));
    }
    /**
     * Check the equality with the other object
     * @param o the other object
     * @returns true when both are equal, false otherwise
     */
    equals(o) {
        return this.x.equals(o.x) && this.y.equals(o.y) && this.z.equals(o.z);
    }
    /**
     * Get the string representation of the element
     * @returns a string representation
     */
    toString() {
        return `(${this.x.toString()}, ${this.y.toString()}, ${this.z.toString()})`;
    }
}
GfP6.ZERO = new GfP6();
GfP6.ONE = new GfP6(gfp2_1.default.zero(), gfp2_1.default.zero(), gfp2_1.default.one());
exports.default = GfP6;
