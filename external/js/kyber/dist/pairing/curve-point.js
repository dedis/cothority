"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const bn_js_1 = __importDefault(require("bn.js"));
const gfp_1 = __importDefault(require("./gfp"));
const constants_1 = require("./constants");
const curveB = new gfp_1.default(3);
/**
 * Point class used by G1
 */
class CurvePoint {
    constructor(x, y, z, t) {
        this.x = new gfp_1.default(x || 0);
        this.y = new gfp_1.default(y || 1);
        this.z = new gfp_1.default(z || 0);
        this.t = new gfp_1.default(t || 0);
    }
    /**
     * Get the x element of the point
     * @returns the x element
     */
    getX() {
        return this.x;
    }
    /**
     * Get the y element of the point
     * @returns the y element
     */
    getY() {
        return this.y;
    }
    /**
     * Check if the point is valid by checking if it is on the curve
     * @returns true when the point is valid, false otherwise
     */
    isOnCurve() {
        let yy = this.y.sqr();
        const xxx = this.x.pow(new bn_js_1.default(3));
        yy = yy.sub(xxx);
        yy = yy.sub(curveB);
        if (yy.signum() < 0 || yy.compareTo(new gfp_1.default(constants_1.p)) >= 0) {
            yy = yy.mod(constants_1.p);
        }
        return yy.signum() == 0;
    }
    /**
     * Set the point to the infinity
     */
    setInfinity() {
        this.x = new gfp_1.default(0);
        this.y = new gfp_1.default(1);
        this.z = new gfp_1.default(0);
        this.t = new gfp_1.default(0);
    }
    /**
     * Check if the point is the infinity
     * @returns true when infinity, false otherwise
     */
    isInfinity() {
        return this.z.isZero();
    }
    /**
     * Add a to b and set the value to the point
     * @param a the first point
     * @param b the second point
     */
    add(a, b) {
        if (a.isInfinity()) {
            this.copy(b);
            return;
        }
        if (b.isInfinity()) {
            this.copy(a);
            return;
        }
        const z1z1 = a.z.sqr().mod(constants_1.p);
        const z2z2 = b.z.sqr().mod(constants_1.p);
        const u1 = a.x.mul(z2z2).mod(constants_1.p);
        const u2 = b.x.mul(z1z1).mod(constants_1.p);
        let t = b.z.mul(z2z2).mod(constants_1.p);
        const s1 = a.y.mul(t).mod(constants_1.p);
        t = a.z.mul(z1z1).mod(constants_1.p);
        const s2 = b.y.mul(t).mod(constants_1.p);
        const h = u2.sub(u1);
        t = h.add(h);
        const i = t.sqr().mod(constants_1.p);
        const j = h.mul(i).mod(constants_1.p);
        t = s2.sub(s1);
        if (h.signum() === 0 && t.signum() === 0) {
            this.dbl(a);
            return;
        }
        const r = t.add(t);
        const v = u1.mul(i).mod(constants_1.p);
        let t4 = r.sqr().mod(constants_1.p);
        t = v.add(v);
        let t6 = t4.sub(j);
        this.x = t6.sub(t).mod(constants_1.p);
        t = v.sub(this.x);
        t4 = s1.mul(j).mod(constants_1.p);
        t6 = t4.add(t4);
        t4 = r.mul(t).mod(constants_1.p);
        this.y = t4.sub(t6).mod(constants_1.p);
        t = a.z.add(b.z);
        t4 = t.sqr().mod(constants_1.p);
        t = t4.sub(z1z1);
        t4 = t.sub(z2z2);
        this.z = t4.mul(h).mod(constants_1.p);
    }
    /**
     * Compute the double of a and set the value to the point
     * @param a the point to double
     */
    dbl(a) {
        const A = a.x.sqr().mod(constants_1.p);
        const B = a.y.sqr().mod(constants_1.p);
        const C = B.sqr().mod(constants_1.p);
        let t = a.x.add(B);
        let t2 = t.sqr().mod(constants_1.p);
        t = t2.sub(A);
        t2 = t.sub(C);
        const d = t2.add(t2);
        t = A.add(A);
        const e = t.add(A);
        const f = e.sqr().mod(constants_1.p);
        t = d.add(d);
        this.x = f.sub(t).mod(constants_1.p);
        t = C.add(C);
        t2 = t.add(t);
        t = t2.add(t2);
        this.y = d.sub(this.x);
        t2 = e.mul(this.y).mod(constants_1.p);
        this.y = t2.sub(t).mod(constants_1.p);
        t = a.y.mul(a.z).mod(constants_1.p);
        this.z = t.add(t).mod(constants_1.p);
    }
    /**
     * Multiply a by a scalar
     * @param a      the point to multiply
     * @param scalar the scalar
     */
    mul(a, scalar) {
        const sum = new CurvePoint();
        sum.setInfinity();
        const t = new CurvePoint();
        for (let i = scalar.bitLength(); i >= 0; i--) {
            t.dbl(sum);
            if (scalar.testn(i)) {
                sum.add(t, a);
            }
            else {
                sum.copy(t);
            }
        }
        this.copy(sum);
    }
    /**
     * Normalize the point coordinates
     */
    makeAffine() {
        if (this.z.isOne()) {
            return;
        }
        else if (this.z.isZero()) {
            this.setInfinity();
            return;
        }
        const zInv = this.z.invmod(constants_1.p);
        let t = this.y.mul(zInv).mod(constants_1.p);
        const zInv2 = zInv.sqr().mod(constants_1.p);
        this.y = t.mul(zInv2).mod(constants_1.p);
        t = this.x.mul(zInv2).mod(constants_1.p);
        this.x = t;
        this.z = new gfp_1.default(1);
        this.t = new gfp_1.default(1);
    }
    /**
     * Compute the negative of a and set the value to the point
     * @param a the point to negate
     */
    negative(a) {
        this.x = a.x;
        this.y = a.y.negate();
        this.z = a.z;
        this.t = new gfp_1.default(0);
    }
    /**
     * Fill the point with the values of a
     * @param p the point to copy
     */
    copy(p) {
        // immutable objects so we can copy them
        this.x = p.x;
        this.y = p.y;
        this.z = p.z;
        this.t = p.t;
    }
    /**
     * Get a clone of the current point
     * @returns a clone of the point
     */
    clone() {
        const p = new CurvePoint();
        p.copy(this);
        return p;
    }
    /**
     * Check the equality between the point and the object
     * @param o the object
     * @returns true when both are equal, false otherwise
     */
    equals(o) {
        if (!(o instanceof CurvePoint)) {
            return false;
        }
        const a = this.clone();
        a.makeAffine();
        const b = o.clone();
        b.makeAffine();
        return a.x.equals(b.x) && a.y.equals(b.y) && a.z.equals(b.z) && a.t.equals(b.t);
    }
    /**
     * Get the string representation of the point
     * @returns the string representation
     */
    toString() {
        const p = this.clone();
        p.makeAffine();
        return `(${p.getX().toString()},${p.getY().toString()})`;
    }
}
CurvePoint.generator = new CurvePoint(1, -2, 1, 0);
exports.default = CurvePoint;
