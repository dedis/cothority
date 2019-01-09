/// <reference types="node" />
import { Group, Scalar, Point } from "../../index";
import BN = require("bn.js");
export declare type BNType = string | Buffer | BN;
export default class Weierstrass implements Group {
    curve: any;
    redN: any;
    bitSize: number;
    name: string;
    constructor(config: {
        name: string;
        bitSize: number;
        gx: BNType;
        gy: BNType;
        p?: BNType;
        a?: BNType;
        b?: BNType;
        n: BN;
    });
    coordLen(): number;
    /**
    * Returns the size in bytes of a scalar
    */
    scalarLen(): number;
    /**
    * Returns the size in bytes of a point
    */
    scalar(): Scalar;
    /**
    * Returns the size in bytes of a point
    */
    pointLen(): number;
    /**
    * Returns a new Point
    */
    point(): Point;
    /**
     * Get the name of the curve
     */
    string(): string;
}
