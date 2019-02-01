import { Group, Scalar, Point } from "../../index";
import Ed25519Point from "./point";
import Ed25519Scalar from "./scalar"
import { randomBytes, createHash } from "crypto";
import { eddsa, curve } from "elliptic";
import BN from 'bn.js';

const ec = new eddsa("ed25519");
const orderRed = BN.red(ec.curve.n);

export default class Ed25519 implements Group {
    curve: curve.edwards;

    constructor() {
        this.curve = ec.curve;
    }

    /**
     * Get the name of the curve
     * @returns the name
     */
    string(): string {
        return "Ed25519";
    }

    /** @inheritdoc */
    scalarLen(): number {
        return 32;
    }

    /** @inheritdoc */
    scalar(): Scalar {
        return new Ed25519Scalar(this, orderRed);
    }

    /** @inheritdoc */
    pointLen(): number {
        return 32;
    }

    /** @inheritdoc */
    point(): Point {
        return new Ed25519Point();
    }

    /**
     * NewKey returns a formatted Ed25519 key (avoiding subgroup attack by requiring
     * it to be a multiple of 8).
     * @returns the key as a scalar
     */
    newKey(): Scalar {
        const bytes = randomBytes(32);
        const hash = createHash("sha512");
        hash.update(bytes);
        const scalar = Buffer.from(hash.digest());
        scalar[0] &= 0xf8;
        scalar[31] &= 0x3f;
        scalar[31] |= 0x40;

        return this.scalar().setBytes(scalar);
    }
}