/// <reference types="node" />
import BN from "bn.js";
import { Point } from "../../index";
import Ed25519 from "./curve";
import Ed25519Scalar from "./scalar";
export declare type PointType = number | Buffer | BN;
export default class Ed25519Point implements Point {
    ref: {
        point: any;
        curve: Ed25519;
    };
    constructor(curve: Ed25519, X?: PointType, Y?: PointType, Z?: PointType, T?: PointType);
    string(): string;
    inspect(): string;
    toString(): string;
    equal(p2: Ed25519Point): boolean;
    null(): Ed25519Point;
    base(): Ed25519Point;
    pick(callback?: (length: number) => Buffer): Ed25519Point;
    set(p: Ed25519Point): Ed25519Point;
    clone(): Ed25519Point;
    embedLen(): number;
    embed(data: Buffer, callback?: (length: number) => Buffer): Ed25519Point;
    data(): Buffer;
    add(p1: Ed25519Point, p2: Ed25519Point): Ed25519Point;
    sub(p1: Ed25519Point, p2: Ed25519Point): Ed25519Point;
    neg(p: Ed25519Point): Ed25519Point;
    mul(s: Ed25519Scalar, p?: Ed25519Point): Ed25519Point;
    marshalSize(): number;
    marshalBinary(): Buffer;
    unmarshalBinary(bytes: Buffer): void;
}
