/// <reference types="node" />
import { Point } from ".";
/**
 * Factory that must be used to decode points coming from the network or
 * a TOML file. It will take care of looking up the right instance of the
 * point. A dedicated toProto function is provided for the points to encode
 * them back if required.
 */
export declare class PointFactory {
    private static SUITE_ED25519;
    private static SUITE_BN256;
    private tags;
    private suites;
    constructor();
    /**
     * Decode a point using the 8 first bytes to look up the
     * correct instance
     *
     * @param buf bytes to decode
     * @returns the point
     */
    fromProto(buf: Buffer): Point;
    /**
     * Decode the point stored as an hexadecimal string by using
     * the suite as a reference for the point instance
     *
     * @param suite Name of the suite to use
     * @param pub   The point encoded as an hex-string
     * @returns the point
     */
    fromToml(suite: string, pub: string): Point;
}
declare const _default: PointFactory;
export default _default;
