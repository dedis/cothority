import BN = require("bn.js");
import * as crypto from "crypto"
import { int } from "../../random"
import { Scalar } from "../../index"
import Weierstrass from "./curve";

/**
* Scalar
* @param {module:curves/nist/curve~Weirstrass} curve
* @param {BN.Red} red - BN.js Reduction context
* @constructor
*/
export default class NistScalar implements Scalar {
    ref: { arr: any, red: any, curve: Weierstrass}
    constructor(curve: Weierstrass, red: any) {
        this.ref = {
            arr: new BN(0, 16).toRed(red),
            red: red,
            curve: curve
        };
    }

    string() {
        return this.toString()
    }

    inspect() {
        return this.toString()
    }
    
    /**
    * Equality test for two Scalars derived from the same Group
    */
    equal(s2: NistScalar): boolean {
        return this.ref.arr.fromRed().cmp(s2.ref.arr.fromRed()) == 0;
    }
    
    /**
    * Sets the receiver equal to another Scalar a
    */
    set(a: NistScalar): NistScalar {
        this.ref = a.ref;
        return this;
    }
    
    /**
    * Returns a copy of the scalar
    */
    clone(): NistScalar {
        return new NistScalar(this.ref.curve, this.ref.red).setBytes(
            Buffer.from(this.ref.arr.fromRed().toArray("be"))
        );
    }
    
    /**
    * Set to the additive identity (0)
    */
    zero(): NistScalar {
        this.ref.arr = new BN(0, 16).toRed(this.ref.red);
        return this;
    }
    
    /**
    * Set to the modular sums of scalars s1 and s2
    */
    add(s1: NistScalar, s2: NistScalar): NistScalar {
        this.ref.arr = s1.ref.arr.redAdd(s2.ref.arr);
        return this;
    }
    
    /**
    * Set to the modular difference
    */
    sub(s1: NistScalar, s2: NistScalar): NistScalar {
        this.ref.arr = s1.ref.arr.redSub(s2.ref.arr);
        return this;
    }
    
    /**
    * Set to the modular negation of scalar a
    */
    neg(a: NistScalar): NistScalar {
        this.ref.arr = a.ref.arr.redNeg();
        return this;
    }
    
    /**
    * Set to the multiplicative identity (1)
    */
    one(): NistScalar {
        this.ref.arr = new BN(1, 16).toRed(this.ref.red);
        return this;
    }
    
    /**
    * Set to the modular products of scalars s1 and s2
    */
    mul(s1: NistScalar, s2: NistScalar): NistScalar {
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
    div(s1: NistScalar, s2: NistScalar): NistScalar {
        this.ref.arr = s1.ref.arr.redMul(s2.ref.arr.redInvm());
        return this;
    }
    
    /**
    * Set to the modular inverse of scalar a
    */
    inv(a: NistScalar): NistScalar {
        this.ref.arr = a.ref.arr.redInvm();
        return this;
    }
    
    /**
    * Sets the scalar from big-endian Uint8Array
    * and reduces to the appropriate modulus
    */
    setBytes(b: Buffer): NistScalar{
        this.ref.arr = new BN(b, 16, "be").toRed(this.ref.red);
        return this;
    }
    
    /**
    * Returns a big-endian representation of the scalar
    */
    bytes(): Buffer{
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
    pick(callback?: (length: number) => Buffer): NistScalar {
        callback = callback || crypto.randomBytes;
        let bytes = int(this.ref.curve.curve.n, callback);
        this.setBytes(bytes);
        return this;
    }
    
    marshalSize(): number {
        return this.ref.curve.scalarLen();
    }
    
    /**
    * Returns the binary representation (big endian) of the scalar
    */
    marshalBinary(): Buffer {
        return Buffer.from(
            this.ref.arr.fromRed().toArray("be", this.ref.curve.scalarLen())
        );
    }
    
    /**
    * Reads the binary representation (big endian) of scalar
    * 
    * @throws {Error} if bytes.length != marshalSize
    */
    unmarshalBinary(bytes: Buffer) {
        if (bytes.length != this.marshalSize()) {
            throw new Error("bytes.length > marshalSize");
        }

        const bnObj = new BN(bytes, 16);
        if (bnObj.cmp(this.ref.curve.curve.n) > 0) {
            throw new Error("bytes > q");
        }
        this.setBytes(bytes);
    }
}