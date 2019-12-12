// tslint:disable:no-bitwise
import BN from "bn.js";
import elliptic from "elliptic";
import { BNType } from "../../constants";
import { Group, Point, Scalar } from "../../index";
import NistPoint from "./point";
import NistScalar from "./scalar";

// tslint:disable-next-line
interface ReductionContext {}

export default class Weierstrass implements Group {
    curve: elliptic.curve.short;
    redN: ReductionContext;
    bitSize: number;
    name: string;

    constructor(config: { name: string, bitSize: number, gx: BNType, gy: BNType,
        p: BNType, a: BNType, b: BNType, n: BN}) {
        const { name, bitSize, gx, gy, p, a, b, n } = config;
        this.name = name;
        const toBN = (bt: BNType) => new BN(bt, 16, "le");
        this.curve = new elliptic.curve.short({
            a: toBN(a),
            b: toBN(b),
            g: ([new BN(gx, 16, "le"), new BN(gy, 16, "le")] as any),
            n,
            p: toBN(p),
        });
        this.bitSize = bitSize;
        this.redN = BN.red(n);
    }

    coordLen(): number {
        return (this.bitSize + 7) >> 3;
    }

    /** @inheritdoc */
    scalarLen(): number {
        return (this.curve.n.bitLength() + 7) >> 3;
    }

    /** @inheritdoc */
    scalar(): Scalar {
        return new NistScalar(this, this.redN);
    }

    /** @inheritdoc */
    pointLen(): number {
        // ANSI X9.62: 1 header byte plus 2 coords
        return this.coordLen() * 2 + 1;
    }

    /** @inheritdoc */
    point(): Point {
        return new NistPoint(this);
    }

    /**
     * Get the name of the curve
     * @returns the name
     */
    string(): string {
        return this.name;
    }
}
