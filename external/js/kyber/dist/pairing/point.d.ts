/// <reference types="node" />
import { Point } from '../index';
import { G1, G2, GT, BNType } from './bn';
import BN256Scalar from './scalar';
/**
 * Implementation of the point interface for G1
 */
export declare class BN256G1Point implements Point {
    private g1;
    constructor(k?: BNType);
    /**
     * Get the element of the point
     * @returns the element
     */
    getElement(): G1;
    /** @inheritdoc */
    null(): BN256G1Point;
    /** @inheritdoc */
    base(): BN256G1Point;
    /** @inheritdoc */
    pick(callback?: (length: number) => Buffer): BN256G1Point;
    /** @inheritdoc */
    set(p: BN256G1Point): BN256G1Point;
    /** @inheritdoc */
    clone(): BN256G1Point;
    /** @inheritdoc */
    embedLen(): number;
    /** @inheritdoc */
    embed(data: Buffer, callback?: (length: number) => Buffer): BN256G1Point;
    /** @inheritdoc */
    data(): Buffer;
    /** @inheritdoc */
    add(a: BN256G1Point, b: BN256G1Point): BN256G1Point;
    /** @inheritdoc */
    sub(a: BN256G1Point, b: BN256G1Point): BN256G1Point;
    /** @inheritdoc */
    neg(p: BN256G1Point): BN256G1Point;
    /** @inheritdoc */
    mul(s: BN256Scalar, p: BN256G1Point): BN256G1Point;
    /**
     * Compute the pairing between the current point and
     * and the provided G2 point
     * @returns the resulting GT point
     */
    pair(g2: BN256G2Point): GT;
    /** @inheritdoc */
    marshalBinary(): Buffer;
    /** @inheritdoc */
    unmarshalBinary(buf: Buffer): void;
    /** @inheritdoc */
    marshalSize(): number;
    /** @inheritdoc */
    equal(p2: BN256G1Point): boolean;
    /** @inheritdoc */
    string(): string;
}
/**
 * Implementation of the point interface for G2
 */
export declare class BN256G2Point implements Point {
    private g2;
    constructor(k?: BNType);
    /**
     * Get the element of the point
     * @returns the element
     */
    getElement(): G2;
    /** @inheritdoc */
    null(): BN256G2Point;
    /** @inheritdoc */
    base(): BN256G2Point;
    /** @inheritdoc */
    pick(callback?: (length: number) => Buffer): BN256G2Point;
    /** @inheritdoc */
    set(p: BN256G2Point): BN256G2Point;
    /** @inheritdoc */
    clone(): BN256G2Point;
    /** @inheritdoc */
    embedLen(): number;
    /** @inheritdoc */
    embed(data: Buffer, callback?: (length: number) => Buffer): BN256G2Point;
    /** @inheritdoc */
    data(): Buffer;
    /** @inheritdoc */
    add(p1: BN256G2Point, p2: BN256G2Point): BN256G2Point;
    /** @inheritdoc */
    sub(p1: BN256G2Point, p2: BN256G2Point): BN256G2Point;
    /** @inheritdoc */
    neg(p: BN256G2Point): BN256G2Point;
    /** @inheritdoc */
    mul(s: BN256Scalar, p?: BN256G2Point): BN256G2Point;
    /**
     * Compute the pairing between the current point and
     * the provided G1 point
     * @returns the resulting GT point
     */
    pair(g1: BN256G1Point): GT;
    /** @inheritdoc */
    marshalBinary(): Buffer;
    /** @inheritdoc */
    unmarshalBinary(bytes: Buffer): void;
    /** @inheritdoc */
    marshalSize(): number;
    /** @inheritdoc */
    equal(p2: BN256G2Point): boolean;
    /** @inheritdoc */
    string(): string;
}
