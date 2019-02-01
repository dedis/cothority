/// <reference types="node" />
import { BNType } from "../../constants";
import { Point } from "../../index";
import Weierstrass from "./curve";
import NistScalar from "./scalar";
/**
* Represents a Point on the nist curve
*
* The value of the parameters is expected in little endian form if being
* passed as a buffer
*/
export default class NistPoint implements Point {
    ref: {
        curve: Weierstrass;
        point: any;
    };
    constructor(curve: Weierstrass, x?: BNType, y?: BNType);
    /** @inheritdoc */
    set(p2: NistPoint): NistPoint;
    /** @inheritdoc */
    clone(): NistPoint;
    /** @inheritdoc */
    null(): NistPoint;
    /** @inheritdoc */
    base(): NistPoint;
    /** @inheritdoc */
    embedLen(): number;
    /** @inheritdoc */
    embed(data: Buffer, callback?: (length: number) => Buffer): NistPoint;
    /** @inheritdoc */
    data(): Buffer;
    /** @inheritdoc */
    add(p1: NistPoint, p2: NistPoint): NistPoint;
    /** @inheritdoc */
    sub(p1: NistPoint, p2: NistPoint): NistPoint;
    /** @inheritdoc */
    neg(p: NistPoint): NistPoint;
    /** @inheritdoc */
    mul(s: NistScalar, p?: NistPoint): NistPoint;
    /** @inheritdoc */
    pick(callback?: (length: number) => Buffer): NistPoint;
    /** @inheritdoc */
    marshalSize(): number;
    /** @inheritdoc */
    marshalBinary(): Buffer;
    /** @inheritdoc */
    unmarshalBinary(bytes: Buffer): void;
    /** @inheritdoc */
    equals(p2: NistPoint): boolean;
    inspect(): string;
    /** @inheritdoc */
    toString(): string;
    /** @inheritdoc */
    toProto(): Buffer;
}
