"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const bn_js_1 = __importDefault(require("bn.js"));
const crypto_1 = require("crypto");
const random_1 = require("../../random");
/**
* Scalar
* @param {module:curves/nist/curve~Weirstrass} curve
* @param {BN.Red} red - BN.js Reduction context
* @constructor
*/
class NistScalar {
    constructor(curve, red) {
        this.ref = {
            arr: new bn_js_1.default(0, 16).toRed(red),
            red: red,
            curve: curve
        };
    }
    string() {
        return this.toString();
    }
    inspect() {
        return this.toString();
    }
    /**
    * Equality test for two Scalars derived from the same Group
    */
    equal(s2) {
        return this.ref.arr.fromRed().cmp(s2.ref.arr.fromRed()) == 0;
    }
    /**
    * Sets the receiver equal to another Scalar a
    */
    set(a) {
        this.ref = a.ref;
        return this;
    }
    /**
    * Returns a copy of the scalar
    */
    clone() {
        return new NistScalar(this.ref.curve, this.ref.red).setBytes(Buffer.from(this.ref.arr.fromRed().toArray("be")));
    }
    /**
    * Set to the additive identity (0)
    */
    zero() {
        this.ref.arr = new bn_js_1.default(0, 16).toRed(this.ref.red);
        return this;
    }
    /**
    * Set to the modular sums of scalars s1 and s2
    */
    add(s1, s2) {
        this.ref.arr = s1.ref.arr.redAdd(s2.ref.arr);
        return this;
    }
    /**
    * Set to the modular difference
    */
    sub(s1, s2) {
        this.ref.arr = s1.ref.arr.redSub(s2.ref.arr);
        return this;
    }
    /**
    * Set to the modular negation of scalar a
    */
    neg(a) {
        this.ref.arr = a.ref.arr.redNeg();
        return this;
    }
    /**
    * Set to the multiplicative identity (1)
    */
    one() {
        this.ref.arr = new bn_js_1.default(1, 16).toRed(this.ref.red);
        return this;
    }
    /**
    * Set to the modular products of scalars s1 and s2
    */
    mul(s1, s2) {
        this.ref.arr = s1.ref.arr.redMul(s2.ref.arr);
        return this;
    }
    /**
    * Set to the modular division of scalar s1 by scalar s2
    *
    * @param {module:curves/nist/scalar~Scalar} s1
    * @param {module:curves/nist/scalar~Scalar} s2
    * @return {module:curves/nist/scalar~Scalar}
    */
    div(s1, s2) {
        this.ref.arr = s1.ref.arr.redMul(s2.ref.arr.redInvm());
        return this;
    }
    /**
    * Set to the modular inverse of scalar a
    */
    inv(a) {
        this.ref.arr = a.ref.arr.redInvm();
        return this;
    }
    /**
    * Sets the scalar from a big-endian buffer
    * and reduces to the appropriate modulus
    */
    setBytes(b) {
        this.ref.arr = new bn_js_1.default(b, 16, "be").toRed(this.ref.red);
        return this;
    }
    /**
    * Returns a big-endian representation of the scalar
    */
    bytes() {
        return Buffer.from(this.ref.arr.fromRed().toArray("be"));
    }
    toString() {
        let bytes = Buffer.from(this.ref.arr.fromRed().toArray("be"));
        return Array.from(bytes, b => {
            return ("0" + (b & 0xff).toString(16)).slice(-2);
        }).join("");
    }
    /**
    * Set to a random scalar
    */
    pick(callback) {
        callback = callback || crypto_1.randomBytes;
        let bytes = random_1.int(this.ref.curve.curve.n, callback);
        this.setBytes(bytes);
        return this;
    }
    marshalSize() {
        return this.ref.curve.scalarLen();
    }
    /**
    * Returns the binary representation (big endian) of the scalar
    */
    marshalBinary() {
        return Buffer.from(this.ref.arr.fromRed().toArray("be", this.ref.curve.scalarLen()));
    }
    /**
    * Reads the binary representation (big endian) of scalar
    *
    * @throws {Error} if bytes.length != marshalSize
    */
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
