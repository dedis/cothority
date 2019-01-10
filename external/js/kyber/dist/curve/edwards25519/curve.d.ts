import { Group, Scalar, Point } from "../../index";
import { curve } from "elliptic";
export default class Ed25519 implements Group {
    curve: curve.edwards;
    constructor();
    /**
     * Return the name of the curve
     */
    string(): string;
    /**
     * Returns 32, the size in bytes of a Scalar on Ed25519 curve
     */
    scalarLen(): number;
    /**
     * Returns a new Scalar for the prime-order subgroup of Ed25519 curve
     */
    scalar(): Scalar;
    /**
     * Returns 32, the size of a Point on Ed25519 curve
     *
     * @returns {number}
     */
    pointLen(): number;
    /**
     * Creates a new point on the Ed25519 curve
     */
    point(): Point;
    /**
     * NewKey returns a formatted Ed25519 key (avoiding subgroup attack by requiring
     * it to be a multiple of 8).
     */
    newKey(): Scalar;
}
