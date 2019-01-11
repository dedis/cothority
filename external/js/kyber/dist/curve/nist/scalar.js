"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const bn_js_1 = __importDefault(require("bn.js"));
const crypto_1 = require("crypto");
const random_1 = require("../../random");
class NistScalar {
    constructor(curve, red) {
        this.ref = {
            arr: new bn_js_1.default(0, 16).toRed(red),
            red: red,
            curve: curve
        };
    }
    /** @inheritdoc */
    string() {
        return this.toString();
    }
    inspect() {
        return this.toString();
    }
    /** @inheritdoc */
    equal(s2) {
        return this.ref.arr.fromRed().cmp(s2.ref.arr.fromRed()) == 0;
    }
    /** @inheritdoc */
    set(a) {
        this.ref = a.ref;
        return this;
    }
    /** @inheritdoc */
    clone() {
        return new NistScalar(this.ref.curve, this.ref.red).setBytes(Buffer.from(this.ref.arr.fromRed().toArray("be")));
    }
    /** @inheritdoc */
    zero() {
        this.ref.arr = new bn_js_1.default(0, 16).toRed(this.ref.red);
        return this;
    }
    /** @inheritdoc */
    add(s1, s2) {
        this.ref.arr = s1.ref.arr.redAdd(s2.ref.arr);
        return this;
    }
    /** @inheritdoc */
    sub(s1, s2) {
        this.ref.arr = s1.ref.arr.redSub(s2.ref.arr);
        return this;
    }
    /** @inheritdoc */
    neg(a) {
        this.ref.arr = a.ref.arr.redNeg();
        return this;
    }
    /** @inheritdoc */
    one() {
        this.ref.arr = new bn_js_1.default(1, 16).toRed(this.ref.red);
        return this;
    }
    /** @inheritdoc */
    mul(s1, s2) {
        this.ref.arr = s1.ref.arr.redMul(s2.ref.arr);
        return this;
    }
    /** @inheritdoc */
    div(s1, s2) {
        this.ref.arr = s1.ref.arr.redMul(s2.ref.arr.redInvm());
        return this;
    }
    /** @inheritdoc */
    inv(a) {
        this.ref.arr = a.ref.arr.redInvm();
        return this;
    }
    /** @inheritdoc */
    setBytes(b) {
        this.ref.arr = new bn_js_1.default(b, 16, "be").toRed(this.ref.red);
        return this;
    }
    /** @inheritdoc */
    bytes() {
        return Buffer.from(this.ref.arr.fromRed().toArray("be"));
    }
    toString() {
        let bytes = Buffer.from(this.ref.arr.fromRed().toArray("be"));
        return Array.from(bytes, b => {
            return ("0" + (b & 0xff).toString(16)).slice(-2);
        }).join("");
    }
    /** @inheritdoc */
    pick(callback) {
        callback = callback || crypto_1.randomBytes;
        let bytes = random_1.int(this.ref.curve.curve.n, callback);
        this.setBytes(bytes);
        return this;
    }
    /** @inheritdoc */
    marshalSize() {
        return this.ref.curve.scalarLen();
    }
    /** @inheritdoc */
    marshalBinary() {
        return Buffer.from(this.ref.arr.fromRed().toArray("be", this.ref.curve.scalarLen()));
    }
    /** @inheritdoc */
    unmarshalBinary(bytes) {
        if (bytes.length != this.marshalSize()) {
            throw new Error("bytes.length > marshalSize");
        }
        const bnObj = new bn_js_1.default(bytes, 16);
        if (bnObj.cmp(this.ref.curve.curve.n) > 0) {
            throw new Error("bytes > q");
        }
        this.setBytes(bytes);
    }
}
exports.default = NistScalar;
