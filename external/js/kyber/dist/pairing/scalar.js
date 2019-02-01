"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const crypto_1 = require("crypto");
const bn_js_1 = __importDefault(require("bn.js"));
const constants_1 = require("./constants");
const random_1 = require("../random");
/**
 * Scalar used in combination with G1 and G2 points
 */
class BN256Scalar {
    constructor(value) {
        this.v = new bn_js_1.default(value).umod(constants_1.p);
    }
    /**
     * Get the BigNumber value of the scalar
     * @returns the value
     */
    getValue() {
        return this.v;
    }
    /** @inheritdoc */
    set(a) {
        this.v = a.v.clone();
        return this;
    }
    /** @inheritdoc */
    one() {
        this.v = new bn_js_1.default(1);
        return this;
    }
    /** @inheritdoc */
    zero() {
        this.v = new bn_js_1.default(0);
        return this;
    }
    /** @inheritdoc */
    add(a, b) {
        this.v = a.v.add(b.v).umod(constants_1.p);
        return this;
    }
    /** @inheritdoc */
    sub(a, b) {
        this.v = a.v.sub(b.v).umod(constants_1.p);
        return this;
    }
    /** @inheritdoc */
    neg(a) {
        this.v = a.v.neg().umod(constants_1.p);
        return this;
    }
    /** @inheritdoc */
    div(a, b) {
        this.v = a.v.div(b.v).umod(constants_1.p);
        return this;
    }
    /** @inheritdoc */
    mul(s1, b) {
        this.v = s1.v.mul(b.v).umod(constants_1.p);
        return this;
    }
    /** @inheritdoc */
    inv(a) {
        this.v = a.v.invm(constants_1.p);
        return this;
    }
    /** @inheritdoc */
    pick(callback) {
        callback = callback || crypto_1.randomBytes;
        const bytes = random_1.int(constants_1.p, callback);
        this.setBytes(bytes);
        return this;
    }
    /** @inheritdoc */
    setBytes(bytes) {
        this.v = new bn_js_1.default(bytes, 16);
        return this;
    }
    /** @inheritdoc */
    marshalBinary() {
        return this.v.toArrayLike(Buffer, 'be', 32);
    }
    /** @inheritdoc */
    unmarshalBinary(buf) {
        this.v = new bn_js_1.default(buf, 16);
    }
    /** @inheritdoc */
    clone() {
        const s = new BN256Scalar(new bn_js_1.default(this.v));
        return s;
    }
    /** @inheritdoc */
    equals(s2) {
        return this.v.eq(s2.v);
    }
}
exports.default = BN256Scalar;
