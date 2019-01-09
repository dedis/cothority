/// <reference types="node" />
import { Scalar } from "../../index";
import Weierstrass from "./curve";
/**
* Scalar
* @param {module:curves/nist/curve~Weirstrass} curve
* @param {BN.Red} red - BN.js Reduction context
* @constructor
*/
export default class NistScalar implements Scalar {
    ref: {
        arr: any;
        red: any;
        curve: Weierstrass;
    };
    constructor(curve: Weierstrass, red: any);
    string(): string;
    inspect(): string;
    /**
    * Equality test for two Scalars derived from the same Group
    */
    equal(s2: NistScalar): boolean;
    /**
    * Sets the receiver equal to another Scalar a
    */
    set(a: NistScalar): NistScalar;
    /**
    * Returns a copy of the scalar
    */
    clone(): NistScalar;
    /**
    * Set to the additive identity (0)
    */
    zero(): NistScalar;
    /**
    * Set to the modular sums of scalars s1 and s2
    */
    add(s1: NistScalar, s2: NistScalar): NistScalar;
    /**
    * Set to the modular difference
    */
    sub(s1: NistScalar, s2: NistScalar): NistScalar;
    /**
    * Set to the modular negation of scalar a
    */
    neg(a: NistScalar): NistScalar;
    /**
    * Set to the multiplicative identity (1)
    */
    one(): NistScalar;
    /**
    * Set to the modular products of scalars s1 and s2
    */
    mul(s1: NistScalar, s2: NistScalar): NistScalar;
    /**
    * Set to the modular division of scalar s1 by scalar s2
    *
    * @param {module:curves/nist/scalar~Scalar} s1
    * @param {module:curves/nist/scalar~Scalar} s2
    * @return {module:curves/nist/scalar~Scalar}
    */
    div(s1: NistScalar, s2: NistScalar): NistScalar;
    /**
    * Set to the modular inverse of scalar a
    */
    inv(a: NistScalar): NistScalar;
    /**
    * Sets the scalar from a big-endian buffer
    * and reduces to the appropriate modulus
    */
    setBytes(b: Buffer): NistScalar;
    /**
    * Returns a big-endian representation of the scalar
    */
    bytes(): Buffer;
    toString(): string;
    /**
    * Set to a random scalar
    */
    pick(callback?: (length: number) => Buffer): NistScalar;
    marshalSize(): number;
    /**
    * Returns the binary representation (big endian) of the scalar
    */
    marshalBinary(): Buffer;
    /**
    * Reads the binary representation (big endian) of scalar
    *
    * @throws {Error} if bytes.length != marshalSize
    */
    unmarshalBinary(bytes: Buffer): void;
}
