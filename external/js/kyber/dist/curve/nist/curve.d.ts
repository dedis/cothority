import elliptic from "elliptic";
import BN, { ReductionContext, BNType } from "bn.js";
import { Group, Scalar, Point } from "../../index";
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
        p?: BNType;
        a: BNType;
        b: BNType;
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
