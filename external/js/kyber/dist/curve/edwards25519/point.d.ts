/// <reference types="node" />
import { Point } from "../../index";
import { BNType } from '../../constants';
import Ed25519Scalar from "./scalar";
export default class Ed25519Point implements Point {
    static MARSHAL_ID: Buffer;
    ref: {
        point: any;
    };
    constructor(X?: BNType, Y?: BNType, Z?: BNType, T?: BNType);
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
    inspect(): string;
    /** @inheritdoc */
    equals(p2: Ed25519Point): boolean;
    /** @inheritdoc */
    toString(): string;
    /** @inheritdoc */
    toProto(): Buffer;
}
