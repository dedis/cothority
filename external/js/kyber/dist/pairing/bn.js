"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const bn_js_1 = __importDefault(require("bn.js"));
const curve_point_1 = __importDefault(require("./curve-point"));
const twist_point_1 = __importDefault(require("./twist-point"));
const gfp2_1 = __importDefault(require("./gfp2"));
const gfp12_1 = __importDefault(require("./gfp12"));
const gfp6_1 = __importDefault(require("./gfp6"));
const opt_ate_1 = require("./opt-ate");
/**
 * Wrapper around the basic curve point. It acts as a mutable object and
 * then every modification is done in-place.
 */
class G1 {
    constructor(k) {
        this.p = new curve_point_1.default();
        if (k) {
            this.scalarBaseMul(new bn_js_1.default(k));
        }
    }
    /**
     * Get the curve point
     * @returns the point
     */
    getPoint() {
        return this.p;
    }
    /**
     * Set the point to the generator of the curve
     */
    setBase() {
        this.p = curve_point_1.default.generator.clone();
    }
    /**
     * Set the point to infinity
     */
    setInfinity() {
        this.p.setInfinity();
    }
    /**
     * Check if the point is the infinity
     * @returns true when infinity, false otherwise
     */
    isInfinity() {
        return this.p.isInfinity();
    }
    /**
     * Multiply the generator by the scalar k and set the value
     * @param k the scalar
     */
    scalarBaseMul(k) {
        this.p.mul(curve_point_1.default.generator, k);
    }
    /**
     * Multiply a by the scalar k and set the value
     * @param a the point
     * @param k the scalar
     */
    scalarMul(a, k) {
        this.p.mul(a.p, k);
    }
    /**
     * Add a to b and set the value
     * @param a the first point
     * @param b the second point
     */
    add(a, b) {
        this.p.add(a.p, b.p);
    }
    /**
     * Compute the negative of a and set the value
     * @param the point to negate
     */
    neg(a) {
        this.p.negative(a.p);
    }
    /**
     * Get the buffer size after marshaling
     * @returns the length
     */
    marshalSize() {
        return G1.MARSHAL_SIZE;
    }
    /**
     * Serialize the point into bytes
     * @returns the buffer
     */
    marshal() {
        const p = this.p.clone();
        const buf = Buffer.alloc(G1.MARSHAL_SIZE, 0);
        if (p.isInfinity()) {
            return buf;
        }
        p.makeAffine();
        const xBytes = p.getX().toBytes();
        const yBytes = p.getY().toBytes();
        return Buffer.concat([xBytes, yBytes]);
    }
    /**
     * Take a buffer to deserialize a point
     * @param bytes the buffer
     */
    unmarshal(bytes) {
        if (bytes.length != G1.MARSHAL_SIZE) {
            throw new Error('wrong buffer size for a G1 point');
        }
        this.p = new curve_point_1.default(bytes.slice(0, G1.ELEM_SIZE), bytes.slice(G1.ELEM_SIZE), 1, 1);
        if (this.p.getX().isZero() && this.p.getY().isZero()) {
            this.p.setInfinity();
            return;
        }
        if (!this.p.isOnCurve()) {
            throw new Error('malformed G1 point');
        }
    }
    /**
     * Check the equality between the point and the object
     * @param o the object
     * @returns true when both are equal, false otherwise
     */
    equals(o) {
        if (!(o instanceof G1)) {
            return false;
        }
        return this.p.equals(o.p);
    }
    /**
     * Get a clone of the element
     * @returns the new element
     */
    clone() {
        const g = new G1();
        g.p = this.p.clone();
        return g;
    }
    /**
     * Get the string representation of the point
     * @returns the string representation
     */
    toString() {
        return `bn256.G1${this.p.toString()}`;
    }
}
G1.ELEM_SIZE = 256 / 8;
G1.MARSHAL_SIZE = G1.ELEM_SIZE * 2;
exports.G1 = G1;
/**
 * Wrapper around the twist point. It acts as a mutable object and
 * then every modification is done in-place.
 */
