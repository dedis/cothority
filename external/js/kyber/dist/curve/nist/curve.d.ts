/// <reference types="node" />
import elliptic from "elliptic";
import BN = require('bn.js');
import { Group, Scalar, Point } from "../../index";
interface ReductionContext {
}
declare type BNType = number | string | number[] | Buffer | BN;
export default class Weierstrass implements Group {
    curve: elliptic.curve.short;
    redN: ReductionContext;
    bitSize: number;
    name: string;
    constructor(config: {
        name: string;
        bitSize: number;
        gx: BNType;
        gy: BNType;
        p: BNType;
        a: BNType;
        b: BNType;
        n: BN;
    });
    coordLen(): number;
    /** @inheritdoc */
    scalarLen(): number;
    /** @inheritdoc */
    scalar(): Scalar;
    /** @inheritdoc */
    pointLen(): number;
    /** @inheritdoc */
    point(): Point;
    /**
     * Get the name of the curve
     * @returns the name
     */
    string(): string;
}
export {};
