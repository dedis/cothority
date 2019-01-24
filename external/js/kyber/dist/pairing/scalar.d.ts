/// <reference types="node" />
import BN from 'bn.js';
import { Scalar } from '../index';
export declare type BNType = number | string | number[] | Buffer | BN;
/**
 * Scalar used in combination with G1 and G2 points
 */
export default class BN256Scalar implements Scalar {
    private v;
    constructor(value?: BNType);
    /**
     * Get the BigNumber value of the scalar
     * @returns the value
     */
    getValue(): BN;
    /** @inheritdoc */
    set(a: BN256Scalar): BN256Scalar;
    /** @inheritdoc */
    one(): BN256Scalar;
    /** @inheritdoc */
    zero(): BN256Scalar;
    /** @inheritdoc */
    add(a: BN256Scalar, b: BN256Scalar): BN256Scalar;
    /** @inheritdoc */
    sub(a: BN256Scalar, b: BN256Scalar): BN256Scalar;
    /** @inheritdoc */
    neg(a: BN256Scalar): BN256Scalar;
    /** @inheritdoc */
    div(a: BN256Scalar, b: BN256Scalar): BN256Scalar;
    /** @inheritdoc */
    mul(s1: BN256Scalar, b: BN256Scalar): BN256Scalar;
    /** @inheritdoc */
    inv(a: BN256Scalar): BN256Scalar;
    /** @inheritdoc */
    pick(callback?: (length: number) => Buffer): BN256Scalar;
    /** @inheritdoc */
    setBytes(bytes: Buffer): BN256Scalar;
    /** @inheritdoc */
    marshalBinary(): Buffer;
    /** @inheritdoc */
    unmarshalBinary(buf: Buffer | string): void;
    /** @inheritdoc */
    clone(): BN256Scalar;
    /** @inheritdoc */
    equal(s2: BN256Scalar): boolean;
}
