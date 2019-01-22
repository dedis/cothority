import { Group, Scalar, Point } from "../../index";
import { curve } from "elliptic";
export default class Ed25519 implements Group {
    curve: curve.edwards;
    constructor();
    /**
     * Get the name of the curve
     * @returns the name
     */
    string(): string;
    /** @inheritdoc */
    scalarLen(): number;
    /** @inheritdoc */
    scalar(): Scalar;
    /** @inheritdoc */
    pointLen(): number;
    /** @inheritdoc */
    point(): Point;
    /**
     * NewKey returns a formatted Ed25519 key (avoiding subgroup attack by requiring
     * it to be a multiple of 8).
     * @returns the key as a scalar
     */
    newKey(): Scalar;
}