class G2 {
    constructor(k) {
        this.p = new twist_point_1.default();
        if (k) {
            this.scalarBaseMul(new bn_js_1.default(k));
        }
    }
    /**
     * Get the twist point
     * @returns the point
     */
    getPoint() {
        return this.p;
    }
    /**
     * Set to the generator of the curve
     */
    setBase() {
        this.p = twist_point_1.default.generator.clone();
    }
    /**
     * Set the point to the infinity
     */
    setInfinity() {
        this.p.setInfinity();
    }
    /**
     * Check if the point is the infinity
     * @returns true when infinity, false otherwise
     */
    isInfinity() {
        return this.p.isInfinity();
    }
    /**
     * Mutliply the generator by a scalar k and set the value
     * @param k the scalar
     */
    scalarBaseMul(k) {
        this.p.mul(twist_point_1.default.generator, k);
    }
    /**
     * Multiply a by a scalar k and set the value
     * @param a the point
     * @param k the scalar
     */
    scalarMul(a, k) {
        this.p.mul(a.p, k);
    }
    /**
     * Add a to b and set the value
     * @param a the first point
     * @param b the second point
     */
    add(a, b) {
        this.p.add(a.p, b.p);
    }
    /**
     * Compute the negative of a and set the value
     * @param a the point
     */
    neg(a) {
        this.p.neg(a.p);
    }
    /**
     * Get the size of the buffer after marshaling
     * @returns the size
     */
    marshalSize() {
        return G2.MARSHAL_SIZE;
    }
    /**
     * Serialize the point into bytes
     * @returns the buffer
     */
    marshal() {
        if (this.isInfinity()) {
            return Buffer.alloc(G2.MARSHAL_SIZE, 0);
        }
        const t = this.clone();
        t.p.makeAffine();
        const xxBytes = t.p.getX().getX().toBytes();
        const xyBytes = t.p.getX().getY().toBytes();
        const yxBytes = t.p.getY().getX().toBytes();
        const yyBytes = t.p.getY().getY().toBytes();
        return Buffer.concat([xxBytes, xyBytes, yxBytes, yyBytes]);
    }
    /**
     * Take a buffer and deserialize a point
     * @param bytes the buffer
     */
    unmarshal(bytes) {
        if (bytes.length !== G2.MARSHAL_SIZE) {
            throw new Error('wrong buffer size for G2 point');
        }
        const xxBytes = bytes.slice(0, G2.ELEM_SIZE);
        const xyBytes = bytes.slice(G2.ELEM_SIZE, G2.ELEM_SIZE * 2);
        const yxBytes = bytes.slice(G2.ELEM_SIZE * 2, G2.ELEM_SIZE * 3);
        const yyBytes = bytes.slice(G2.ELEM_SIZE * 3);
        this.p = new twist_point_1.default(new gfp2_1.default(xxBytes, xyBytes), new gfp2_1.default(yxBytes, yyBytes), gfp2_1.default.one(), gfp2_1.default.one());
        if (this.p.getX().isZero() && this.p.getY().isZero()) {
            this.p.setInfinity();
            return;
        }
        if (!this.p.isOnCurve()) {
            throw new Error('malformed G2 point');
        }
    }
    /**
     * Get a clone of G2
     * @returns the clone
     */
    clone() {
        const t = new G2();
        t.p = this.p.clone();
        return t;
    }
    /**
     * Check the equality of the current point and the object
     * @param o the object
     * @returns true when both are equal, false otherwise
     */
    equals(o) {
        if (!(o instanceof G2)) {
            return false;
        }
        return this.p.equals(o.p);
    }
    /**
     * Get the string representation of the point
     * @returns the string representation
     */
    toString() {
        return `bn256.G2${this.p.toString()}`;
    }
}
G2.ELEM_SIZE = 256 / 8;
G2.MARSHAL_SIZE = G2.ELEM_SIZE * 4;
exports.G2 = G2;
/**
 * Wrapper around the result of pairing of G1 and G2. It acts as a mutable
 * object and then every modification is done in-place.
 */
