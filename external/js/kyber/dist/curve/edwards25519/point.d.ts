/// <reference types="node" />
import BN = require('bn.js');
import { Point } from "../../index";
import Ed25519 from "./curve";
import Ed25519Scalar from "./scalar";
declare type BNType = number | string | number[] | Buffer | BN;
export default class Ed25519Point implements Point {
    ref: {
        point: any;
        curve: Ed25519;
    };
    constructor(curve: Ed25519, X?: BNType, Y?: BNType, Z?: BNType, T?: BNType);
    /** @inheritdoc */
    string(): string;
    inspect(): string;
    toString(): string;
    /** @inheritdoc */
    equal(p2: Ed25519Point): boolean;
    /** @inheritdoc */
    null(): Ed25519Point;
    /** @inheritdoc */
    base(): Ed25519Point;
    /** @inheritdoc */
    pick(callback?: (length: number) => Buffer): Ed25519Point;
    /** @inheritdoc */
    set(p: Ed25519Point): Ed25519Point;
    /** @inheritdoc */
    clone(): Ed25519Point;
    /** @inheritdoc */
    embedLen(): number;
    /** @inheritdoc */
    embed(data: Buffer, callback?: (length: number) => Buffer): Ed25519Point;
    /** @inheritdoc */
    data(): Buffer;
    /** @inheritdoc */
    add(p1: Ed25519Point, p2: Ed25519Point): Ed25519Point;
    /** @inheritdoc */
    sub(p1: Ed25519Point, p2: Ed25519Point): Ed25519Point;
    /** @inheritdoc */
    neg(p: Ed25519Point): Ed25519Point;
    /** @inheritdoc */
    mul(s: Ed25519Scalar, p?: Ed25519Point): Ed25519Point;
    /** @inheritdoc */
    marshalSize(): number;
    /** @inheritdoc */
    marshalBinary(): Buffer;
    /** @inheritdoc */
    unmarshalBinary(bytes: Buffer): void;
}
export {};
