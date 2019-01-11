/// <reference types="node" />
import Ed25519 from "./curve";
import { Scalar } from "../../index";
export default class Ed25519Scalar implements Scalar {
    ref: {
        arr: any;
        curve: Ed25519;
        red: any;
    };
    constructor(curve: Ed25519, red: any);
    /** @inheritdoc */
    marshalSize(): number;
    /** @inheritdoc */
    marshalBinary(): Buffer;
    /** @inheritdoc */
    unmarshalBinary(bytes: Buffer): void;
    /** @inheritdoc */
    equal(s2: Ed25519Scalar): boolean;
    /** @inheritdoc */
    set(a: Ed25519Scalar): Ed25519Scalar;
    /** @inheritdoc */
    clone(): Scalar;
    /** @inheritdoc */
    zero(): Scalar;
    /** @inheritdoc */
    add(a: Ed25519Scalar, b: Ed25519Scalar): Ed25519Scalar;
    /** @inheritdoc */
    sub(a: Ed25519Scalar, b: Ed25519Scalar): Ed25519Scalar;
    /** @inheritdoc */
    neg(a: Ed25519Scalar): Ed25519Scalar;
    /** @inheritdoc */
    mul(s1: Ed25519Scalar, s2: Ed25519Scalar): Ed25519Scalar;
    /** @inheritdoc */
    div(s1: Ed25519Scalar, s2: Ed25519Scalar): Ed25519Scalar;
    /** @inheritdoc */
    inv(a: Ed25519Scalar): Ed25519Scalar;
    /** @inheritdoc */
    one(): Ed25519Scalar;
    /** @inheritdoc */
    pick(callback?: (length: number) => Buffer): Scalar;
    /** @inheritdoc */
    setBytes(bytes: Buffer): Scalar;
    toString(): string;
}
