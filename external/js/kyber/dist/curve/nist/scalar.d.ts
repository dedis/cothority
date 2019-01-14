/// <reference types="node" />
import { Scalar } from "../../index";
import Weierstrass from "./curve";
export default class NistScalar implements Scalar {
    ref: {
        arr: any;
        red: any;
        curve: Weierstrass;
    };
    constructor(curve: Weierstrass, red: any);
    /** @inheritdoc */
    string(): string;
    inspect(): string;
    /** @inheritdoc */
    equal(s2: NistScalar): boolean;
    /** @inheritdoc */
    set(a: NistScalar): NistScalar;
    /** @inheritdoc */
    clone(): NistScalar;
    /** @inheritdoc */
    zero(): NistScalar;
    /** @inheritdoc */
    add(s1: NistScalar, s2: NistScalar): NistScalar;
    /** @inheritdoc */
    sub(s1: NistScalar, s2: NistScalar): NistScalar;
    /** @inheritdoc */
    neg(a: NistScalar): NistScalar;
    /** @inheritdoc */
    one(): NistScalar;
    /** @inheritdoc */
    mul(s1: NistScalar, s2: NistScalar): NistScalar;
    /** @inheritdoc */
    div(s1: NistScalar, s2: NistScalar): NistScalar;
    /** @inheritdoc */
    inv(a: NistScalar): NistScalar;
    /** @inheritdoc */
    setBytes(b: Buffer): NistScalar;
    /** @inheritdoc */
    bytes(): Buffer;
    toString(): string;
    /** @inheritdoc */
    pick(callback?: (length: number) => Buffer): NistScalar;
    /** @inheritdoc */
    marshalSize(): number;
    /** @inheritdoc */
    marshalBinary(): Buffer;
    /** @inheritdoc */
    unmarshalBinary(bytes: Buffer): void;
}