class GT {
    constructor(g) {
        this.g = g || new gfp12_1.default();
    }
    static pair(g1, g2) {
        return opt_ate_1.optimalAte(g1, g2);
    }
    static one() {
        return new GT(gfp12_1.default.one());
    }
    /**
     * Check if the point is one
     * @returns true when one, false otherwise
     */
    isOne() {
        return this.g.isOne();
    }
    /**
     * Multiply the point a by a scalar k and set the value
     * @param a the point
     * @param k the scalar
     */
    scalarMul(a, k) {
        this.g = a.g.exp(k);
    }
    /**
     * Add two points a and b and set the value
     * @param a the first point
     * @param b the second point
     */
    add(a, b) {
        this.g = a.g.mul(b.g);
    }
    /**
     * Compute the negative of a and set the value
     * @param a the point
     */
    neg(a) {
        this.g = a.g.invert();
    }
    /**
     * Serialize the point into bytes
     * @returns the buffer
     */
    marshal() {
        const xxxBytes = this.g.getX().getX().getX().toBytes();
        const xxyBytes = this.g.getX().getX().getY().toBytes();
        const xyxBytes = this.g.getX().getY().getX().toBytes();
        const xyyBytes = this.g.getX().getY().getY().toBytes();
        const xzxBytes = this.g.getX().getZ().getX().toBytes();
        const xzyBytes = this.g.getX().getZ().getY().toBytes();
        const yxxBytes = this.g.getY().getX().getX().toBytes();
        const yxyBytes = this.g.getY().getX().getY().toBytes();
        const yyxBytes = this.g.getY().getY().getX().toBytes();
        const yyyBytes = this.g.getY().getY().getY().toBytes();
        const yzxBytes = this.g.getY().getZ().getX().toBytes();
        const yzyBytes = this.g.getY().getZ().getY().toBytes();
        return Buffer.concat([
            xxxBytes, xxyBytes, xyxBytes,
            xyyBytes, xzxBytes, xzyBytes,
            yxxBytes, yxyBytes, yyxBytes,
            yyyBytes, yzxBytes, yzyBytes,
        ]);
    }
    /**
     * Take a buffer and deserialize a point
     * @param bytes the buffer
     */
    unmarshal(bytes) {
        if (bytes.length !== GT.MARSHAL_SIZE) {
            throw new Error('wrong buffer size for a GT point');
        }
        const n = GT.ELEM_SIZE;
        const xxxBytes = bytes.slice(0, n);
        const xxyBytes = bytes.slice(n, n * 2);
        const xyxBytes = bytes.slice(n * 2, n * 3);
        const xyyBytes = bytes.slice(n * 3, n * 4);
        const xzxBytes = bytes.slice(n * 4, n * 5);
        const xzyBytes = bytes.slice(n * 5, n * 6);
        const yxxBytes = bytes.slice(n * 6, n * 7);
        const yxyBytes = bytes.slice(n * 7, n * 8);
        const yyxBytes = bytes.slice(n * 8, n * 9);
        const yyyBytes = bytes.slice(n * 9, n * 10);
        const yzxBytes = bytes.slice(n * 10, n * 11);
        const yzyBytes = bytes.slice(n * 11);
        this.g = new gfp12_1.default(new gfp6_1.default(new gfp2_1.default(xxxBytes, xxyBytes), new gfp2_1.default(xyxBytes, xyyBytes), new gfp2_1.default(xzxBytes, xzyBytes)), new gfp6_1.default(new gfp2_1.default(yxxBytes, yxyBytes), new gfp2_1.default(yyxBytes, yyyBytes), new gfp2_1.default(yzxBytes, yzyBytes)));
    }
    /**
     * Check the equality of the point and an object
     * @param o the object
     * @returns true when both are equal, false otherwise
     */
    equals(o) {
        if (!(o instanceof GT)) {
            return false;
        }
        return this.g.equals(o.g);
    }
    /**
     * Get the string representation of the point
     * @returns the string representation
     */
    toString() {
        return `bn256.GT${this.g.toString()}`;
    }
}
GT.ELEM_SIZE = 256 / 8;
GT.MARSHAL_SIZE = GT.ELEM_SIZE * 12;
exports.GT = GT;
