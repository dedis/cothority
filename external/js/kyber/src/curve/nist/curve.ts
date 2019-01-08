import { Group, Scalar, Point } from "../../index";
import NistPoint from "./point";
import NistScalar from "./scalar"
import elliptic = require("elliptic");
import BN = require("bn.js");

export type BNType = string | Buffer | BN

export default class Weierstrass implements Group {
    curve: any;
    redN: any;
    bitSize: number
    name: string
    
    constructor(config: { name: string, bitSize: number, gx: BNType, gy: BNType, p?: BNType, a?: BNType, b?: BNType, n: BN}) {
        let { name, bitSize, gx, gy, ...options } = config;
        this.name = name;
        options["g"] = [new BN(gx, 16, "le"), new BN(gy, 16, "le")];
        for (let k in options) {
            if (k === "g") {
                continue;
            }
            options[k] = new BN(options[k], 16, "le");
        }
        this.curve = new elliptic.curve.short(options);
        this.bitSize = bitSize;
        this.redN = BN.red(options.n);
    }
    
    coordLen() {
        return (this.bitSize + 7) >> 3;
    }
    
    /**
    * Returns the size in bytes of a scalar
    */
    scalarLen(): number {
        return (this.curve.n.bitLength() + 7) >> 3;
    }
    
    /**
    * Returns the size in bytes of a point
    */
    scalar(): Scalar {
        return new NistScalar(this, this.redN);
    }
    
    /**
    * Returns the size in bytes of a point
    */
    pointLen(): number {
        // ANSI X9.62: 1 header byte plus 2 coords
        return this.coordLen() * 2 + 1;
    }
    
    /**
    * Returns a new Point
    */
    point(): Point {
        return new NistPoint(this);
    }

    /**
     * Get the name of the curve
     */
    string(): string {
        return this.name;
    }
}