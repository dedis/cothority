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
    marshalSize(): number;
    marshalBinary(): Buffer;
    unmarshalBinary(bytes: Buffer): void;
    equal(s2: Ed25519Scalar): boolean;
    set(a: Ed25519Scalar): Ed25519Scalar;
    clone(): Scalar;
    zero(): Scalar;
    add(a: Ed25519Scalar, b: Ed25519Scalar): Ed25519Scalar;
    sub(a: Ed25519Scalar, b: Ed25519Scalar): Ed25519Scalar;
    neg(a: Ed25519Scalar): Ed25519Scalar;
    mul(s1: Ed25519Scalar, s2: Ed25519Scalar): Ed25519Scalar;
    div(s1: Ed25519Scalar, s2: Ed25519Scalar): Ed25519Scalar;
    inv(a: Ed25519Scalar): Ed25519Scalar;
    one(): Ed25519Scalar;
    pick(callback?: (length: number) => Buffer): Scalar;
    setBytes(bytes: Buffer): Scalar;
    toString(): string;
}
