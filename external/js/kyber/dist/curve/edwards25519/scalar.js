"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const BN = require("bn.js");
const crypto_1 = require("crypto");
const random_1 = require("../../random");
class Ed25519Scalar {
    constructor(curve, red) {
        this.ref = {
            arr: new BN(0, 16).toRed(red),
            curve: curve,
            red: red,
        };
    }
    marshalSize() {
        return 32;
    }
    marshalBinary() {
        return Buffer.from(this.ref.arr.fromRed().toArray("le", 32));
    }
    unmarshalBinary(bytes) {
        if (bytes.length > this.marshalSize()) {
            throw new Error("bytes.length > marshalSize");
        }
        this.ref.arr = new BN(bytes, 16, "le").toRed(this.ref.red);
    }
    equal(s2) {
        return this.ref.arr.fromRed().cmp(s2.ref.arr.fromRed()) == 0;
    }
    set(a) {
        this.ref = a.ref;
        return this;
    }
    clone() {
        return new Ed25519Scalar(this.ref.curve, this.ref.red).setBytes(Buffer.from(this.ref.arr.fromRed().toArray("le")));
    }
    zero() {
        this.ref.arr = new BN(0, 16).toRed(this.ref.red);
        return this;
    }
    add(a, b) {
        this.ref.arr = a.ref.arr.redAdd(b.ref.arr);
        return this;
    }
    sub(a, b) {
        this.ref.arr = a.ref.arr.redSub(b.ref.arr);
        return this;
    }
    neg(a) {
        this.ref.arr = a.ref.arr.redNeg();
        return this;
    }
    mul(s1, s2) {
        this.ref.arr = s1.ref.arr.redMul(s2.ref.arr);
        return this;
    }
    div(s1, s2) {
        this.ref.arr = s1.ref.arr.redMul(s2.ref.arr.redInvm());
        return this;
    }
    inv(a) {
        this.ref.arr = a.ref.arr.redInvm();
        return this;
    }
    one() {
        this.ref.arr = new BN(1, 16).toRed(this.ref.red);
        return this;
    }
    pick(callback) {
        callback = callback || crypto_1.randomBytes;
        const bytes = random_1.int(this.ref.curve.curve.n, callback);
        this.ref.arr = new BN(bytes, 16).toRed(this.ref.red);
        return this;
    }
    setBytes(bytes) {
        this.ref.arr = new BN(bytes, 16, "le").toRed(this.ref.red);
        return this;
    }
    toString() {
        const bytes = this.ref.arr.fromRed().toArray("le", 32);
        return bytes.map(b => ("0" + (b & 0xff).toString(16)).slice(-2)).join("");
    }
}
exports.default = Ed25519Scalar;
